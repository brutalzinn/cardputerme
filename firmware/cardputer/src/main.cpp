#include <M5Cardputer.h>
#include <WiFi.h>
#include <WiFiUdp.h>
#include <WebSocketsClient.h>
#include <ArduinoJson.h>
#include <HTTPClient.h>
#include <vector>
#include "esp_sleep.h"

#ifndef ENV_WIFI_SSID
#define ENV_WIFI_SSID "YOUR_WIFI_SSID"
#endif
#ifndef ENV_WIFI_PASS
#define ENV_WIFI_PASS "YOUR_WIFI_PASSWORD"
#endif
#ifndef ENV_WRAP_COLS
#define ENV_WRAP_COLS 20
#endif

const char* WIFI_SSID = ENV_WIFI_SSID;
const char* WIFI_PASS = ENV_WIFI_PASS;
const char* WS_PATH   = "/ws";
const int   WRAP_COLS = ENV_WRAP_COLS;
const int   BEACON_PORT = 8000;
const unsigned long BEACON_TTL_MS = 6500;

#define SCR_W       240
#define SCR_H       135
#define HEADER_H    14
#define STATUS_H    16
#define SBAR_W      3

#define RGB_LED_PIN   21
#define LED_PULSE_MS  600
#define WAV_MAX_BYTES 200000
#define BAT_SAMPLES        8
#define BAT_HYSTERESIS_MV  80
#define BAT_PCT_HYSTERESIS 2

#define COL_BG      0x0000
#define COL_HDR     0x10A2
#define COL_TEXT    0xFFFF
#define COL_ACCENT  0x07FF
#define COL_DIM     0x8410
#define COL_WARN    0xFD20

struct Line { String text; uint16_t color; };
std::vector<Line> g_lines;
String   g_status = "";
std::vector<Line> g_header;
bool     g_hadFrame = false;
uint16_t g_statusColor = COL_ACCENT;
bool     g_sessionExists = false;
int      g_page = 0;
int      g_size = 2;

int lineH() { return g_size * 8; }

bool follow = true;

WebSocketsClient webSocket;
bool wsConnected = false;

WiFiUDP g_udp;
struct Found { IPAddress ip; uint16_t port; String name; unsigned long seen; };
std::vector<Found> g_found;
bool g_haveTarget = false;
IPAddress g_targetIp;
uint16_t g_targetPort = 0;
bool g_listDirty = false;

enum PowerState { POWER_ON, POWER_DIM, POWER_OFF };
PowerState g_power = POWER_ON;

#define BRIGHT_ON   255
#define BRIGHT_DIM  40
#define BRIGHT_OFF  0

bool wifiUp() { return WiFi.status() == WL_CONNECTED; }

// Local battery display — owned entirely by the device, defined near the
// sampling code below; declared here so drawHeader() can call it.
String batteryText(uint16_t& color);
void drawBatteryIndicator();

int bodyBottom() {
  return SCR_H - STATUS_H;
}
int linesPerPage() {
  int n = (bodyBottom() - HEADER_H) / lineH();
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

// The header is composed by the server and drawn here verbatim. The device
// decides nothing about what it says, so changing it costs a server restart
// and never a re-flash (#45). The only local text is the fallback below, for
// the one case no server can speak to: there is no server.
void drawHeader() {
  auto& d = M5Cardputer.Display;
  d.fillRect(0, 0, SCR_W, HEADER_H, COL_HDR);
  d.setTextSize(1);
  d.setCursor(3, 4);
  if (!g_hadFrame) {
    d.setTextColor(COL_WARN, COL_HDR);
    d.print(wifiUp() ? "connecting..." : "no wifi");
    drawBatteryIndicator();
    return;
  }
  for (size_t i = 0; i < g_header.size(); i++) {
    d.setTextColor(g_header[i].color, COL_HDR);
    d.print(g_header[i].text);
  }
  drawBatteryIndicator();
}

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

void drawBody() {
  auto& d = M5Cardputer.Display;
  int bb = bodyBottom();
  d.fillRect(0, HEADER_H, SCR_W - SBAR_W, bb - HEADER_H, COL_BG);
  d.setTextSize(g_size);

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
    if (y + lineH() > bb) break;
    d.setTextColor(g_lines[i].color, COL_BG);
    d.setCursor(3, y);
    d.print(g_lines[i].text);
    y += lineH();
  }
  drawScrollbar();
}

int g_statusOffset = 0;
unsigned long g_statusTick = 0;

void drawStatusBar() {
  auto& d = M5Cardputer.Display;
  int y0 = SCR_H - STATUS_H;
  d.fillRect(0, y0, SCR_W, STATUS_H, COL_HDR);
  d.setTextSize(1);
  d.setTextColor(g_statusColor, COL_HDR);
  d.setCursor(3, y0 + 4);
  const int maxChars = 39;
  String s = g_status;
  if ((int)s.length() <= maxChars) { d.print(s); return; }

  String loop = s + "   ";
  int n = loop.length();
  int off = g_statusOffset % n;
  String win = loop.substring(off);
  if ((int)win.length() < maxChars) win += loop.substring(0, maxChars - win.length());
  if ((int)win.length() > maxChars) win = win.substring(0, maxChars);
  d.print(win);
}

void tickStatusMarquee() {
  const int maxChars = 39;
  if ((int)g_status.length() <= maxChars) return;
  unsigned long now = millis();
  if (now - g_statusTick < 300) return;
  g_statusTick = now;
  g_statusOffset++;
  drawStatusBar();
}

void redraw() {
  drawHeader();
  drawBody();
  drawStatusBar();
}

int foundIndex(const IPAddress& ip, uint16_t port) {
  for (size_t i = 0; i < g_found.size(); i++) {
    if (g_found[i].ip == ip && g_found[i].port == port) return (int)i;
  }
  return -1;
}

void drawSearching() {
  auto& d = M5Cardputer.Display;
  d.fillScreen(COL_BG);
  drawHeader();
  d.setTextSize(2);
  d.setTextColor(COL_ACCENT, COL_BG);
  d.setCursor(3, HEADER_H + 4);
  d.print("cardputerme");
  d.setTextSize(1);
  d.setTextColor(COL_DIM, COL_BG);
  d.setCursor(3, HEADER_H + 28);
  d.print("Listening for terminals...");
  d.setCursor(3, HEADER_H + 40);
  d.print("On the computer: cardputerme");
}

void connectToFound(int idx) {
  if (idx < 0 || idx >= (int)g_found.size()) return;
  g_targetIp = g_found[idx].ip;
  g_targetPort = g_found[idx].port;
  g_haveTarget = true;
  g_lines.clear();
  g_status = "";
  webSocket.disconnect();
  webSocket.begin(g_targetIp.toString(), g_targetPort, WS_PATH);
  M5Cardputer.Display.fillScreen(COL_BG);
  redraw();
}

void leaveServer() {
  g_haveTarget = false;
  wsConnected = false;
  webSocket.disconnect();
  g_lines.clear();
  g_status = "";
  drawSearching();
}

void pollBeacons() {
  int size = g_udp.parsePacket();
  while (size > 0) {
    char buf[256];
    int len = g_udp.read(buf, 255);
    if (len > 0) {
      buf[len] = 0;
      JsonDocument doc;
      bool ok = deserializeJson(doc, buf, len) == DeserializationError::Ok;
      if (ok && strcmp(doc["app"] | "", "cardputerme") == 0) {
        IPAddress ip = g_udp.remoteIP();
        uint16_t port = (uint16_t)(doc["port"] | 0);
        String name = String((const char*)(doc["name"] | ""));
        if (port > 0) {
          int idx = foundIndex(ip, port);
          if (idx < 0) {
            Found f; f.ip = ip; f.port = port; f.name = name; f.seen = millis();
            g_found.push_back(f);
            g_listDirty = true;
          }
          if (idx >= 0) {
            g_found[idx].seen = millis();
            if (g_found[idx].name != name) { g_found[idx].name = name; g_listDirty = true; }
          }
        }
      }
    }
    size = g_udp.parsePacket();
  }
}

void expireFound() {
  for (int i = (int)g_found.size() - 1; i >= 0; i--) {
    if (millis() - g_found[i].seen <= BEACON_TTL_MS) continue;
    g_found.erase(g_found.begin() + i);
    g_listDirty = true;
  }
}

void beep(bool question) {
  if (question) {
    M5Cardputer.Speaker.tone(1200, 90);
    delay(110);
    M5Cardputer.Speaker.tone(1900, 140);
    return;
  }
  M5Cardputer.Speaker.tone(1600, 70);
}

uint8_t g_ledR = 0, g_ledG = 0, g_ledB = 0;
String g_ledPattern = "off";
unsigned long g_ledTick = 0;
bool g_ledPhase = true;

void paintLed() {
  bool dark = g_ledPattern == "off" || (g_ledPattern == "pulse" && !g_ledPhase);
  if (dark) {
    neopixelWrite(RGB_LED_PIN, 0, 0, 0);
    return;
  }
  neopixelWrite(RGB_LED_PIN, g_ledR, g_ledG, g_ledB);
}

void applyLed(JsonDocument& doc) {
  g_ledR = doc["r"] | 0;
  g_ledG = doc["g"] | 0;
  g_ledB = doc["b"] | 0;
  g_ledPattern = String((const char*)(doc["pattern"] | "off"));
  g_ledPhase = true;
  g_ledTick = millis();
  paintLed();
}

void tickLed() {
  if (g_ledPattern != "pulse") return;
  unsigned long now = millis();
  if (now - g_ledTick < LED_PULSE_MS) return;
  g_ledTick = now;
  g_ledPhase = !g_ledPhase;
  paintLed();
}

uint8_t* g_wav = nullptr;
size_t g_wavCap = 0;

bool loadWav(const char* url, size_t& outLen) {
  HTTPClient http;
  if (!http.begin(url)) return false;
  if (http.GET() != HTTP_CODE_OK) { http.end(); return false; }
  int len = http.getSize();
  if (len <= 0 || len > WAV_MAX_BYTES) { http.end(); return false; }
  if ((size_t)len > g_wavCap) {
    uint8_t* grown = (uint8_t*)realloc(g_wav, len);
    if (!grown) { http.end(); return false; }
    g_wav = grown;
    g_wavCap = len;
  }
  WiFiClient* stream = http.getStreamPtr();
  int got = 0;
  while (got < len && http.connected()) {
    int n = stream->readBytes(g_wav + got, len - got);
    if (n <= 0) break;
    got += n;
  }
  http.end();
  outLen = got;
  return got == len;
}

String g_wavUrl = "";
size_t g_wavLen = 0;
String g_pendingSound = "";

void playPending() {
  if (g_pendingSound.length() == 0) return;
  String url = g_pendingSound;
  g_pendingSound = "";

  if (url == g_wavUrl && g_wavLen > 0) {
    M5Cardputer.Speaker.playWav(g_wav, g_wavLen);
    return;
  }
  size_t len = 0;
  if (loadWav(url.c_str(), len)) {
    g_wavUrl = url;
    g_wavLen = len;
    M5Cardputer.Speaker.playWav(g_wav, len);
    return;
  }
  M5Cardputer.Speaker.tone(1200, 90);
}

void applySound(JsonDocument& doc) {
  if (doc["stop"] | false) {
    g_pendingSound = "";
    M5Cardputer.Speaker.stop();
    return;
  }
  const char* url = doc["url"] | "";
  if (strlen(url) == 0) {
    M5Cardputer.Speaker.tone(doc["freq"] | 1200, doc["ms"] | 90);
    return;
  }
  g_pendingSound = String(url);
}

void readCells(JsonArray from, std::vector<Line>& into) {
  into.clear();
  for (JsonObject o : from) {
    Line cell;
    cell.text = String((const char*)(o["text"] | ""));
    cell.color = (uint16_t)((uint32_t)(o["color"] | (uint32_t)COL_TEXT));
    into.push_back(cell);
  }
}

void applyDisplay(JsonDocument& doc) {
  g_hadFrame = true;
  g_sessionExists = doc["sessionExists"] | true;
  g_size = doc["size"] | 2;
  readCells(doc["header"].as<JsonArray>(), g_header);
  JsonObject status = doc["status"];
  String newStatus = String((const char*)(status["text"] | ""));
  if (newStatus != g_status) g_statusOffset = 0;
  g_status = newStatus;
  g_statusColor = (uint16_t)((uint32_t)(status["color"] | (uint32_t)COL_ACCENT));

  int prevPage = g_page, prevPages = pageCount();
  readCells(doc["body"].as<JsonArray>(), g_lines);
  int pages = pageCount();
  bool atNewest = follow || prevPage >= prevPages;
  if (atNewest || g_page > pages - 1) g_page = pages ? pages - 1 : 0;
  redraw();
}

void sendDoc(JsonDocument& doc) {
  if (!wsConnected) return;
  String body;
  serializeJson(doc, body);
  webSocket.sendTXT(body);
}

int g_lastMv = -1;
int g_lastUsb = -1;
unsigned long g_supplyTick = 0;

// Local battery display state — owned entirely by the device now, so it is
// visible with no session and no server (-1 = never sampled; an unread
// battery is not a flat one).
int  g_batPct = -1;
bool g_batUsb = false;

void sendEvent(const char* type) {
  JsonDocument doc;
  doc["type"] = type;
  sendDoc(doc);
}

void sendBeacons() {
  if (!wsConnected) return;
  JsonDocument doc;
  doc["type"] = "beacons";
  JsonArray list = doc["list"].to<JsonArray>();
  for (size_t i = 0; i < g_found.size(); i++) {
    JsonObject o = list.add<JsonObject>();
    o["name"] = g_found[i].name;
    o["ip"] = g_found[i].ip.toString();
    o["port"] = g_found[i].port;
  }
  sendDoc(doc);
}

void sendReport(bool usb, int mv) {
  if (!wsConnected) return;
  JsonDocument doc;
  doc["type"] = "report";
  doc["usb"] = usb;
  doc["wifi"] = wifiUp();
  doc["mv"] = mv;
  doc["battery"] = M5Cardputer.Power.getBatteryLevel();
  doc["heap"] = (int)ESP.getFreeHeap();
  doc["heapmin"] = (int)ESP.getMinFreeHeap();
  sendDoc(doc);
}

int sampleMilliVolts() {
  long total = 0;
  for (int i = 0; i < BAT_SAMPLES; i++) total += M5Cardputer.Power.getBatteryVoltage();
  return (int)(total / BAT_SAMPLES);
}

// getBatteryLevel() returns a negative error code on this board (confirmed
// on hardware, 2026-08-17) — silently ignored for months since the wire
// field it fed was never consumed. getBatteryVoltage()/mv is proven
// reliable (used throughout, including /health), so derive the percentage
// from it directly with the same mapping M5Unified uses internally rather
// than depend on the broken call.
int batteryPercentFromMv(int mv) {
  long level = ((long)(mv - 3300) * 100) / (4150 - 3350);
  if (level < 0) return 0;
  if (level > 100) return 100;
  return (int)level;
}

// batteryText/drawBatteryIndicator: the display owns this now, independent
// of whether a server frame has ever arrived. -1 means never sampled — an
// unread battery must read as unknown, never as a flat 0%.
String batteryText(uint16_t& color) {
  if (g_batPct < 0) { color = COL_DIM; return "--%"; }
  color = g_batUsb ? COL_ACCENT : COL_TEXT;
  String pct = String(g_batPct) + "%";
  return g_batUsb ? ("+" + pct) : pct;
}

void drawBatteryIndicator() {
  auto& d = M5Cardputer.Display;
  uint16_t color;
  String text = batteryText(color);
  const int w = 46;
  d.fillRect(SCR_W - w, 0, w, HEADER_H, COL_HDR);
  d.setTextSize(1);
  d.setTextColor(color, COL_HDR);
  d.drawRightString(text, SCR_W - 3, 4);
}

// updateBatteryDisplay applies its own small deadband on top of the caller's
// mv sample so single-read ADC noise (~50mV, several percent points) can't
// flicker the digits.
void updateBatteryDisplay(bool usb, int mv) {
  int pct = batteryPercentFromMv(mv);
  if (g_batPct < 0 || abs(pct - g_batPct) >= BAT_PCT_HYSTERESIS) g_batPct = pct;
  g_batUsb = usb;
  drawBatteryIndicator();
}

void reportSupply(bool usb, int mv, bool force) {
  bool changed = force || (int)usb != g_lastUsb || abs(mv - g_lastMv) >= BAT_HYSTERESIS_MV;
  if (!changed) return;
  g_lastUsb = (int)usb;
  g_lastMv = mv;
  sendReport(usb, mv);
}

void tickSupply() {
  unsigned long now = millis();
  if (now - g_supplyTick < 5000) return;
  g_supplyTick = now;
  bool usb = HWCDC::isPlugged();
  int mv = sampleMilliVolts();
  updateBatteryDisplay(usb, mv);
  reportSupply(usb, mv, false);
}

uint8_t brightnessFor(PowerState p) {
  if (p == POWER_OFF) return BRIGHT_OFF;
  if (p == POWER_DIM) return BRIGHT_DIM;
  return BRIGHT_ON;
}

void applyPower(PowerState p) {
  g_power = p;
  M5Cardputer.Display.setBrightness(brightnessFor(p));
}

void requestWake() {
  applyPower(POWER_ON);
  sendEvent("wake");
}

void requestSleep() {
  applyPower(POWER_OFF);
  sendEvent("sleep");
}

PowerState powerFromName(const char* name) {
  if (strcmp(name, "off") == 0) return POWER_OFF;
  if (strcmp(name, "dim") == 0) return POWER_DIM;
  return POWER_ON;
}

void togglePower() {
  if (g_power != POWER_ON) { requestWake(); return; }
  requestSleep();
}

void onWsEvent(WStype_t type, uint8_t* payload, size_t length) {
  switch (type) {
    case WStype_CONNECTED:
      wsConnected = true;
      redraw();
      reportSupply(HWCDC::isPlugged(), sampleMilliVolts(), true);
      sendBeacons();
      break;
    case WStype_DISCONNECTED:
      wsConnected = false;
      if (!g_haveTarget) break;
      g_hadFrame = false;
      g_header.clear();
      redraw();
      break;
    case WStype_TEXT: {
      JsonDocument doc;
      if (deserializeJson(doc, payload, length)) return;
      const char* t = doc["type"] | "";
      if (strcmp(t, "display") == 0) { applyDisplay(doc); return; }
      if (strcmp(t, "power") == 0) {
        PowerState want = powerFromName(doc["state"] | "on");
        // The server no longer knows about USB power — it can only ever
        // command the passive idle ladder (dim/off), never a deliberate
        // sleep (that's requestSleep(), a separate local path). On charge
        // there is no battery to save, so the device overrides the ladder
        // itself instead of asking the server to.
        if (g_batUsb && want != POWER_ON) want = POWER_ON;
        applyPower(want);
        return;
      }
      if (strcmp(t, "connect") == 0) {
        IPAddress ip;
        if (!ip.fromString((const char*)(doc["ip"] | ""))) return;
        int idx = foundIndex(ip, (uint16_t)(doc["port"] | 0));
        if (idx >= 0) connectToFound(idx);
        return;
      }
      if (strcmp(t, "led") == 0) { applyLed(doc); return; }
      if (strcmp(t, "sound") == 0) { applySound(doc); return; }

      return;
    }
    default:
      break;
  }
}

char g_heldKey[24];
JsonDocument g_keyBatch;
JsonArray g_keyEvents;

void queueKeyEvent(const String& key, const char* state) {
  if (g_keyEvents.isNull()) g_keyEvents = g_keyBatch["events"].to<JsonArray>();
  JsonObject e = g_keyEvents.add<JsonObject>();
  e["key"] = key;
  e["state"] = state;
}

void flushKeyEvents() {
  if (g_keyEvents.isNull() || g_keyEvents.size() == 0) return;
  g_keyBatch["type"] = "keys";
  sendDoc(g_keyBatch);
  g_keyBatch.clear();
  g_keyEvents = JsonArray();
}

void sendKey(const char* key) {
  strncpy(g_heldKey, key, sizeof(g_heldKey) - 1);
  g_heldKey[sizeof(g_heldKey) - 1] = 0;
  queueKeyEvent(key, "down");
}

void releaseKey() {
  if (g_heldKey[0] == 0) return;
  queueKeyEvent(g_heldKey, "up");
  g_heldKey[0] = 0;
}

const char* arrowFor(char c) {
  if (c == ';') return "up";
  if (c == '.') return "down";
  if (c == ',') return "left";
  if (c == '/') return "right";
  return nullptr;
}

String keyFor(const Keyboard_Class::KeysState& st, char c) {
  const char* arrow = arrowFor(c);
  if (arrow && st.fn) return arrow;
  if (arrow && st.opt) return String("opt+") + arrow;
  if (arrow && st.ctrl) return String("ctrl+") + arrow;
  if (st.ctrl) return String("ctrl+") + c;
  if (st.shift && (c == '`' || c == '~')) return "shift+esc";
  if (c == '`') return "esc";
  return String(c);
}

String currentKey(const Keyboard_Class::KeysState& st) {
  if (!st.word.empty()) return keyFor(st, st.word[0]);
  if (st.del) return "backspace";
  if (st.tab) return "tab";
  if (st.enter && st.shift) return "shift+enter";
  if (st.enter) return "enter";
  return "";
}

void handleKeys(const Keyboard_Class::KeysState& st) {
  for (char c : st.word) {
    if (st.fn && (c == '`' || c == '~')) { sendKey("fn+esc"); return; }
    if (!g_haveTarget) continue;
    sendKey(keyFor(st, c).c_str());
  }
  if (!g_haveTarget) return;
  if (st.del) sendKey("backspace");
  if (st.tab) sendKey("tab");
  if (st.enter && st.shift) sendKey("shift+enter");
  if (st.enter && !st.shift) sendKey("enter");
}

void handleKeyboard() {
  if (!M5Cardputer.Keyboard.isChange()) return;
  Keyboard_Class::KeysState st = M5Cardputer.Keyboard.keysState();
  String cur = currentKey(st);
  if (cur != g_heldKey) releaseKey();
  if (cur.length() == 0) { flushKeyEvents(); return; }
  if (g_power != POWER_ON) applyPower(POWER_ON);
  handleKeys(st);
  flushKeyEvents();
}

#define FULL_PCT            99
#define CHARGE_POLL_S        60
#define CHARGE_POLL_FULL_S  300
#define FULL_FLASH_MS       3000
#define CHARGE_SPLASH_MS    2000
#define BOOT_SPLASH_MS      1500

// Survives deep sleep (RAM is otherwise wiped on reset, lost on power-cycle)
// — the only state that has to carry across a chip reset while the device
// polls its own charge status with everything else powered down.
RTC_DATA_ATTR bool rtc_charging = false;
RTC_DATA_ATTR bool rtc_fullShown = false;

bool alertActive() {
  return g_ledPattern != "off" || g_pendingSound.length() > 0;
}

// True ESP32 sleep — radio off, WebSocket drops. Charging is a physical
// fact independent of whether anyone is watching, so this always wins over
// whatever the server last commanded, session or no session. Never
// returns: the chip resets and setup() decides what to do on wake.
void enterChargeSleep(uint32_t seconds) {
  rtc_charging = true;
  esp_sleep_enable_timer_wakeup((uint64_t)seconds * 1000000ULL);
  esp_deep_sleep_start();
}

// The LED shares its power rail with the backlight (#39 hardware limit), so
// "green at full" needs the rail briefly back on — it cannot be LED-only.
void flashFullGreen() {
  M5Cardputer.Display.setBrightness(BRIGHT_DIM);
  neopixelWrite(RGB_LED_PIN, 0, 255, 0);
  delay(FULL_FLASH_MS);
  neopixelWrite(RGB_LED_PIN, 0, 0, 0);
  M5Cardputer.Display.setBrightness(BRIGHT_OFF);
}

// One splash the moment charge-sleep begins, so plugging in USB gives
// visible confirmation before the screen goes dark for the rest of the
// charge — never repeated on the 60s poll wakes, which stay silent/dark
// so the screen-on time doesn't compete with the battery for USB current.
void showChargingSplash() {
  auto& d = M5Cardputer.Display;
  d.setBrightness(BRIGHT_ON);
  d.fillScreen(COL_BG);
  d.setTextSize(2);
  d.setTextColor(COL_ACCENT, COL_BG);
  d.drawCenterString("Recharging...", SCR_W / 2, 44);
  d.setTextSize(1);
  d.setTextColor(COL_TEXT, COL_BG);
  d.drawCenterString("+" + String(g_batPct) + "%", SCR_W / 2, 76);
  delay(CHARGE_SPLASH_MS);
}

// Called once, from the normally-running state, the moment charging starts
// (or is already true at boot) and no alert is waiting to be seen or heard
// — an unread alert must finish first. Drops the connection deliberately;
// the server queues alerts for exactly this case.
void beginChargeSleep() {
  webSocket.disconnect();
  wsConnected = false;
  WiFi.disconnect(true);
  bool full = g_batPct >= FULL_PCT;
  rtc_fullShown = full;
  if (full) flashFullGreen();
  if (!full) {
    showChargingSplash();
    M5Cardputer.Display.setBrightness(BRIGHT_OFF);
  }
  enterChargeSleep(full ? CHARGE_POLL_FULL_S : CHARGE_POLL_S);
}

// Runs at the very top of setup(), before WiFi/display bring-up, so a
// charge-poll wake never pays for a reconnect it doesn't need. Returning
// true is never actually observed — enterChargeSleep() resets the chip
// instead of returning — but the early-return shape keeps setup() readable.
bool handleChargeWake() {
  bool timerWake = esp_sleep_get_wakeup_cause() == ESP_SLEEP_WAKEUP_TIMER;
  if (!rtc_charging || !timerWake) { rtc_charging = false; return false; }

  bool usb = HWCDC::isPlugged();
  int pct = batteryPercentFromMv(sampleMilliVolts());
  if (!usb) { rtc_charging = false; rtc_fullShown = false; return false; }

  if (pct >= FULL_PCT && !rtc_fullShown) {
    flashFullGreen();
    rtc_fullShown = true;
  }
  enterChargeSleep(rtc_fullShown ? CHARGE_POLL_FULL_S : CHARGE_POLL_S);
  return true;
}

void drawBootSplash() {
  auto& d = M5Cardputer.Display;
  d.fillScreen(COL_BG);
  d.setTextSize(3);
  d.setTextColor(COL_ACCENT, COL_BG);
  d.drawCenterString("cardputerme", SCR_W / 2, 40);
  d.setTextSize(1);
  d.setTextColor(COL_DIM, COL_BG);
  d.drawCenterString("by brutalzinn", SCR_W / 2, 80);
  delay(BOOT_SPLASH_MS);
}

void setup() {
  auto cfg = M5.config();
  M5Cardputer.begin(cfg, true);
  M5Cardputer.Display.setRotation(1);
  M5Cardputer.Speaker.begin();
  M5Cardputer.Speaker.setVolume(140);

  if (handleChargeWake()) return;

  applyPower(POWER_ON);
  drawBootSplash();
  M5Cardputer.Display.fillScreen(COL_BG);

  connectWifi();

  g_udp.begin(BEACON_PORT);
  webSocket.onEvent(onWsEvent);
  webSocket.setReconnectInterval(3000);
  drawSearching();
}

void loop() {
  M5Cardputer.update();
  if (g_haveTarget) webSocket.loop();

  pollBeacons();
  expireFound();
  if (!g_haveTarget && !g_found.empty()) connectToFound(0);
  if (!g_haveTarget && g_listDirty) { g_listDirty = false; drawSearching(); }
  if (g_listDirty && wsConnected) { g_listDirty = false; sendBeacons(); }
  if (g_haveTarget && !wsConnected && foundIndex(g_targetIp, g_targetPort) < 0) leaveServer();

  tickSupply();
  if (g_batUsb && g_batPct >= 0 && !alertActive()) beginChargeSleep();
  tickLed();
  playPending();

  if (M5Cardputer.BtnA.wasPressed()) togglePower();

  handleKeyboard();
  tickStatusMarquee();

  delay(5);
}

