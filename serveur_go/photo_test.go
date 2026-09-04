package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// Photo synthétique en portrait avec un dégradé : vérifie le redimensionnement,
// le cadrage et le tramage de bout en bout.
func testJPEG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(x * 255 / w)
			img.Set(x, y, color.RGBA{R: v, G: v, B: uint8(y * 255 / h), A: 255})
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

func TestPhotoToRaw(t *testing.T) {
	src := testJPEG(1200, 1600) // portrait, comme une photo de téléphone

	for _, opt := range []PhotoOptions{
		{Fit: "cover", Dither: "floyd"},
		{Fit: "contain", Dither: "floyd"},
		{Fit: "cover", Dither: "seuil", Rotate: 90, Brightness: 30, Contrast: -20},
	} {
		raw, err := photoToRaw(src, opt)
		if err != nil {
			t.Fatalf("photoToRaw(%+v) : %v", opt, err)
		}
		if len(raw) != RawSize {
			t.Fatalf("photoToRaw(%+v) : %d octets, attendu %d", opt, len(raw), RawSize)
		}

		var black int
		for _, b := range raw {
			for i := 0; i < 8; i++ {
				if b&(1<<uint(i)) != 0 {
					black++
				}
			}
		}
		// Un dégradé doit produire un mélange de noir et de blanc, jamais un buffer uniforme.
		if black == 0 || black == RawSize*8 {
			t.Fatalf("photoToRaw(%+v) : buffer uniforme (%d pixels noirs)", opt, black)
		}

		if _, err := rawToPNG(raw); err != nil {
			t.Fatalf("rawToPNG : %v", err)
		}
	}
}

func TestPhotoToRawRejectsGarbage(t *testing.T) {
	if _, err := photoToRaw([]byte("pas une image"), PhotoOptions{}); err == nil {
		t.Fatal("une entrée invalide doit renvoyer une erreur")
	}
}

// Une image plus petite que l'écran doit être agrandie sans planter.
func TestPhotoToRawUpscale(t *testing.T) {
	raw, err := photoToRaw(testJPEG(120, 90), PhotoOptions{Fit: "contain", Dither: "floyd"})
	if err != nil {
		t.Fatalf("photoToRaw : %v", err)
	}
	if len(raw) != RawSize {
		t.Fatalf("%d octets, attendu %d", len(raw), RawSize)
	}
}

func TestExifOrientation(t *testing.T) {
	// JPEG sans EXIF : orientation neutre.
	if got := exifOrientation(testJPEG(10, 10)); got != 1 {
		t.Fatalf("exifOrientation sans EXIF = %d, attendu 1", got)
	}
	if got := exifOrientation([]byte{0x00, 0x01}); got != 1 {
		t.Fatalf("exifOrientation sur données courtes = %d, attendu 1", got)
	}
}
