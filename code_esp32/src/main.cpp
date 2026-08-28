#include <WiFi.h>
#include <HTTPClient.h>
#include <GxEPD2_BW.h>
#include <esp_sleep.h>

#define EPD_CS   14
#define EPD_DC   17
#define EPD_RST  16
#define EPD_BUSY 4

const char* ssid     = "Freebox-083257";
const char* password = "9h77rqhr6xqcr2v9q6924s";
const char* imageURL = "http://dashboard-esp32-info.mussetau.fr/api/v1/dashboard.raw";

#define IMG_WIDTH  800
#define IMG_HEIGHT 480
#define IMG_SIZE   (IMG_WIDTH / 8 * IMG_HEIGHT)

#define SLEEP_MINUTES    15
#define uS_TO_S_FACTOR   1000000ULL
#define WIFI_TIMEOUT_MS  15000
#define WIFI_MAX_RETRIES 4

GxEPD2_BW<GxEPD2_750_T7, GxEPD2_750_T7::HEIGHT> display(
    GxEPD2_750_T7(EPD_CS, EPD_DC, EPD_RST, EPD_BUSY));

uint8_t* imageBuffer = nullptr;

bool connectWiFi() {
  WiFi.mode(WIFI_STA);
  WiFi.setSleep(false); // meilleure fiabilité de connexion

  for (int attempt = 1; attempt <= WIFI_MAX_RETRIES; attempt++) {
    Serial.printf("Connexion WiFi (tentative %d/%d)", attempt, WIFI_MAX_RETRIES);
    WiFi.begin(ssid, password);

    unsigned long start = millis();
    while (WiFi.status() != WL_CONNECTED && millis() - start < WIFI_TIMEOUT_MS) {
      delay(500);
      Serial.print(".");
    }

    if (WiFi.status() == WL_CONNECTED) {
      Serial.println(" connecté !");
      return true;
    }

    Serial.println(" échec.");
    WiFi.disconnect(true); // repart de zéro avant de retenter
    delay(2000);           // laisse la box "digérer" le refus
  }
  return false;
}

bool fetchImage() {
  HTTPClient http;
  http.begin(imageURL);
  int httpCode = http.GET();

  if (httpCode != HTTP_CODE_OK) {
    Serial.printf("Erreur HTTP : %d\n", httpCode);
    http.end();
    return false;
  }

  WiFiClient* stream = http.getStreamPtr();
  stream->setTimeout(15000);
  size_t received = stream->readBytes(imageBuffer, IMG_SIZE);

  http.end();
  return received == IMG_SIZE;
}

void goToSleep() {
  Serial.println("Mise en veille pour 15 minutes...");
  WiFi.disconnect(true); // déconnexion propre : évite le refus au prochain réveil
  Serial.flush();
  esp_sleep_enable_timer_wakeup(SLEEP_MINUTES * 60ULL * uS_TO_S_FACTOR);
  esp_deep_sleep_start();
}

void setup() {
  Serial.begin(115200);

  imageBuffer = (uint8_t*) malloc(IMG_SIZE);
  if (!imageBuffer) {
    Serial.println("Échec allocation mémoire");
    goToSleep();
  }

  display.init(115200, true, 2, false);
  display.setRotation(0);

  if (!connectWiFi()) {
    Serial.println("Échec de connexion WiFi après plusieurs tentatives.");
    free(imageBuffer);
    goToSleep();
  }

  if (fetchImage()) {
    display.setFullWindow();
    display.firstPage();
    do {
      display.fillScreen(GxEPD_WHITE);
      display.drawBitmap(0, 0, imageBuffer, IMG_WIDTH, IMG_HEIGHT, GxEPD_BLACK);
    } while (display.nextPage());
    display.hibernate();
    Serial.println("Image affichée.");
  } else {
    Serial.println("Échec de récupération de l'image.");
  }

  free(imageBuffer);
  goToSleep();
}

void loop() {}