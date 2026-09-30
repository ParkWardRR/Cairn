#include <Arduino.h>
#include "state_machine.h"

static StateMachine stateMachine;

#if CAIRN_NET_SELFTEST
#include <WiFi.h>
#include "config.h"

// Bench-only check: proves the Wi-Fi credentials, DNS and the sync server are
// all reachable from the device before committing to a drive. Built via the
// freematics-selftest env; never compiled into the production binary.
static void netSelfTest() {
    Serial.println("[SELFTEST] Wi-Fi + server reachability");

    WiFi.mode(WIFI_STA);
    WiFi.disconnect(true);
    delay(200);

    // The ESP32 is 2.4 GHz only, so list what the radio can actually see.
    int found = WiFi.scanNetworks();
    Serial.printf("[SELFTEST] scan found %d network(s)\n", found);
    bool targetVisible = false;
    for (int i = 0; i < found; i++) {
        bool match = (WiFi.SSID(i) == WIFI_SSID);
        if (match) targetVisible = true;
        Serial.printf("[SELFTEST]   %-32s ch%-3d %4d dBm enc=%d%s\n",
                      WiFi.SSID(i).c_str(), WiFi.channel(i), WiFi.RSSI(i),
                      (int)WiFi.encryptionType(i), match ? "  <-- target" : "");
    }
    WiFi.scanDelete();
    if (!targetVisible) {
        Serial.printf("[SELFTEST] FAIL: \"%s\" not visible on 2.4 GHz\n", WIFI_SSID);
        return;
    }

    WiFi.begin(WIFI_SSID, WIFI_PASSWORD);

    unsigned long t = millis();
    while (WiFi.status() != WL_CONNECTED && millis() - t < 20000) {
        delay(500);
    }
    if (WiFi.status() != WL_CONNECTED) {
        Serial.printf("[SELFTEST] FAIL: could not join \"%s\" (status %d)\n",
                      WIFI_SSID, (int)WiFi.status());
        return;
    }
    Serial.printf("[SELFTEST] joined \"%s\"  IP %s  RSSI %d dBm\n",
                  WIFI_SSID, WiFi.localIP().toString().c_str(), WiFi.RSSI());
    Serial.printf("[SELFTEST] DNS server %s\n", WiFi.dnsIP().toString().c_str());

    IPAddress addr;
    if (!WiFi.hostByName(SERVER_HOSTNAME, addr)) {
        Serial.printf("[SELFTEST] FAIL: could not resolve %s\n", SERVER_HOSTNAME);
        return;
    }
    Serial.printf("[SELFTEST] %s -> %s\n", SERVER_HOSTNAME, addr.toString().c_str());

    WiFiClient client;
    if (!client.connect(addr, SERVER_PORT, 8000)) {
        Serial.printf("[SELFTEST] FAIL: could not connect to %s:%u\n",
                      SERVER_HOSTNAME, SERVER_PORT);
        return;
    }
    client.printf("GET /api/v1/health HTTP/1.1\r\n"
                  "Host: %s:%u\r\nConnection: close\r\n\r\n",
                  SERVER_HOSTNAME, SERVER_PORT);

    t = millis();
    while (!client.available() && millis() - t < 8000) {
        delay(50);
    }
    if (!client.available()) {
        Serial.println("[SELFTEST] FAIL: no response from server");
        client.stop();
        return;
    }
    Serial.println("[SELFTEST] response:");
    while (client.available()) {
        Serial.write(client.read());
    }
    Serial.println();
    client.stop();
    Serial.println("[SELFTEST] PASS: server reachable end to end");
}
#endif

void setup() {
    Serial.begin(115200);
    delay(500);

    Serial.println();
    Serial.println("========================================");
    Serial.println("  Cairn v0.2 -- Offline Car Journal");
    Serial.println("  Freematics ONE+ Model B (ESP32)");
    Serial.println("========================================");
    Serial.println();

    Serial.printf("CPU: %u MHz  Flash: %u MB\n",
                  ESP.getCpuFreqMHz(), ESP.getFlashChipSize() >> 20);
    Serial.printf("Heap: %u KB\n", ESP.getHeapSize() >> 10);
#if BOARD_HAS_PSRAM
    if (psramInit()) {
        Serial.printf("PSRAM: %u MB\n", esp_spiram_get_size() >> 20);
    }
#endif
    Serial.println();

#ifdef PIN_LED
    pinMode(PIN_LED, OUTPUT);
    digitalWrite(PIN_LED, HIGH);
#endif

    stateMachine.init();

#if CAIRN_NET_SELFTEST
    netSelfTest();
#endif

#ifdef PIN_LED
    digitalWrite(PIN_LED, LOW);
#endif
}

void loop() {
    stateMachine.update();
}
