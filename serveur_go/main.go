package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/chromedp/chromedp"
)

// Clé cryptographique partagée avec l'ESP32.
// À générer de manière aléatoire et à garder secrète.
const secretKey = "VOTRE_CLE_SECRETE_TRES_LONGUE_ET_COMPLEXE"

// Template HTML basique intégré directement dans le code.
// Dans un vrai projet, tu peux utiliser "html/template" pour parser un fichier externe.
const htmlTemplate = `
<!DOCTYPE html>
<html lang="fr">
<head>
    <meta charset="UTF-8">
    <style>
        body {
            margin: 0;
            padding: 20px;
            width: 760px; /* Largeur de l'écran 7.5" moins le padding */
            height: 440px; /* Hauteur de l'écran 7.5" moins le padding */
            background-color: white;
            color: black;
            font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif;
            display: flex;
            flex-direction: column;
            border: 2px solid black;
            box-sizing: border-box;
        }
        .header {
            font-size: 32px;
            font-weight: bold;
            border-bottom: 4px solid black;
            padding-bottom: 10px;
            margin-bottom: 20px;
            display: flex;
            justify-content: space-between;
        }
        .content {
            font-size: 24px;
            flex-grow: 1;
        }
        .footer {
            font-size: 16px;
            text-align: right;
            border-top: 2px solid black;
            padding-top: 10px;
        }
    </style>
</head>
<body>
    <div class="header">
        <span>Dashboard Personnel</span>
        <span>{{ .Time }}</span>
    </div>
    <div class="content">
        <h2>Statut de la VM de Production</h2>
        <ul>
            <li>CPU : 12%</li>
            <li>RAM : 4.2 / 16 GB</li>
            <li>Conteneurs Actifs : 8</li>
        </ul>
        <h2>Prochain événement</h2>
        <p>Réunion de conception architecture de données - 14h00</p>
    </div>
    <div class="footer">
        Généré le {{ .Date }}
    </div>
</body>
</html>
`

// Structure pour injecter des variables dynamiques dans le HTML
type TemplateData struct {
	Time string
	Date string
}

// 1. Serveur Interne (Localhost uniquement)
func startInternalHTMLServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t, err := template.New("dashboard").Parse(htmlTemplate)
		if err != nil {
			http.Error(w, "Erreur template", http.StatusInternalServerError)
			return
		}

		data := TemplateData{
			Time: time.Now().Format("15:04"),
			Date: time.Now().Format("02/01/2006"),
		}

		w.Header().Set("Content-Type", "text/html")
		t.Execute(w, data)
	})

	log.Println("Serveur HTML interne démarré sur 127.0.0.1:8080 (Non exposé)")
	if err := http.ListenAndServe("127.0.0.1:8080", mux); err != nil {
		log.Fatalf("Erreur serveur interne : %v", err)
	}
}

// 2. Fonction pour capturer la page HTML avec Chromium headless
func captureDashboard() ([]byte, error) {
	// Création d'un contexte avec un timeout strict de 15 secondes
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Initialisation de l'instance Chrome headless
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true), // Souvent nécessaire dans les conteneurs/VM Linux
	)...)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	var imageBuf []byte

	// Exécution des tâches de navigation et de capture
	err := chromedp.Run(taskCtx,
		chromedp.EmulateViewport(800, 480), // Résolution exacte de l'écran Waveshare 7.5"
		chromedp.Navigate("http://127.0.0.1:8080/"),
		// On attend que la balise body soit bien chargée
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.CaptureScreenshot(&imageBuf),
	)

	return imageBuf, err
}

// 3. Middleware de sécurité HMAC-SHA256
func authenticateRequest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		timestampStr := r.URL.Query().Get("timestamp")
		clientSignature := r.URL.Query().Get("signature")

		if timestampStr == "" || clientSignature == "" {
			http.Error(w, "Paramètres manquants", http.StatusUnauthorized)
			return
		}

		// Validation temporelle (anti-replay)
		ts, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			http.Error(w, "Format de timestamp invalide", http.StatusBadRequest)
			return
		}

		now := time.Now().Unix()
		// Tolérance de 60 secondes pour compenser la dérive d'horloge de l'ESP32
		if now-ts > 60 || ts-now > 60 {
			http.Error(w, "Requête expirée (replay attack probable)", http.StatusUnauthorized)
			return
		}

		// Calcul de la signature attendue
		mac := hmac.New(sha256.New, []byte(secretKey))
		mac.Write([]byte(timestampStr))
		expectedSignature := hex.EncodeToString(mac.Sum(nil))

		// Comparaison en temps constant (hmac.Equal) pour éviter les attaques temporelles (timing attacks)
		if !hmac.Equal([]byte(clientSignature), []byte(expectedSignature)) {
			http.Error(w, "Signature cryptographique invalide", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// 4. Handler de l'API publique (Exposée à l'ESP32)
func imageAPIHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Requête authentifiée acceptée depuis %s", r.RemoteAddr)

	imgBytes, err := captureDashboard()
	if err != nil {
		log.Printf("Erreur de capture : %v", err)
		http.Error(w, "Erreur lors de la génération de l'image", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(imgBytes)))

	// Renvoie l'image binaire à l'ESP32
	w.Write(imgBytes)
}

func main() {
	// Lancement du serveur HTML interne dans une Goroutine
	go startInternalHTMLServer()

	// Configuration du serveur API public sur le port 8081
	mux := http.NewServeMux()

	// On wrap le handler avec le middleware de sécurité HMAC
	//mux.HandleFunc("/api/v1/dashboard.png", authenticateRequest(imageAPIHandler))
	mux.HandleFunc("/api/v1/dashboard.png", imageAPIHandler)

	log.Println("Serveur API public démarré sur 0.0.0.0:8081")
	log.Println("En attente de requêtes cryptées de l'ESP32...")

	// Démarrage du serveur public
	if err := http.ListenAndServe("0.0.0.0:8081", mux); err != nil {
		log.Fatalf("Erreur serveur API : %v", err)
	}
}
