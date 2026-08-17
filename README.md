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

les differentes fonctions sont séparées dans les fichiers go.

Pour google :
Rends-toi sur la Google Cloud Console.

    Crée un nouveau projet (ou sélectionne le projet existant dédié à ton dashboard).

    Dans la barre de recherche en haut, ou via le menu de gauche, navigue vers API et services > Bibliothèque.

    Cherche Google Calendar API et clique sur le bouton bleu Activer.

Étape 2 : Création du Compte de Service

    Dans le menu de gauche de la console Google Cloud, clique sur API et services > Identifiants.

    En haut de l'écran, clique sur + CRÉER DES IDENTIFIANTS et sélectionne Compte de service.

    Remplis le champ Nom du compte de service (ex: lecteur-agenda). Une adresse e-mail spécifique (se terminant par ...iam.gserviceaccount.com) se générera automatiquement juste en dessous.

    Clique sur Créer et continuer, puis clique simplement sur OK tout en bas. (Il n'est pas nécessaire d'attribuer des rôles complexes pour ce cas d'usage).

Étape 3 : Génération et sauvegarde de la clé secrète (JSON)

    De retour sur la page Identifiants, fais défiler la page jusqu'à la section Comptes de service.

    Clique sur l'adresse e-mail du compte que tu viens de créer.

    Sur la nouvelle page, navigue vers l'onglet CLÉS (en haut).

    Clique sur le bouton déroulant Ajouter une clé, puis choisis Créer une clé.

    Laisse le format sur JSON et valide en cliquant sur Créer.

    Le fichier se télécharge automatiquement sur ton ordinateur. Renomme-le immédiatement en credentials.json et place-le à la racine de ton projet de code.

    Note de sécurité : Ce fichier est l'équivalent d'un mot de passe. Il ne doit jamais être partagé publiquement ou poussé sur un dépôt GitHub public.

Etape 4 : Partage des agendas au compte de service

Le compte de service est un robot virtuel. Par défaut, il n'a accès à aucun agenda privé. Il faut l'inviter manuellement sur chaque agenda souhaité.

    Copie l'adresse e-mail complète de ton compte de service (lecteur-agenda@...iam.gserviceaccount.com).

    Ouvre l'interface web classique de Google Agenda dans ton navigateur.

    Dans la colonne de gauche, sous Mes agendas, survole le calendrier que tu souhaites lire et clique sur les trois petits points verticaux (⋮).

    Sélectionne Paramètres et partage.

    Fais défiler la page jusqu'à la section Partager avec des personnes ou des groupes spécifiques.

    Clique sur le bouton Ajouter des personnes et des groupes.

    Colle l'adresse e-mail du compte de service et attribue-lui au minimum l'autorisation Afficher les détails des événements.

    Répète cette opération (étapes 3 à 7) pour chaque agenda supplémentaire que ton code devra interroger.

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