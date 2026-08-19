#include <Arduino.h>
#include <GxEPD2_BW.h>
#include <Fonts/FreeSans12pt7b.h> // Police incluse par défaut avec Adafruit_GFX

// put function declarations here:
int myFunction(int, int);



// --- CONFIGURATION DES BROCHES ---
// À adapter selon ta carte ! 
// Voici un exemple de câblage classique. Si tu as relié les câbles du HAT 
// différemment sur ton FireBeetle, modifie ces numéros :
const int EPD_CS = 15;
const int EPD_DC = 14;
const int EPD_RST = 26;
const int EPD_BUSY = 25;

// Initialisation de la dalle 7.5 pouces Noir & Blanc
GxEPD2_BW<GxEPD2_750_T7, GxEPD2_750_T7::HEIGHT> display(GxEPD2_750_T7(EPD_CS, EPD_DC, EPD_RST, EPD_BUSY));

void setup() {
  // put your setup code here, to run once:
  int result = myFunction(2, 3);


  Serial.begin(115200);
  delay(1000);
  Serial.println("Démarrage du test de l'écran...");

  // 1. Allumage de l'écran
  display.init(115200, true, 2, false);

  // 2. Configuration du texte
  display.setRotation(0); // 0 = Mode Paysage
  display.setFont(&FreeSans12pt7b);
  display.setTextColor(GxEPD_BLACK);

  // 3. Dessin sur l'écran
  display.setFullWindow();
  display.firstPage();
  do {
    display.fillScreen(GxEPD_WHITE); // Fond blanc
    
    // On se place au milieu de l'écran (qui fait 800x480)
    display.setCursor(250, 240); 
    display.print("L'ecran fonctionne parfaitement !");
    
    // Ajout d'un petit rectangle décoratif autour
    display.drawRect(230, 200, 380, 60, GxEPD_BLACK);

  } while (display.nextPage());

  Serial.println("Test terminé. Mise en veille de l'écran.");
  
  // 4. Extinction propre pour protéger la dalle
  display.hibernate(); 

}

void loop() {
  // put your main code here, to run repeatedly:

}

// put function definitions here:
int myFunction(int x, int y) {
  return x + y;
}