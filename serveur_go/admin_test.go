package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminRequiresPassword(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "")
	rec := httptest.NewRecorder()
	requireAdminAuth(adminPageHandler)(rec, httptest.NewRequest("GET", "/admin", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("sans ADMIN_PASSWORD : code %d, attendu 503", rec.Code)
	}

	t.Setenv("ADMIN_PASSWORD", "secret")
	rec = httptest.NewRecorder()
	requireAdminAuth(adminPageHandler)(rec, httptest.NewRequest("GET", "/admin", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sans identifiants : code %d, attendu 401", rec.Code)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin", nil)
	req.SetBasicAuth("admin", "mauvais")
	requireAdminAuth(adminPageHandler)(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("mauvais mot de passe : code %d, attendu 401", rec.Code)
	}
}

// Parcours complet : upload d'une photo, activation du mode, puis service du buffer à l'ESP32.
func TestUploadThenServeRaw(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	setMode(ModeDashboard)

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("photo", "vacances.jpg")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(testJPEG(900, 600))
	mw.WriteField("fit", "cover")
	mw.WriteField("dither", "floyd")
	mw.WriteField("activate", "1")
	mw.Close()

	req := httptest.NewRequest("POST", "/admin/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	adminUploadHandler(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upload : code %d, attendu 303 (%s)", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "err=") {
		t.Fatalf("upload en erreur : %s", loc)
	}
	if st := currentState(); st.Mode != ModePhoto || st.PhotoName != "vacances.jpg" {
		t.Fatalf("état après upload : %+v", st)
	}

	// L'ESP32 doit maintenant recevoir la photo, pas le dashboard.
	rec = httptest.NewRecorder()
	imageRawAPIHandler(rec, httptest.NewRequest("GET", "/api/v1/dashboard.raw", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard.raw : code %d", rec.Code)
	}
	if n := rec.Body.Len(); n != RawSize {
		t.Fatalf("dashboard.raw : %d octets, attendu %d", n, RawSize)
	}

	// L'aperçu doit refléter le même buffer.
	rec = httptest.NewRecorder()
	adminPreviewHandler(rec, httptest.NewRequest("GET", "/admin/preview.png", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("preview : code %d type %q", rec.Code, rec.Header().Get("Content-Type"))
	}

	// Le mode est relu depuis le disque après redémarrage.
	state = AppState{Mode: ModeDashboard}
	loadState()
	if currentState().Mode != ModePhoto {
		t.Fatal("le mode photo doit survivre à un redémarrage")
	}
}

func TestModeSwitchWithoutPhoto(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	setMode(ModeDashboard)

	req := httptest.NewRequest("POST", "/admin/mode", strings.NewReader("mode=photo"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	adminModeHandler(rec, req)

	if !strings.Contains(rec.Header().Get("Location"), "err=") {
		t.Fatalf("passer en mode photo sans photo doit échouer : %s", rec.Header().Get("Location"))
	}
	if currentState().Mode != ModeDashboard {
		t.Fatal("le mode ne doit pas changer sans photo enregistrée")
	}
}

func TestAdminPageRenders(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	setMode(ModeDashboard)

	rec := httptest.NewRecorder()
	adminPageHandler(rec, httptest.NewRequest("GET", "/admin?ok=Test", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("page admin : code %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Mode d'affichage") {
		t.Fatal("le template admin ne s'est pas rendu")
	}
}
