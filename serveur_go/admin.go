package main

import (
	"crypto/subtle"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ModeDashboard = "dashboard"
	ModePhoto     = "photo"

	maxUploadBytes = 30 << 20 // 30 Mo : large de quoi encaisser une photo de téléphone
)

// État persisté sur disque pour survivre à un redémarrage du conteneur.
type AppState struct {
	Mode         string    `json:"mode"`
	PhotoName    string    `json:"photo_name"`
	PhotoUpdated time.Time `json:"photo_updated"`
}

var (
	stateMu sync.RWMutex
	state   = AppState{Mode: ModeDashboard}
)

func dataDir() string {
	if d := os.Getenv("DATA_DIR"); d != "" {
		return d
	}
	return "data"
}

func statePath() string { return filepath.Join(dataDir(), "state.json") }
func photoPath() string { return filepath.Join(dataDir(), "photo.raw") }

func loadState() {
	if err := os.MkdirAll(dataDir(), 0o755); err != nil {
		log.Printf("Impossible de créer %s : %v", dataDir(), err)
	}

	b, err := os.ReadFile(statePath())
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Lecture de l'état impossible : %v", err)
		}
		return
	}

	var s AppState
	if err := json.Unmarshal(b, &s); err != nil {
		log.Printf("État illisible, retour au mode dashboard : %v", err)
		return
	}
	if s.Mode != ModePhoto {
		s.Mode = ModeDashboard
	}

	stateMu.Lock()
	state = s
	stateMu.Unlock()
	log.Printf("État chargé : mode=%s photo=%q", s.Mode, s.PhotoName)
}

func saveStateLocked() {
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.Printf("Sérialisation de l'état impossible : %v", err)
		return
	}
	if err := os.WriteFile(statePath(), b, 0o644); err != nil {
		log.Printf("Écriture de l'état impossible : %v", err)
	}
}

func currentState() AppState {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return state
}

func setMode(mode string) {
	if mode != ModePhoto {
		mode = ModeDashboard
	}
	stateMu.Lock()
	state.Mode = mode
	saveStateLocked()
	stateMu.Unlock()
	log.Printf("Mode d'affichage : %s", mode)
}

// storePhoto écrit le buffer 1bpp de manière atomique : l'ESP32 peut lire le fichier
// au même instant, il ne doit jamais tomber sur un fichier à moitié écrit.
func storePhoto(raw []byte, name string) error {
	tmp := photoPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, photoPath()); err != nil {
		return err
	}

	stateMu.Lock()
	state.PhotoName = name
	state.PhotoUpdated = time.Now()
	saveStateLocked()
	stateMu.Unlock()
	return nil
}

func readPhoto() ([]byte, error) {
	raw, err := os.ReadFile(photoPath())
	if err != nil {
		return nil, err
	}
	if len(raw) != RawSize {
		return nil, os.ErrInvalid
	}
	return raw, nil
}

// --- Authentification de la page d'admin ---

func requireAdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		password := os.Getenv("ADMIN_PASSWORD")
		if password == "" {
			http.Error(w, "ADMIN_PASSWORD n'est pas défini côté serveur : interface désactivée.", http.StatusServiceUnavailable)
			return
		}

		_, given, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(given), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Dashboard ESP32", charset="UTF-8"`)
			http.Error(w, "Authentification requise", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// --- Handlers ---

type adminView struct {
	Mode        string
	IsPhoto     bool
	HasPhoto    bool
	PhotoName   string
	PhotoDate   string
	Message     string
	MessageKind string // "ok" ou "err"
	SleepInfo   string
}

func adminPageHandler(w http.ResponseWriter, r *http.Request) {
	st := currentState()
	_, err := readPhoto()

	view := adminView{
		Mode:      st.Mode,
		IsPhoto:   st.Mode == ModePhoto,
		HasPhoto:  err == nil,
		PhotoName: st.PhotoName,
		SleepInfo: "L'écran se rafraîchit à son prochain réveil (toutes les 15 min).",
	}
	if !st.PhotoUpdated.IsZero() {
		view.PhotoDate = st.PhotoUpdated.Format("02/01/2006 à 15:04")
	}
	if msg := r.URL.Query().Get("ok"); msg != "" {
		view.Message, view.MessageKind = msg, "ok"
	} else if msg := r.URL.Query().Get("err"); msg != "" {
		view.Message, view.MessageKind = msg, "err"
	}

	t, err := template.ParseFiles("template/admin.html")
	if err != nil {
		log.Printf("Template admin illisible : %v", err)
		http.Error(w, "Template introuvable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, view); err != nil {
		log.Printf("Rendu du template admin : %v", err)
	}
}

func adminUploadHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		redirectAdmin(w, r, "", "Fichier trop volumineux (30 Mo max) ou formulaire invalide.")
		return
	}

	file, header, err := r.FormFile("photo")
	if err != nil {
		redirectAdmin(w, r, "", "Aucun fichier reçu.")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		redirectAdmin(w, r, "", "Lecture du fichier impossible.")
		return
	}

	opt := PhotoOptions{
		Fit:        r.FormValue("fit"),
		Rotate:     formInt(r, "rotate", 0),
		Brightness: formInt(r, "brightness", 0),
		Contrast:   formInt(r, "contrast", 0),
		Dither:     r.FormValue("dither"),
	}

	raw, err := photoToRaw(data, opt)
	if err != nil {
		log.Printf("Conversion de %q échouée : %v", header.Filename, err)
		redirectAdmin(w, r, "", "Conversion impossible : "+err.Error())
		return
	}

	if err := storePhoto(raw, header.Filename); err != nil {
		log.Printf("Enregistrement de la photo : %v", err)
		redirectAdmin(w, r, "", "Enregistrement impossible sur le serveur.")
		return
	}

	msg := "Photo enregistrée."
	if r.FormValue("activate") != "" {
		setMode(ModePhoto)
		msg = "Photo enregistrée et mode photo activé."
	}
	redirectAdmin(w, r, msg, "")
}

func adminModeHandler(w http.ResponseWriter, r *http.Request) {
	mode := r.FormValue("mode")
	if mode == ModePhoto {
		if _, err := readPhoto(); err != nil {
			redirectAdmin(w, r, "", "Aucune photo enregistrée : envoyez-en une d'abord.")
			return
		}
	}
	setMode(mode)
	if mode == ModePhoto {
		redirectAdmin(w, r, "Mode photo activé.", "")
		return
	}
	redirectAdmin(w, r, "Mode dashboard activé.", "")
}

// adminPreviewHandler renvoie exactement ce que l'ESP32 va afficher, en PNG.
func adminPreviewHandler(w http.ResponseWriter, r *http.Request) {
	var raw []byte
	var err error

	if currentState().Mode == ModePhoto {
		raw, err = readPhoto()
	} else {
		var pngBytes []byte
		pngBytes, err = captureDashboard()
		if err == nil {
			raw, err = toMonochrome1bpp(pngBytes, ScreenWidth, ScreenHeight)
		}
	}
	if err != nil {
		http.Error(w, "Aperçu indisponible", http.StatusNotFound)
		return
	}

	out, err := rawToPNG(raw)
	if err != nil {
		http.Error(w, "Aperçu indisponible", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(out)
}

func redirectAdmin(w http.ResponseWriter, r *http.Request, ok, errMsg string) {
	q := ""
	if ok != "" {
		q = "?ok=" + urlEscape(ok)
	} else if errMsg != "" {
		q = "?err=" + urlEscape(errMsg)
	}
	http.Redirect(w, r, "/admin"+q, http.StatusSeeOther)
}

func urlEscape(s string) string {
	return strings.NewReplacer(" ", "%20", "\"", "%22", "'", "%27", "&", "%26", "#", "%23", "\n", " ", "\r", " ").Replace(s)
}

func formInt(r *http.Request, name string, def int) int {
	v, err := strconv.Atoi(r.FormValue(name))
	if err != nil {
		return def
	}
	return v
}
