/*
 * Cardputer ADV  <->  Claude Code remote
 * -------------------------------------------------------------------
 * Reads Claude Code's output (paged into legible "cards") from the Node.js
 * bridge on the Mac Mini, and sends commands typed on the physical keyboard.
 *
 * Board:   M5Stack Cardputer ADV (StampS3A / ESP32-S3FN8), 240x135 ST7789V2.
 * Library: M5Cardputer (auto-detects Cardputer vs ADV via M5Unified).
 *
 * UI
 *   VIEW mode (default): shows one card of Claude's output.
 *     ;  -> previous card        .  -> next card
 *     `  -> refresh              any letter/key -> start typing a command
 *   INPUT mode: type a command on the keyboard.
 *     Enter -> POST it to the server, then refresh
 *     `     -> cancel back to VIEW           (del = backspace)
 * -------------------------------------------------------------------
 */

#include <M5Cardputer.h>
#include <WiFi.h>
#include <WebSocketsClient.h>
#include <ArduinoJson.h>
#include <vector>

// ===================================================================
// All settings come from firmware/.env (injected as -DENV_* at build).
// Fallbacks below are only used if a key is missing from .env.
#ifndef ENV_WIFI_SSID
#define ENV_WIFI_SSID "YOUR_WIFI_SSID"
#endif
#ifndef ENV_WIFI_PASS
#define ENV_WIFI_PASS "YOUR_WIFI_PASSWORD"
#endif
#ifndef ENV_WS_HOST
#define ENV_WS_HOST "192.168.0.149"
#endif
#ifndef ENV_WS_PORT
#define ENV_WS_PORT 4711
#endif
#ifndef ENV_WRAP_COLS
#define ENV_WRAP_COLS 20
#endif

const char* WIFI_SSID = ENV_WIFI_SSID;
const char* WIFI_PASS = ENV_WIFI_PASS;
const char* WS_HOST   = ENV_WS_HOST;
const int   WS_PORT   = ENV_WS_PORT;
const char* WS_PATH   = "/ws";
const int   WRAP_COLS = ENV_WRAP_COLS;   // must match server .env
// ===================================================================

// Screen geometry (Cardputer ADV = 240x135)
#define SCR_W       240
#define SCR_H       135
#define HEADER_H    14        // top status bar height
#define BODY_TOP    16        // first body text baseline area
#define LINE_H      16        // size-2 font line height (12x16 px glyphs)
#define SBAR_W      3         // right-edge scroll indicator width

// Coherent dark theme (RGB565)
#define COL_BG      0x0000    // black — max legibility
#define COL_HDR     0x10A2    // dark slate header bar
#define COL_TEXT    0xFFFF    // white — Claude reply text
#define COL_ACCENT  0x07FF    // cyan — mode / accents
#define COL_PROMPT  0xFFE0    // yellow — the "> your prompt" lines
#define COL_DIM     0x8410    // grey — hints / scrollbar track
#define COL_OK      0x07E6    // green — wifi ok / toast
#define COL_WARN    0xFD20    // orange — no wifi / errors

// NOTE: don't use bare INPUT/OUTPUT here — they are reserved Arduino GPIO macros.
enum Mode { MODE_VIEW, MODE_INPUT };
Mode mode = MODE_VIEW;

std::vector<std::vector<String>> g_cards;
int  g_index = 0;
bool g_sessionExists = false;

// Auto-scroll: when `follow` is true we jump to the newest card as Claude's
// reply grows (so the Cardputer mirrors Claude live). Paging up turns it off;
// paging back to the last card turns it on again.
bool follow = true;

WebSocketsClient webSocket;
bool wsConnected = false;

String inputBuf = "";
String toast = "";
unsigned long toastUntil = 0;

// ------------------------------------------------------------------ helpers
void showToast(const String& msg, uint16_t ms = 1200) {
  toast = msg;
  toastUntil = millis() + ms;
}

bool wifiUp() { return WiFi.status() == WL_CONNECTED; }

void connectWifi() {
  M5Cardputer.Display.fillScreen(COL_BG);
  M5Cardputer.Display.setTextColor(COL_TEXT, COL_BG);
  M5Cardputer.Display.setTextSize(2);
  M5Cardputer.Display.setCursor(4, 8);
  M5Cardputer.Display.print("WiFi...");
  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASS);
  unsigned long start = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - start < 20000) {
    delay(250);
    M5Cardputer.Display.print(".");
  }
  M5Cardputer.Display.fillScreen(COL_BG);
}

// ------------------------------------------------------------------ drawing
// Colored top bar: [wifi][mode] .......... [toast]  [card i/total]
void drawHeader() {
  auto& d = M5Cardputer.Display;
  d.fillRect(0, 0, SCR_W, HEADER_H, COL_HDR);
  d.setTextSize(1);

  d.setTextColor(wifiUp() ? COL_OK : COL_WARN, COL_HDR);
  d.setCursor(3, 4);
  d.print(wifiUp() ? "WiFi" : "NoWiFi");
  d.setTextColor(COL_ACCENT, COL_HDR);
  d.print(mode == MODE_INPUT ? " CMD" : " VIEW");

  // transient toast, centered-ish
  if (millis() < toastUntil && toast.length()) {
    d.setTextColor(COL_OK, COL_HDR);
    d.setCursor(66, 4);
    d.print(toast);
  }

  // right: card position
  d.setTextColor(COL_TEXT, COL_HDR);
  String pos = String(g_cards.empty() ? 0 : g_index + 1) + "/" + String((int)g_cards.size());
  d.setCursor(SCR_W - (int)pos.length() * 6 - 3, 4);
  d.print(pos);
}

// Right-edge scroll indicator showing which card we're on within the whole reply.
void drawScrollbar() {
  auto& d = M5Cardputer.Display;
  int total = (int)g_cards.size();
  int top = HEADER_H + 1, bot = SCR_H - 1;
  int h = bot - top;
  d.fillRect(SCR_W - SBAR_W, top, SBAR_W, h, COL_HDR);       // track
  if (total <= 1) return;
  int thumbH = h / total; if (thumbH < 4) thumbH = 4;
  int y = top + (int)((long)(h - thumbH) * g_index / (total - 1));
  d.fillRect(SCR_W - SBAR_W, y, SBAR_W, thumbH, COL_ACCENT);  // thumb
}

// Body fills the whole screen below the header. In INPUT mode the last 18px
// are reserved for the command line, so the reply scrolls above it.
void drawBody() {
  auto& d = M5Cardputer.Display;
  int bodyBottom = (mode == MODE_INPUT) ? (SCR_H - 18) : SCR_H;
  d.fillRect(0, HEADER_H, SCR_W - SBAR_W, bodyBottom - HEADER_H, COL_BG);
  d.setTextSize(2);

  if (g_cards.empty()) {
    d.setTextColor(COL_DIM, COL_BG);
    d.setCursor(4, HEADER_H + 20);
    d.print("(no data - ` refresh)");
    drawScrollbar();
    return;
  }

  const auto& card = g_cards[g_index];
  int y = HEADER_H + 2;
  for (const auto& line : card) {
    if (y + LINE_H > bodyBottom) break;
    // Highlight the "> your prompt" lines vs Claude's reply text.
    bool isPrompt = line.startsWith("> ");
    d.setTextColor(isPrompt ? COL_PROMPT : COL_TEXT, COL_BG);
    d.setCursor(3, y);
    d.print(line);
    y += LINE_H;
  }
  drawScrollbar();
}

// Command entry bar pinned to the bottom (only shown in INPUT mode).
void drawInputBar() {
  auto& d = M5Cardputer.Display;
  int y0 = SCR_H - 18;
  d.fillRect(0, y0, SCR_W, 18, COL_HDR);
  d.setTextSize(2);
  d.setTextColor(COL_ACCENT, COL_HDR);
  d.setCursor(2, y0 + 2);
  String shown = inputBuf;
  const int maxChars = 19;                 // fits 240px at size 2
  if ((int)shown.length() > maxChars) shown = shown.substring(shown.length() - maxChars);
  d.print(">");
  d.print(shown);
  d.print("_");
}

void redraw() {
  drawHeader();
  drawBody();
  if (mode == MODE_INPUT) drawInputBar();
}

// ------------------------------------------------------------------ network
// Short beep(s) so the user knows Claude produced output or needs an answer.
void beep(bool question) {
  if (question) {                         // rising two-tone = "Claude needs you"
    M5Cardputer.Speaker.tone(1200, 90);
    delay(110);
    M5Cardputer.Speaker.tone(1900, 140);
  } else {                                // single soft tone = new reply
    M5Cardputer.Speaker.tone(1600, 70);
  }
}

// Render a pushed {type:"cards"} message. follow=true auto-scrolls to newest.
void applyCards(JsonDocument& doc) {
  g_sessionExists = doc["sessionExists"] | false;
  JsonArray cards = doc["cards"].as<JsonArray>();

  int prevIdx = g_index;
  int prevTotal = (int)g_cards.size();

  g_cards.clear();
  for (JsonArray card : cards) {
    std::vector<String> lines;
    for (JsonVariant v : card) lines.push_back(String((const char*)v));
    g_cards.push_back(lines);
  }

  int total = (int)g_cards.size();
  if (follow || prevIdx >= prevTotal) g_index = total ? total - 1 : 0;
  else if (g_index > total - 1)       g_index = total ? total - 1 : 0;
  redraw();
}

// The only channel the device uses: a persistent WebSocket. Server pushes
// {type:"cards"|"notify"}; device sends {type:"cmd"}.
void onWsEvent(WStype_t type, uint8_t* payload, size_t length) {
  switch (type) {
    case WStype_CONNECTED:
      wsConnected = true;
      showToast("Connected");
      redraw();
      break;
    case WStype_DISCONNECTED:
      wsConnected = false;
      showToast("Reconnecting");
      redraw();
      break;
    case WStype_TEXT: {
      JsonDocument doc;
      if (deserializeJson(doc, payload, length)) return;
      const char* t = doc["type"] | "";
      if (strcmp(t, "cards") == 0) {
        applyCards(doc);
      } else if (strcmp(t, "notify") == 0) {
        bool q = (strcmp(doc["reason"] | "", "question") == 0);
        beep(q);
        showToast(q ? "Claude asks!" : "New reply");
        drawHeader();
      }
      break;
    }
    default:
      break;
  }
}

// Send a typed command up to the server over the same socket.
void sendCommand(const String& text) {
  if (!text.length()) return;
  if (!wsConnected) { showToast("No link"); redraw(); return; }
  JsonDocument doc;
  doc["type"] = "cmd";
  doc["text"] = text;
  String body;
  serializeJson(doc, body);
  webSocket.sendTXT(body);
  showToast("Sent");
}

// ------------------------------------------------------------------ input
void handleViewKey(const Keyboard_Class::KeysState& st) {
  for (char c : st.word) {
    if (c == ';') {                       // up -> previous card (stop following)
      if (g_index > 0) g_index--;
      follow = false;
      redraw();
      return;
    } else if (c == '.') {                // down -> next card
      if (g_index + 1 < (int)g_cards.size()) g_index++;
      if (g_index >= (int)g_cards.size() - 1) follow = true;  // at newest -> follow again
      redraw();
      return;
    } else {                              // any other key -> start a command
      mode = MODE_INPUT;
      inputBuf = "";
      inputBuf += c;
      redraw();
      return;
    }
  }
  if (st.enter) {                         // Enter alone -> start an empty command
    mode = MODE_INPUT;
    inputBuf = "";
    redraw();
  }
}

void handleInputKey(const Keyboard_Class::KeysState& st) {
  bool changed = false;
  for (char c : st.word) {
    inputBuf += c;
    changed = true;
  }
  if (st.del && inputBuf.length()) {      // backspace
    inputBuf.remove(inputBuf.length() - 1);
    changed = true;
  }
  if (st.enter) {                         // submit over the websocket
    String cmd = inputBuf;
    inputBuf = "";
    mode = MODE_VIEW;
    follow = true;                        // auto-scroll to Claude's incoming reply
    sendCommand(cmd);
    redraw();
    return;
  }
  if (changed) drawInputBar();            // cheap partial redraw while typing
}

// ------------------------------------------------------------------ Arduino
void setup() {
  auto cfg = M5.config();
  M5Cardputer.begin(cfg, true);           // true = enable keyboard
  M5Cardputer.Display.setRotation(1);     // landscape 240x135
  M5Cardputer.Speaker.begin();
  M5Cardputer.Speaker.setVolume(140);
  M5Cardputer.Display.fillScreen(COL_BG);

  connectWifi();

  // Pure WebSocket: the only channel for cards (down) and commands (up).
  webSocket.begin(WS_HOST, WS_PORT, WS_PATH);
  webSocket.onEvent(onWsEvent);
  webSocket.setReconnectInterval(3000);
  showToast("Linking...");
  redraw();
}

void loop() {
  M5Cardputer.update();
  webSocket.loop();                       // pump the socket (no polling)

  if (M5Cardputer.Keyboard.isChange() && M5Cardputer.Keyboard.isPressed()) {
    Keyboard_Class::KeysState st = M5Cardputer.Keyboard.keysState();
    if (mode == MODE_VIEW) handleViewKey(st);
    else                   handleInputKey(st);
  }

  // Clear an expired toast without a redraw storm.
  static bool toastWasShown = false;
  bool toastShowing = (millis() < toastUntil) && toast.length();
  if (toastWasShown && !toastShowing) drawHeader();
  toastWasShown = toastShowing;

  delay(5);
}
