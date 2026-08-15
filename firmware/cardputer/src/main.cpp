#include <M5Cardputer.h>
#include <WiFi.h>
#include <WiFiUdp.h>
#include <WebSocketsClient.h>
#include <ArduinoJson.h>
#include <vector>

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

#define COL_BG      0x0000
#define COL_HDR     0x10A2
#define COL_TEXT    0xFFFF
#define COL_ACCENT  0x07FF
#define COL_DIM     0x8410
#define COL_OK      0x07E6
#define COL_WARN    0xFD20

struct Line { String text; uint16_t color; };
std::vector<Line> g_lines;
String   g_status = "";
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
bool g_autoConnect = true;
IPAddress g_targetIp;
uint16_t g_targetPort = 0;
String g_targetName = "";
bool g_listDirty = false;

String toast = "";
unsigned long toastUntil = 0;

void showToast(const String& msg, uint16_t ms = 1200) {
  toast = msg;
  toastUntil = millis() + ms;
}

bool wifiUp() { return WiFi.status() == WL_CONNECTED; }

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

void drawHeader() {
  auto& d = M5Cardputer.Display;
  d.fillRect(0, 0, SCR_W, HEADER_H, COL_HDR);
  d.setTextSize(1);

  d.setTextColor(wifiUp() ? COL_OK : COL_WARN, COL_HDR);
  d.setCursor(3, 4);
  d.print(wifiUp() ? "WiFi" : "NoWiFi");
  d.setTextColor(COL_ACCENT, COL_HDR);
  d.print(wsConnected ? " LIVE" : " ....");

  if (millis() < toastUntil && toast.length()) {
    d.setTextColor(COL_OK, COL_HDR);
    d.setCursor(66, 4);
    d.print(toast);
  }

  int batt = M5Cardputer.Power.getBatteryLevel();
  String pos = String(g_lines.empty() ? 0 : g_page + 1) + "/" + String(pageCount());
  String tail = String(batt) + "% " + pos;
  d.setCursor(SCR_W - (int)tail.length() * 6 - 3, 4);
  d.setTextColor(batt > 20 ? COL_OK : COL_WARN, COL_HDR);
  d.print(String(batt) + "%");
  d.setTextColor(COL_TEXT, COL_HDR);
  d.print(" " + pos);
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

void drawServerList() {
  auto& d = M5Cardputer.Display;
  d.fillScreen(COL_BG);
  drawHeader();
  d.setTextSize(2);
  d.setTextColor(COL_ACCENT, COL_BG);
  d.setCursor(3, HEADER_H + 4);
  d.print("cardputerme");
  d.setTextSize(1);
  if (g_found.empty()) {
    d.setTextColor(COL_DIM, COL_BG);
    d.setCursor(3, HEADER_H + 28);
    d.print("Listening for terminals...");
    d.setCursor(3, HEADER_H + 40);
    d.print("On the computer: cardputerme");
    return;
  }
  int y = HEADER_H + 26;
  for (size_t i = 0; i < g_found.size() && i < 9; i++) {
    if (y + 22 > bodyBottom()) break;
    d.setTextColor(COL_TEXT, COL_BG);
    d.setCursor(3, y);
    d.print(String((int)i + 1) + ". " + g_found[i].name);
    d.setTextColor(COL_DIM, COL_BG);
    d.setCursor(15, y + 10);
    d.print(g_found[i].ip.toString() + ":" + String(g_found[i].port));
    y += 24;
  }
}

void connectToFound(int idx) {
  if (idx < 0 || idx >= (int)g_found.size()) return;
  g_targetIp = g_found[idx].ip;
  g_targetPort = g_found[idx].port;
  g_targetName = g_found[idx].name;
  g_haveTarget = true;
  g_lines.clear();
  g_status = "";
  webSocket.disconnect();
  webSocket.begin(g_targetIp.toString(), g_targetPort, WS_PATH);
  showToast(g_targetName);
  M5Cardputer.Display.fillScreen(COL_BG);
  redraw();
}

void leaveServer() {
  g_haveTarget = false;
  g_autoConnect = false;
  wsConnected = false;
  webSocket.disconnect();
  g_lines.clear();
  g_status = "";
  drawServerList();
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

void applyDisplay(JsonDocument& doc) {
  g_sessionExists = doc["sessionExists"] | true;
  g_size = doc["size"] | 2;
  JsonObject status = doc["status"];
  String newStatus = String((const char*)(status["text"] | ""));
  if (newStatus != g_status) g_statusOffset = 0;
  g_status = newStatus;
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

void onWsEvent(WStype_t type, uint8_t* payload, size_t length) {
  switch (type) {
    case WStype_CONNECTED:
      wsConnected = true;
      showToast("Connected");
      redraw();
      break;
    case WStype_DISCONNECTED:
      wsConnected = false;
      if (!g_haveTarget) break;
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

      return;
    }
    default:
      break;
  }
}

void sendKey(const char* key) {
  if (!wsConnected) return;
  JsonDocument doc;
  doc["type"] = "key";
  doc["key"] = key;
  String body;
  serializeJson(doc, body);
  webSocket.sendTXT(body);
}

const char* arrowFor(char c) {
  if (c == ';') return "up";
  if (c == '.') return "down";
  if (c == ',') return "left";
  if (c == '/') return "right";
  return nullptr;
}

void handleKeys(const Keyboard_Class::KeysState& st) {
  for (char c : st.word) {
    if (st.fn && (c == '`' || c == '~')) { leaveServer(); return; }
    if (!g_haveTarget) {
      if (c >= '1' && c <= '9') connectToFound(c - '1');
      continue;
    }
    const char* arrow = arrowFor(c);
    if (arrow && st.ctrl && st.fn) { sendKey((String("ctrl+fn+") + arrow).c_str()); continue; }
    if (arrow && st.fn) { sendKey(arrow); continue; }
    if (arrow && st.opt) { sendKey((String("opt+") + arrow).c_str()); continue; }
    if (arrow && st.ctrl) { sendKey((String("ctrl+") + arrow).c_str()); continue; }
    if (st.ctrl) {
      char combo[8] = { 'c', 't', 'r', 'l', '+', c, 0 };
      sendKey(combo);
      continue;
    }

    if (st.shift && (c == '`' || c == '~')) { sendKey("shift+esc"); continue; }
    if (c == '`') { sendKey("esc"); continue; }
    char one[2] = { c, 0 };
    sendKey(one);
  }
  if (!g_haveTarget) return;
  if (st.del) sendKey("backspace");
  if (st.tab) sendKey("tab");
  if (st.enter && st.shift) sendKey("shift+enter");
  if (st.enter && !st.shift) sendKey("enter");
}

void setup() {
  auto cfg = M5.config();
  M5Cardputer.begin(cfg, true);
  M5Cardputer.Display.setRotation(1);
  M5Cardputer.Speaker.begin();
  M5Cardputer.Speaker.setVolume(140);
  M5Cardputer.Display.fillScreen(COL_BG);

  connectWifi();

  g_udp.begin(BEACON_PORT);
  webSocket.onEvent(onWsEvent);
  webSocket.setReconnectInterval(3000);
  drawServerList();
}

void loop() {
  M5Cardputer.update();
  if (g_haveTarget) webSocket.loop();

  pollBeacons();
  expireFound();
  if (!g_haveTarget) {
    if (g_autoConnect && g_found.size() == 1) connectToFound(0);
    if (!g_haveTarget && g_listDirty) { g_listDirty = false; drawServerList(); }
  }
  if (g_haveTarget && g_listDirty) g_listDirty = false;
  if (g_haveTarget && !wsConnected && foundIndex(g_targetIp, g_targetPort) < 0) leaveServer();

  if (M5Cardputer.Keyboard.isChange() && M5Cardputer.Keyboard.isPressed()) {
    Keyboard_Class::KeysState st = M5Cardputer.Keyboard.keysState();
    handleKeys(st);
  }

  static bool toastWasShown = false;
  bool toastShowing = (millis() < toastUntil) && toast.length();
  if (toastWasShown && !toastShowing) drawHeader();
  toastWasShown = toastShowing;

  static unsigned long battTick = 0;
  if (millis() - battTick > 15000) {
    battTick = millis();
    drawHeader();
  }

  tickStatusMarquee();

  delay(5);
}

