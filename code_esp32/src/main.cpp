#include <WiFi.h>
#include <HTTPClient.h>
#include <GxEPD2_BW.h>

#define EPD_CS   14
#define EPD_DC   17
#define EPD_RST  16
#define EPD_BUSY 4

const char* ssid     = "Airbox_BE0A";
const char* password = "T9FBbegA8gdN";
const char* imageURL = "http://dashboard-esp32-info.mussetau.fr/api/v1/dashboard.raw"; // URL de l'image à récupérer

#define IMG_WIDTH  800
#define IMG_HEIGHT 480
#define IMG_SIZE   (IMG_WIDTH / 8 * IMG_HEIGHT) // 48000 octets

GxEPD2_BW<GxEPD2_750_T7, GxEPD2_750_T7::HEIGHT> display(
    GxEPD2_750_T7(EPD_CS, EPD_DC, EPD_RST, EPD_BUSY));

uint8_t* imageBuffer = nullptr;

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

void setup() {
  Serial.begin(115200);

  imageBuffer = (uint8_t*) malloc(IMG_SIZE);
  if (!imageBuffer) {
    Serial.println("Échec allocation mémoire");
    return;
  }

  display.init(115200, true, 2, false);
  display.setRotation(0); // important, voir remarque plus bas

  WiFi.begin(ssid, password);
  Serial.print("Connexion WiFi");
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }
  Serial.println(" connecté !");

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
}

void loop() {}