package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Structure JSON d'Open-Meteo
type WeatherResponse struct {
	Hourly struct {
		Time          []string  `json:"time"`
		Temperature2m []float64 `json:"temperature_2m"`
		Weathercode   []int     `json:"weathercode"` // Le code météo WMO
	} `json:"hourly"`
}

// Fonction pour traduire le code WMO en texte/emoji
func traduireMeteo(code int) string {
	switch {
	case code == 0:
		return "Soleil"
	case code == 1 || code == 2:
		return "Éclaircies"
	case code == 3:
		return "Nuageux"
	case code == 45 || code == 48:
		return "Brouillard"
	case code >= 51 && code <= 57:
		return "Bruine"
	case code >= 61 && code <= 67:
		return "Pluie"
	case code >= 71 && code <= 77:
		return "Neige"
	case code >= 80 && code <= 82:
		return "Averses"
	case code >= 95 && code <= 99:
		return "Orage"
	default:
		return "Inconnu"
	}
}

func getWeather() (string, string, string, string) {
	// API appelée avec temperature_2m ET weathercode
	url := "https://api.open-meteo.com/v1/forecast?latitude=48.299999&longitude=4.083330&hourly=temperature_2m,weathercode&timezone=auto&forecast_days=1"

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Println("Erreur requête météo :", err)
		return "", "", "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Erreur requête météo : statut", resp.Status)
		return "", "", "", ""
	}

	var weather WeatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
		fmt.Println("Erreur décodage météo :", err)
		return "", "", "", ""
	}

	// Sans ce contrôle, une réponse incomplète fait paniquer le handler :
	// la page renvoyée à Chromium est vide et la capture échoue.
	if len(weather.Hourly.Temperature2m) <= 16 || len(weather.Hourly.Weathercode) <= 16 {
		fmt.Println("Erreur météo : données horaires incomplètes")
		return "", "", "", ""
	}

	// Index 10 = 10:00 (Matin), Index 16 = 16:00 (Après-midi)
	tempMatin := weather.Hourly.Temperature2m[10]
	codeMatin := weather.Hourly.Weathercode[10]
	etatMatin := traduireMeteo(codeMatin)

	tempAprem := weather.Hourly.Temperature2m[16]
	codeAprem := weather.Hourly.Weathercode[16]
	etatAprem := traduireMeteo(codeAprem)

	// Formatage des températures en texte (ex: "18.5 °C")
	tMatinStr := fmt.Sprintf("%.1f °C", tempMatin)
	tApremStr := fmt.Sprintf("%.1f °C", tempAprem)

	return tMatinStr, etatMatin, tApremStr, etatAprem

}
