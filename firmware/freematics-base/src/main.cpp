#include <Arduino.h>
#include "state_machine.h"

static StateMachine stateMachine;

void setup() {
    Serial.begin(115200);
    delay(500);

    Serial.println();
    Serial.println("========================================");
    Serial.println("  Cairn v0.1 -- Offline Car Journal");
    Serial.println("  Freematics ONE+ Model B (ESP32)");
    Serial.println("========================================");
    Serial.println();

    stateMachine.init();
}

void loop() {
    stateMachine.update();
    delay(10);
}
