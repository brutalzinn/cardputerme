/*
 * cardputerme — thin frontend for the generic terminal remote
 * -------------------------------------------------------------------
 * A PURE renderer: it draws the screen the server sends (a generic "display"
 * message = body lines each with their own color + a bottom status bar) and
 * forwards keys. It holds NO content logic — colors, status, menus and meaning
 * are all decided by the server. So new screens/colors need a server change, not
 * a re-flash.
 *
 * Board:   M5Stack Cardputer ADV (StampS3A / ESP32-S3FN8), 240x135 ST7789V2.
 * Library: M5Cardputer (auto-detects Cardputer vs ADV via M5Unified).
 *
 * UI
 *   VIEW mode (default): shows one page of the session screen.
 *     ;  -> previous page        .  -> next page
 *     any letter/key -> start typing a command
 *   INPUT mode: type; Enter sends it over the socket. (del = backspace)
 *   The `esc` key sends `; the SERVER interprets it (` = Esc, `` = picker).
 * -------------------------------------------------------------------
 */

#include <M5Cardputer.h>
#include <WiFi.h>
#include <WebSocketsClient.h>
#include <ArduinoJson.h>
#include <vector>

// ===================================================================
// All settings come from firmware/.env (injected as -DENV_* at build).
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
const int   WRAP_COLS = ENV_WRAP_COLS;
// ===================================================================

// Screen geometry (Cardputer ADV = 240x135)
#define SCR_W       240
#define SCR_H       135
#define HEADER_H    14        // top status bar height
#define STATUS_H    16        // bottom status bar height
#define INPUT_H     18        // command entry bar height (INPUT mode only)
#define LINE_H      16        // size-2 font line height (12x16 px glyphs)
#define SBAR_W      3         // right-edge scroll indicator width

// Chrome colors (RGB565) the firmware owns. BODY colors come FROM THE SERVER,
// per line — the device never decides content color.
#define COL_BG      0x0000    // black
#define COL_HDR     0x10A2    // dark slate bars
#define COL_TEXT    0xFFFF    // white (fallback only)
#define COL_ACCENT  0x07FF    // cyan
#define COL_DIM     0x8410    // grey — hints / scrollbar track
#define COL_OK      0x07E6    // green — wifi ok / toast
#define COL_WARN    0xFD20    // orange — no wifi / errors

// NOTE: don't use bare INPUT/OUTPUT here — reserved Arduino GPIO macros.
enum Mode { MODE_VIEW, MODE_INPUT };
Mode mode = MODE_VIEW;

// The server sends body lines (each with its own color) + a status string.
struct Line { String text; uint16_t color; };
std::vector<Line> g_lines;
String   g_status = "";
uint16_t g_statusColor = COL_ACCENT;
bool     g_sessionExists = false;
int      g_page = 0;

// follow=true keeps us on the newest page as output grows. Paging up turns it
// off; paging back to the last page turns it on again.
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

int bodyBottom() {
  return SCR_H - STATUS_H - (mode == MODE_INPUT ? INPUT_H : 0);
}
int linesPerPage() {
  int n = (bodyBottom() - HEADER_H) / LINE_H;
  return n < 1 ? 1 : n;
}
int pageCount() {
  int lpp = linesPerPage();
  int n = ((int)g_lines.size() + lpp - 1) / lpp;
  return n < 1 ? 1 : n;
}

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
// Top bar: [wifi][mode] .......... [toast]  [page i/total]
void drawHeader() {
  auto& d = M5Cardputer.Display;
  d.fillRect(0, 0, SCR_W, HEADER_H, COL_HDR);
  d.setTextSize(1);

  d.setTextColor(wifiUp() ? COL_OK : COL_WARN, COL_HDR);
  d.setCursor(3, 4);
  d.print(wifiUp() ? "WiFi" : "NoWiFi");
  d.setTextColor(COL_ACCENT, COL_HDR);
  d.print(mode == MODE_INPUT ? " CMD" : " VIEW");

  if (millis() < toastUntil && toast.length()) {
    d.setTextColor(COL_OK, COL_HDR);
    d.setCursor(66, 4);
    d.print(toast);
  }

  d.setTextColor(COL_TEXT, COL_HDR);
  String pos = String(g_lines.empty() ? 0 : g_page + 1) + "/" + String(pageCount());
  d.setCursor(SCR_W - (int)pos.length() * 6 - 3, 4);
  d.print(pos);
}

// Right-edge scroll indicator: which page within the whole screen.
void drawScrollbar() {
  auto& d = M5Cardputer.Display;
  int total = pageCount();
  int top = HEADER_H + 1, bot = bodyBottom() - 1;
  int h = bot - top;
  d.fillRect(SCR_W - SBAR_W, top, SBAR_W, h, COL_HDR);
  if (total <= 1) return;
  int thumbH = h / total; if (thumbH < 4) thumbH = 4;
  int y = top + (int)((long)(h - thumbH) * g_page / (total - 1));
  d.fillRect(SCR_W - SBAR_W, y, SBAR_W, thumbH, COL_ACCENT);
}

// Body: the current page of server-colored lines.
void drawBody() {
  auto& d = M5Cardputer.Display;
  int bb = bodyBottom();
  d.fillRect(0, HEADER_H, SCR_W - SBAR_W, bb - HEADER_H, COL_BG);
  d.setTextSize(2);

  if (g_lines.empty()) {
    d.setTextColor(COL_DIM, COL_BG);
    d.setCursor(4, HEADER_H + 20);
    d.print("(no data)");
    drawScrollbar();
    return;
  }

  int lpp = linesPerPage();
  int start = g_page * lpp;
  int y = HEADER_H + 2;
  for (int i = start; i < start + lpp && i < (int)g_lines.size(); i++) {
    if (y + LINE_H > bb) break;
    d.setTextColor(g_lines[i].color, COL_BG);   // color chosen by the SERVER
    d.setCursor(3, y);
    d.print(g_lines[i].text);
    y += LINE_H;
  }
  drawScrollbar();
}

// Bottom status bar (always shown) — server-composed text, clipped to one line.
void drawStatusBar() {
  auto& d = M5Cardputer.Display;
  int y0 = SCR_H - STATUS_H;
  d.fillRect(0, y0, SCR_W, STATUS_H, COL_HDR);
  d.setTextSize(1);
  d.setTextColor(g_statusColor, COL_HDR);
  d.setCursor(3, y0 + 4);
  String s = g_status;
  const int maxChars = 39;                 // ~6px/char at size 1 across 240px
  if ((int)s.length() > maxChars) s = s.substring(0, maxChars);
  d.print(s);
}

// Command entry bar, just above the status bar (INPUT mode only).
void drawInputBar() {
  auto& d = M5Cardputer.Display;
  int y0 = SCR_H - STATUS_H - INPUT_H;
  d.fillRect(0, y0, SCR_W, INPUT_H, COL_HDR);
  d.setTextSize(2);
  d.setTextColor(COL_ACCENT, COL_HDR);
  d.setCursor(2, y0 + 2);
  String shown = inputBuf;
  const int maxChars = 19;
  if ((int)shown.length() > maxChars) shown = shown.substring(shown.length() - maxChars);
  d.print(">");
  d.print(shown);
  d.print("_");
}

void redraw() {
  drawHeader();
  drawBody();
  drawStatusBar();
  if (mode == MODE_INPUT) drawInputBar();
}

// ------------------------------------------------------------------ network
void beep(bool question) {
  if (question) {
    M5Cardputer.Speaker.tone(1200, 90);
    delay(110);
    M5Cardputer.Speaker.tone(1900, 140);
    return;
  }
  M5Cardputer.Speaker.tone(1600, 70);
}

// Render a pushed {type:"display"} message: body lines (each with a color) + a
// status bar. follow=true keeps us on the newest page.
void applyDisplay(JsonDocument& doc) {
  g_sessionExists = doc["sessionExists"] | true;
  JsonObject status = doc["status"];
  g_status = String((const char*)(status["text"] | ""));
  g_statusColor = (uint16_t)((uint32_t)(status["color"] | (uint32_t)COL_ACCENT));

  int prevPage = g_page, prevPages = pageCount();
  g_lines.clear();
  JsonArray body = doc["body"].as<JsonArray>();
  for (JsonObject o : body) {
    Line ln;
    ln.text = String((const char*)(o["text"] | ""));
    ln.color = (uint16_t)((uint32_t)(o["color"] | (uint32_t)COL_TEXT));
    g_lines.push_back(ln);
  }
  int pages = pageCount();
  bool atNewest = follow || prevPage >= prevPages;
  if (atNewest || g_page > pages - 1) g_page = pages ? pages - 1 : 0;
  redraw();
}

// Persistent WebSocket: server pushes {type:"display"|"notify"|"sessions"};
// device sends {type:"cmd"}. The server decides everything shown.
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
      if (strcmp(t, "display") == 0) { applyDisplay(doc); return; }
      if (strcmp(t, "notify") == 0) {
        bool q = (strcmp(doc["reason"] | "", "question") == 0);
        beep(q);
        showToast(q ? "Answer needed" : "New output");
        drawHeader();
        return;
      }
      // {type:"sessions"} — the picker is rendered server-side as a display; ignore.
      return;
    }
    default:
      break;
  }
}

// Send a typed command up to the server over the same socket.
void sendCommand(const String& text) {
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
    if (c == ';') {                       // up -> previous page (stop following)
      if (g_page > 0) g_page--;
      follow = false;
      redraw();
      return;
    }
    if (c == '.') {                       // down -> next page
      if (g_page + 1 < pageCount()) g_page++;
      if (g_page >= pageCount() - 1) follow = true;
      redraw();
      return;
    }
    // any other key -> start a command (incl. `esc`/backtick; the server reads it)
    mode = MODE_INPUT;
    inputBuf = "";
    inputBuf += c;
    redraw();
    return;
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
    follow = true;
    sendCommand(cmd);
    redraw();
    return;
  }
  if (changed) drawInputBar();
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

  webSocket.begin(WS_HOST, WS_PORT, WS_PATH);
  webSocket.onEvent(onWsEvent);
  webSocket.setReconnectInterval(3000);
  showToast("Linking...");
  redraw();
}

void loop() {
  M5Cardputer.update();
  webSocket.loop();

  if (M5Cardputer.Keyboard.isChange() && M5Cardputer.Keyboard.isPressed()) {
    Keyboard_Class::KeysState st = M5Cardputer.Keyboard.keysState();
    Mode m0 = mode;                       // capture first — handleViewKey may switch to INPUT
    if (m0 == MODE_VIEW) handleViewKey(st);
    if (m0 == MODE_INPUT) handleInputKey(st);
  }

  static bool toastWasShown = false;
  bool toastShowing = (millis() < toastUntil) && toast.length();
  if (toastWasShown && !toastShowing) drawHeader();
  toastWasShown = toastShowing;

  delay(5);
}
