# Dashboard E-Ink (ESP32 + Go)

L'idée de base : **déporter toute la complexité sur le serveur**. L'ESP32 n'a pas assez de RAM pour calculer de belles mises en page. C'est donc la VM qui fait tout le travail de design, génère une image parfaite, et l'ESP32 se contente de l'afficher et de retourner dormir.

---

## 🏗️ Architecture du Projet

1. **Le Serveur (Go)** : 
   - Crée une page HTML interne invisible sur le réseau.
   - Utilise `chromedp` pour simuler un navigateur, ouvrir la page HTML et prendre un screenshot à la résolution exacte de l'écran Waveshare (800x480).
   - Expose une route API protégée pour distribuer cette image.
2. **Le Client (ESP32)** :
   - Se réveille du Deep Sleep.
   - Calcule un jeton de sécurité avec l'heure exacte (HMAC).
   - Télécharge l'image, rafraîchit l'encre électronique, et s'éteint.

---

## 🖥️ Partie 1 : Le Serveur (Backend Go)

Tout est dans le dossier `serveur_go`. C'est un exécutable autonome, ultra-léger.

### Comment tester le design sur Windows
Si je veux modifier la mise en page (ajouter le calendrier, les stats Strava/Altarun, etc.) sans avoir à flasher l'ESP32 à chaque fois :

1. Dans `main.go`, je désactive temporairement la sécurité HMAC en bas du code.
2. Je lance le serveur :
   ```bash
   cd serveur_go
   go run main.go
   ```

3. Je vais sur http://localhost:8080 dans mon navigateur. C'est là que je peux voir et modifier le HTML/CSS en direct.

4. Je vais sur http://localhost:8081/api/v1/dashboard.png pour voir le résultat brut (l'image générée par Chromium).


### Déploiement en Production

Comme l'objectif est d'avoir tous les services de prod centralisés sur une seule VM :

1. Compiler le code pour Linux.

2. S'assurer que le paquet chromium (ou chromium-browser) est installé sur la VM pour que chromedp puisse faire les captures.

3. Faire tourner le binaire en tâche de fond. Le serveur n'expose le HTML qu'en boucle locale (127.0.0.1), donc aucun risque que quelqu'un lise les données sur le réseau.

### Partie 2 : Le Client (ESP32 + Waveshare 7.5")
## Câblage SPI Standard
- VCC -> 3.3V (Surtout pas 5V !)
- GND -> GND
- DIN -> GPIO 23
- CLK -> GPIO 18
- CS -> GPIO 5
- DC -> GPIO 17
- RST -> GPIO 16
- BUSY -> GPIO 4

## La logique du code C++

Le code dans la fonction loop() est vide. Tout se passe dans le setup() en mode One-Shot :
- Boot & Wi-Fi : L'ESP32 s'allume et se connecte au réseau.
- NTP Sync : Il interroge un serveur de temps pour récupérer l'heure exacte à la seconde près.
- Sécurité (HMAC-SHA256) : Il prend ce timestamp, le mixe avec la secretKey (la même que dans le code Go), et l'envoie dans l'URL. Si le serveur Go voit que le timestamp a plus de 60 secondes ou que la signature est fausse, il bloque l'accès.
- Affichage : Utilise la librairie GxEPD2 (beaucoup plus stable que celle de Waveshare) pour "imprimer" l'image pixel par pixel.
- Deep Sleep : L'ESP se met en veille profonde pendant 1 ou 2 heures. L'écran E-ink garde l'image affichée sans consommer le moindre courant.

### Librairies à inclure dans l'IDE Arduino / PlatformIO
- GxEPD2 (Pour gérer l'écran)
- HTTPClient & WiFi
- Bibliothèque pour NTP et crypto SHA256