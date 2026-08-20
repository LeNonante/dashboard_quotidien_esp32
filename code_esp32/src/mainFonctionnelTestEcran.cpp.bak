#include <GxEPD2_BW.h>
#include <Fonts/FreeMonoBold9pt7b.h>

#define EPD_CS   14
#define EPD_DC   17
#define EPD_RST  16
#define EPD_BUSY 4

GxEPD2_BW<GxEPD2_750_T7, GxEPD2_750_T7::HEIGHT> display(
    GxEPD2_750_T7(EPD_CS, EPD_DC, EPD_RST, EPD_BUSY));

void setup() {
  // 2ms de reset au lieu des 10ms par défaut : nécessaire sur les HAT Waveshare
  display.init(115200, true, 2, false);

  display.setRotation(1);
  display.setFont(&FreeMonoBold9pt7b);
  display.setTextColor(GxEPD_BLACK);
  display.setFullWindow();
  display.firstPage();
  do {
    display.fillScreen(GxEPD_WHITE);
    display.setCursor(50, 100);
    display.print("Hello FireBeetle 2!");
  } while (display.nextPage());

  display.hibernate();
}

void loop() {}