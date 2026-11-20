/*
  IoT Checkweigher ESP32 Firmware Reference
  
  Komponen:
  - ESP32 Dev Module
  - LoadCell + HX711 (DT: GPIO 16, SCK: GPIO 4)
  - Rotary Encoder (CLK: GPIO 34, DT: GPIO 35)
  - Infrared Sensor (OUT: GPIO 14)
  - RFID RC522 (SDA/SS: GPIO 5, SCK: GPIO 18, MOSI: GPIO 23, MISO: GPIO 19, RST: GPIO 22)
  - Servo Motor (PWM: GPIO 13)
  - Motor Driver L298N / BTS7960 (PWM: GPIO 25, DIR1: GPIO 26, DIR2: GPIO 27)

  Fitur:
  - Koneksi WiFi dan MQTT ke HiveMQ Cloud (TLS / Port 8883)
  - Scan RFID tag pada barang
  - Deteksi keberadaan barang via sensor Infrared
  - Penimbangan berat barang via LoadCell HX711
  - Klasifikasi 3 kategori:
      * RINGAN (< 300g)       -> Servo belokkan barang ke jalur sortir
      * BERAT (300g - 1000g)  -> Conveyor jalan lurus (servo posisi normal)
      * ERROR (> 1000g)       -> Motor conveyor stop + kirim alert
  - Pengaturan kecepatan motor conveyor dengan kontrol PID berbasis Rotary Encoder
  - Menerima perintah MQTT dari dashboard:
      * checkweigher/command          : START, STOP, TARE
      * checkweigher/config/threshold  : tolerance_min, tolerance_max
      * checkweigher/config/setpoint   : setpoint_rpm
  - Mengirim data ke broker MQTT:
      * checkweigher/weighing         : hasil penimbangan + UID RFID
      * checkweigher/conveyor/rpm      : telemetri PID (actual RPM & PWM)
      * checkweigher/device/status     : status heartbeat sensor & aktuator
*/

#include <WiFi.h>
#include <WiFiClient.h>
#include <PubSubClient.h>
#include <ArduinoJson.h>
#include <SPI.h>
#include <MFRC522.h>
#include <ESP32Servo.h>
#include "HX711.h"

// ================= KONFIGURASI WIFI & HIVEMQ =================
const char* WIFI_SSID     = "YOUR_WIFI_SSID";
const char* WIFI_PASS     = "YOUR_WIFI_PASSWORD";

// Konfigurasi Broker Public HiveMQ (broker.hivemq.com port 1883 TCP / 8883 TLS)
const char* MQTT_BROKER   = "broker.hivemq.com";
const int   MQTT_PORT     = 1883; // 1883 untuk unencrypted (public), 8883 untuk TLS
const char* MQTT_USER     = "";   // Kosongkan untuk broker public
const char* MQTT_PASS     = "";   // Kosongkan untuk broker public
const char* CLIENT_ID     = "ESP32-Checkweigher-Client";

// Topik MQTT
const char* TOPIC_WEIGHING         = "checkweigher/weighing";
const char* TOPIC_DEVICE_STATUS    = "checkweigher/device/status";
const char* TOPIC_CONVEYOR_RPM     = "checkweigher/conveyor/rpm";
const char* TOPIC_COMMAND          = "checkweigher/command";
const char* TOPIC_CONFIG_THRESHOLD = "checkweigher/config/threshold";
const char* TOPIC_CONFIG_SETPOINT  = "checkweigher/config/setpoint";

// ================= PIN DEFINITIONS =================
#define PIN_HX711_DOUT   16
#define PIN_HX711_SCK    4

#define PIN_RFID_SS      5
#define PIN_RFID_RST     22

#define PIN_IR_SENSOR    14
#define PIN_SERVO        13

#define PIN_ENC_CLK      34
#define PIN_ENC_DT       35

#define PIN_MOTOR_PWM    25
#define PIN_MOTOR_DIR1   26
#define PIN_MOTOR_DIR2   27

// ================= OBJEK PERANGKAT =================
// Catatan: Gunakan WiFiClient untuk port 1883 (standar public), atau WiFiClientSecure untuk TLS (port 8883)
WiFiClient net;
PubSubClient mqttClient(net);
HX711 scale;
MFRC522 rfid(PIN_RFID_SS, PIN_RFID_RST);
Servo sortServo;

// ================= PARAMETER SISTEM =================
float toleranceMin = 300.0;    // Batas bawah kategori berat (gram)
float toleranceMax = 1000.0;   // Batas atas sebelum error (gram)
float setpointRPM  = 60.0;     // Target kecepatan konveyor (RPM)

bool conveyorRunning = true;
bool systemActive    = true;

// Posisi Servo
const int SERVO_STRAIGHT = 0;   // Posisi lurus untuk BERAT
const int SERVO_SORT     = 90;  // Posisi belok untuk RINGAN

// PID Control Parameter
float Kp = 2.0;
float Ki = 0.5;
float Kd = 0.1;

float pidError = 0, lastError = 0, integralError = 0, derivativeError = 0;
int motorPWM = 0;

// Encoder Variables
volatile long encoderTicks = 0;
unsigned long lastPIDTime = 0;
float currentRPM = 0.0;
const int ENCODER_PULSES_PER_REV = 20;

// Heartbeat & Telemetry Timer
unsigned long lastHeartbeat = 0;
unsigned long lastRPMTele   = 0;

// State RFID
String scannedRFID = "";

// ================= INTERRUPT SERVICE ROUTINE =================
void IRAM_ATTR onEncoderPulse() {
  encoderTicks++;
}

// ================= SETUP PERANGKAT =================
void setup() {
  Serial.begin(115200);
  Serial.println("[SYSTEM] Inisialisasi Checkweigher ESP32...");

  // Setup Pin Motor
  pinMode(PIN_MOTOR_PWM, OUTPUT);
  pinMode(PIN_MOTOR_DIR1, OUTPUT);
  pinMode(PIN_MOTOR_DIR2, OUTPUT);
  digitalWrite(PIN_MOTOR_DIR1, HIGH);
  digitalWrite(PIN_MOTOR_DIR2, LOW);

  // Setup Pin IR & Encoder
  pinMode(PIN_IR_SENSOR, INPUT);
  pinMode(PIN_ENC_CLK, INPUT_PULLUP);
  pinMode(PIN_ENC_DT, INPUT_PULLUP);
  attachInterrupt(digitalPinToInterrupt(PIN_ENC_CLK), onEncoderPulse, RISING);

  // Setup Servo
  sortServo.attach(PIN_SERVO);
  sortServo.write(SERVO_STRAIGHT);

  // Setup SPI & RFID
  SPI.begin();
  rfid.PCD_Init();
  Serial.println("[RFID] RC522 Reader siap");

  // Setup Loadcell HX711
  scale.begin(PIN_HX711_DOUT, PIN_HX711_SCK);
  scale.set_scale(420.0983); // Kalibrasi faktor sesuai sensor Anda
  scale.tare();              // Reset ke 0 gram saat startup
  Serial.println("[LOADCELL] HX711 siap dan di-tare");

  // Koneksi WiFi
  setupWiFi();

  // Konfigurasi MQTT (jika memakai WiFiClientSecure untuk TLS port 8883, gunakan net.setInsecure())
  mqttClient.setServer(MQTT_BROKER, MQTT_PORT);
  mqttClient.setCallback(onMqttMessageReceived);
  mqttClient.setBufferSize(1024);

  connectMQTT();
}

// ================= MAIN LOOP =================
void loop() {
  if (!mqttClient.connected()) {
    connectMQTT();
  }
  mqttClient.loop();

  unsigned long now = millis();

  // 1. Baca RFID jika ada kartu/tag mendekat
  readRFIDTag();

  // 2. Loop Kontrol PID Conveyor (setiap 100ms)
  if (now - lastPIDTime >= 100) {
    calculatePID();
    lastPIDTime = now;
  }

  // 3. Cek Sensor IR untuk mendeteksi kehadiran barang di zona timbang
  if (systemActive && digitalRead(PIN_IR_SENSOR) == LOW) { // Asumsi Active LOW saat objek ada
    handleWeighingProcess();
  }

  // 4. Kirim Telemetri RPM ke MQTT setiap 500ms
  if (now - lastRPMTele >= 500) {
    sendRPMTelemetry();
    lastRPMTele = now;
  }

  // 5. Kirim Heartbeat Status Device setiap 5000ms (5 detik)
  if (now - lastHeartbeat >= 5000) {
    sendDeviceStatus();
    lastHeartbeat = now;
  }
}

// ================= PROSES PENIMBANGAN =================
void handleWeighingProcess() {
  Serial.println("[WEIGH] Barang terdeteksi oleh Sensor IR di zona timbang");

  // Hentikan konveyor sejenak untuk menimbang stabil
  setMotorSpeed(0);
  delay(300);

  // Baca berat dari loadcell (rata-rata 5 pembacaan)
  float weight = scale.get_units(5);
  if (weight < 0) weight = 0.0;

  Serial.printf("[WEIGH] Berat terukur: %.2f gram\n", weight);

  String category = "BERAT";

  if (weight > toleranceMax) {
    // Kategori: ERROR (> 1000g)
    category = "ERROR";
    Serial.println("[WEIGH] ALERT: Berat melebihi kapasitas (>1000g)! Conveyor dihentikan.");
    conveyorRunning = false;
    setMotorSpeed(0);
  } else if (weight < toleranceMin) {
    // Kategori: RINGAN (< 300g) -> Belokkan ke jalur sortir
    category = "RINGAN";
    Serial.println("[WEIGH] Kategori: RINGAN. Menggerakkan Servo ke posisi sortir...");
    sortServo.write(SERVO_SORT);
    delay(400);

    // Jalankan konveyor untuk mendorong barang ke jalur sortir
    setMotorSpeed(motorPWM);
    delay(1200);

    // Kembalikan servo ke posisi lurus
    sortServo.write(SERVO_STRAIGHT);
  } else {
    // Kategori: BERAT (300g - 1000g) -> Lurus
    category = "BERAT";
    Serial.println("[WEIGH] Kategori: BERAT. Melanjutkan konveyor lurus...");
    sortServo.write(SERVO_STRAIGHT);
    setMotorSpeed(motorPWM);
    delay(1000);
  }

  // Kirim data penimbangan ke MQTT
  sendWeighingData(weight, currentRPM);

  // Reset tag RFID setelah selesai proses penimbangan barang tersebut
  scannedRFID = "";

  // Nyalakan kembali conveyor jika tidak error
  if (conveyorRunning) {
    setMotorSpeed(motorPWM);
  }
}

// ================= BACA RFID TAG =================
void readRFIDTag() {
  if (!rfid.PICC_IsNewCardPresent()) return;
  if (!rfid.PICC_ReadCardSerial()) return;

  String uidStr = "";
  for (byte i = 0; i < rfid.uid.size; i++) {
    if (rfid.uid.uidByte[i] < 0x10) uidStr += "0";
    uidStr += String(rfid.uid.uidByte[i], HEX);
  }
  uidStr.toUpperCase();
  scannedRFID = uidStr;

  Serial.print("[RFID] Tag terdeteksi. UID: ");
  Serial.println(scannedRFID);

  rfid.PICC_HaltA();
  rfid.PCD_StopCrypto1();
}

// ================= KONTROL PID MOTOR CONVEYOR =================
void calculatePID() {
  if (!conveyorRunning) {
    setMotorSpeed(0);
    currentRPM = 0.0;
    encoderTicks = 0;
    return;
  }

  // Hitung RPM aktual dari encoder
  long ticks = encoderTicks;
  encoderTicks = 0;

  // 100ms = 0.1 detik. (ticks / pulse_per_rev) / (0.1 / 60)
  currentRPM = ((float)ticks / ENCODER_PULSES_PER_REV) * 600.0;

  // Hitung Error PID
  pidError = setpointRPM - currentRPM;
  integralError += pidError * 0.1;
  derivativeError = (pidError - lastError) / 0.1;

  float output = (Kp * pidError) + (Ki * integralError) + (Kd * derivativeError);
  motorPWM = constrain((int)output + 120, 0, 255); // Basis offset 120 untuk overcome friksi

  lastError = pidError;
  setMotorSpeed(motorPWM);
}

void setMotorSpeed(int pwm) {
  analogWrite(PIN_MOTOR_PWM, pwm);
}

// ================= PENGIRIMAN MQTT DATA =================
void sendWeighingData(float weight, float speed) {
  StaticJsonDocument<256> doc;
  doc["rfid_uid"]       = scannedRFID.length() > 0 ? scannedRFID : "UNKNOWN";
  doc["weight"]         = weight;
  doc["unit"]           = "gram";
  doc["conveyor_speed"] = speed;
  doc["timestamp"]      = "";

  char buffer[256];
  serializeJson(doc, buffer);
  mqttClient.publish(TOPIC_WEIGHING, buffer);
  Serial.printf("[MQTT PUB] Weighing data terkirim: %s\n", buffer);
}

void sendRPMTelemetry() {
  StaticJsonDocument<128> doc;
  doc["actual_rpm"] = currentRPM;
  doc["pwm_value"]  = motorPWM;
  doc["timestamp"]  = "";

  char buffer[128];
  serializeJson(doc, buffer);
  mqttClient.publish(TOPIC_CONVEYOR_RPM, buffer);
}

void sendDeviceStatus() {
  StaticJsonDocument<512> doc;
  JsonArray arr = doc.to<JsonArray>();

  JsonObject lc = arr.createNestedObject();
  lc["device_name"] = "LoadCell HX711";
  lc["device_type"] = "loadcell";
  lc["status"]      = scale.is_ready() ? "ONLINE" : "ERROR";
  lc["last_value"]  = String(scale.get_units(1), 1) + " g";

  JsonObject enc = arr.createNestedObject();
  enc["device_name"] = "Rotary Encoder";
  enc["device_type"] = "encoder";
  enc["status"]      = "ONLINE";
  enc["last_value"]  = String(currentRPM, 1) + " RPM";

  JsonObject srv = arr.createNestedObject();
  srv["device_name"] = "Servo SG90";
  srv["device_type"] = "servo";
  srv["status"]      = "ONLINE";
  srv["last_value"]  = String(sortServo.read()) + " deg";

  JsonObject ir = arr.createNestedObject();
  ir["device_name"] = "IR Sensor";
  ir["device_type"] = "infrared";
  ir["status"]      = "ONLINE";
  ir["last_value"]  = digitalRead(PIN_IR_SENSOR) == LOW ? "TRIGGERED" : "IDLE";

  JsonObject mot = arr.createNestedObject();
  mot["device_name"] = "DC Motor";
  mot["device_type"] = "motor";
  mot["status"]      = conveyorRunning ? "ONLINE" : "OFFLINE";
  mot["last_value"]  = "PWM: " + String(motorPWM);

  JsonObject rf = arr.createNestedObject();
  rf["device_name"] = "RFID Reader";
  rf["device_type"] = "rfid";
  rf["status"]      = "ONLINE";
  rf["last_value"]  = scannedRFID.length() > 0 ? scannedRFID : "IDLE";

  char buffer[512];
  serializeJson(doc, buffer);
  mqttClient.publish(TOPIC_DEVICE_STATUS, buffer);
}

// ================= PENANGANAN PESAN MQTT (INCOMING) =================
void onMqttMessageReceived(char* topic, byte* payload, unsigned int length) {
  String message = "";
  for (unsigned int i = 0; i < length; i++) {
    message += (char)payload[i];
  }
  Serial.printf("[MQTT RECV] [%s] %s\n", topic, message.c_str());

  StaticJsonDocument<256> doc;
  DeserializationError error = deserializeJson(doc, message);
  if (error) {
    Serial.println("[MQTT] Gagal parse JSON incoming");
    return;
  }

  // 1. Perintah Command (START / STOP / TARE)
  if (strcmp(topic, TOPIC_COMMAND) == 0) {
    const char* cmd = doc["command"];
    if (cmd) {
      if (strcmp(cmd, "START") == 0) {
        conveyorRunning = true;
        systemActive = true;
        Serial.println("[COMMAND] Conveyor Dimulai (START)");
      } else if (strcmp(cmd, "STOP") == 0) {
        conveyorRunning = false;
        setMotorSpeed(0);
        Serial.println("[COMMAND] Conveyor Dihentikan (STOP)");
      } else if (strcmp(cmd, "TARE") == 0) {
        scale.tare();
        Serial.println("[COMMAND] Loadcell di-TARE ke 0.0g");
      }
    }
  }

  // 2. Update Threshold Konfigurasi
  else if (strcmp(topic, TOPIC_CONFIG_THRESHOLD) == 0) {
    if (doc.containsKey("tolerance_min") && doc.containsKey("tolerance_max")) {
      toleranceMin = doc["tolerance_min"];
      toleranceMax = doc["tolerance_max"];
      Serial.printf("[CONFIG] Threshold diperbarui: Min=%.1fg, Max=%.1fg\n", toleranceMin, toleranceMax);
    }
  }

  // 3. Update Setpoint RPM
  else if (strcmp(topic, TOPIC_CONFIG_SETPOINT) == 0) {
    if (doc.containsKey("setpoint_rpm")) {
      setpointRPM = doc["setpoint_rpm"];
      Serial.printf("[CONFIG] Setpoint RPM diperbarui: %.1f RPM\n", setpointRPM);
    }
  }
}

// ================= KONEKSI JARINGAN =================
void setupWiFi() {
  delay(10);
  Serial.println();
  Serial.print("[WIFI] Menghubungkan ke ");
  Serial.println(WIFI_SSID);

  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASS);

  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }

  Serial.println("\n[WIFI] Terhubung!");
  Serial.print("[WIFI] Alamat IP: ");
  Serial.println(WiFi.localIP());
}

void connectMQTT() {
  while (!mqttClient.connected()) {
    Serial.print("[MQTT] Menghubungkan ke broker HiveMQ...");
    bool connected = false;
    if (strlen(MQTT_USER) > 0) {
      connected = mqttClient.connect(CLIENT_ID, MQTT_USER, MQTT_PASS);
    } else {
      connected = mqttClient.connect(CLIENT_ID);
    }

    if (connected) {
      Serial.println(" Berhasil!");

      // Subscribe topik perintah dan konfigurasi
      mqttClient.subscribe(TOPIC_COMMAND);
      mqttClient.subscribe(TOPIC_CONFIG_THRESHOLD);
      mqttClient.subscribe(TOPIC_CONFIG_SETPOINT);
      Serial.println("[MQTT] Berlangganan topik perintah selesai");
    } else {
      Serial.print(" Gagal, rc=");
      Serial.print(mqttClient.state());
      Serial.println(" Coba lagi dalam 3 detik...");
      delay(3000);
    }
  }
}

