#include <Arduino.h>
#include "state_machine.h"

static StateMachine stateMachine;

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

#ifdef PIN_LED
    digitalWrite(PIN_LED, LOW);
#endif
}

void loop() {
    stateMachine.update();
}
