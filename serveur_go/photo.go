package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"math"
)

// Dimensions de l'écran Waveshare 7.5" et taille du buffer 1bpp attendu par l'ESP32.
const (
	ScreenWidth  = 800
	ScreenHeight = 480
	RawSize      = ScreenWidth / 8 * ScreenHeight
)

// Options de conversion choisies dans la page d'admin.
type PhotoOptions struct {
	Fit        string // "cover" (rogne pour remplir) ou "contain" (marges blanches)
	Rotate     int    // rotation manuelle supplémentaire : 0, 90, 180, 270
	Brightness int    // -100 à +100
	Contrast   int    // -100 à +100
	Dither     string // "floyd" (photos) ou "seuil" (texte / dessins au trait)
}

// photoToRaw transforme une photo JPEG/PNG/GIF en buffer 1bpp prêt pour l'e-paper.
// Bit à 1 = pixel noir, même convention que toMonochrome1bpp.
func photoToRaw(data []byte, opt PhotoOptions) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("format d'image non supporté (JPEG, PNG et GIF uniquement) : %w", err)
	}
	_ = format

	gray := toGray(img)
	// Les photos de téléphone stockent leur orientation en EXIF plutôt que dans les pixels.
	gray = applyOrientation(gray, exifOrientation(data))
	gray = rotateGray(gray, opt.Rotate)
	gray = fitToScreen(gray, opt.Fit)
	applyLevels(gray, opt.Brightness, opt.Contrast)

	if opt.Dither == "seuil" {
		return threshold1bpp(gray), nil
	}
	return ditherFloydSteinberg(gray), nil
}

// rawToPNG reconstruit un PNG lisible depuis le buffer 1bpp, pour l'aperçu dans l'admin.
func rawToPNG(raw []byte) ([]byte, error) {
	if len(raw) != RawSize {
		return nil, fmt.Errorf("taille de buffer inattendue : %d (attendu %d)", len(raw), RawSize)
	}
	img := image.NewGray(image.Rect(0, 0, ScreenWidth, ScreenHeight))
	rowBytes := ScreenWidth / 8
	for y := 0; y < ScreenHeight; y++ {
		for x := 0; x < ScreenWidth; x++ {
			bit := raw[y*rowBytes+x/8] & (0x80 >> uint(x%8))
			v := uint8(255)
			if bit != 0 {
				v = 0
			}
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func toGray(img image.Image) *image.Gray {
	b := img.Bounds()
	dst := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			lum := (299*(r>>8) + 587*(g>>8) + 114*(bl>>8)) / 1000
			dst.SetGray(x, y, color.Gray{Y: uint8(lum)})
		}
	}
	return dst
}

// fitToScreen ramène l'image en 800x480 : "cover" rogne les bords, "contain" ajoute des marges blanches.
func fitToScreen(src *image.Gray, mode string) *image.Gray {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw == 0 || sh == 0 {
		return image.NewGray(image.Rect(0, 0, ScreenWidth, ScreenHeight))
	}

	if mode != "contain" {
		// On rogne la source au ratio 800/480 avant de redimensionner.
		cropW, cropH := sw, sh
		if sw*ScreenHeight > sh*ScreenWidth {
			cropW = sh * ScreenWidth / ScreenHeight
		} else {
			cropH = sw * ScreenHeight / ScreenWidth
		}
		crop := src.SubImage(image.Rect(
			src.Bounds().Min.X+(sw-cropW)/2,
			src.Bounds().Min.Y+(sh-cropH)/2,
			src.Bounds().Min.X+(sw-cropW)/2+cropW,
			src.Bounds().Min.Y+(sh-cropH)/2+cropH,
		)).(*image.Gray)
		return resizeGray(crop, ScreenWidth, ScreenHeight)
	}

	scale := math.Min(float64(ScreenWidth)/float64(sw), float64(ScreenHeight)/float64(sh))
	w := int(math.Max(1, math.Round(float64(sw)*scale)))
	h := int(math.Max(1, math.Round(float64(sh)*scale)))
	scaled := resizeGray(src, w, h)

	canvas := image.NewGray(image.Rect(0, 0, ScreenWidth, ScreenHeight))
	for i := range canvas.Pix {
		canvas.Pix[i] = 255
	}
	offX, offY := (ScreenWidth-w)/2, (ScreenHeight-h)/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			canvas.SetGray(offX+x, offY+y, scaled.GrayAt(x, y))
		}
	}
	return canvas
}

// resizeGray fait une moyenne pondérée par l'aire de la fenêtre source : c'est le
// rééchantillonnage qui garde le plus de détail quand on réduit fortement une photo.
func resizeGray(src *image.Gray, w, h int) *image.Gray {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewGray(image.Rect(0, 0, w, h))
	xRatio := float64(sw) / float64(w)
	yRatio := float64(sh) / float64(h)

	for y := 0; y < h; y++ {
		sy0 := float64(y) * yRatio
		sy1 := sy0 + yRatio
		for x := 0; x < w; x++ {
			sx0 := float64(x) * xRatio
			sx1 := sx0 + xRatio

			var sum, weight float64
			for py := int(sy0); py < int(math.Ceil(sy1)); py++ {
				if py < 0 || py >= sh {
					continue
				}
				wy := math.Min(sy1, float64(py+1)) - math.Max(sy0, float64(py))
				if wy <= 0 {
					continue
				}
				for px := int(sx0); px < int(math.Ceil(sx1)); px++ {
					if px < 0 || px >= sw {
						continue
					}
					wx := math.Min(sx1, float64(px+1)) - math.Max(sx0, float64(px))
					if wx <= 0 {
						continue
					}
					sum += float64(src.GrayAt(b.Min.X+px, b.Min.Y+py).Y) * wx * wy
					weight += wx * wy
				}
			}
			if weight > 0 {
				dst.SetGray(x, y, color.Gray{Y: uint8(clampF(sum/weight, 0, 255))})
			}
		}
	}
	return dst
}

func rotateGray(src *image.Gray, deg int) *image.Gray {
	deg = ((deg % 360) + 360) % 360
	if deg == 0 {
		return src
	}
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()

	var dst *image.Gray
	switch deg {
	case 90:
		dst = image.NewGray(image.Rect(0, 0, sh, sw))
		for y := 0; y < sh; y++ {
			for x := 0; x < sw; x++ {
				dst.SetGray(sh-1-y, x, src.GrayAt(b.Min.X+x, b.Min.Y+y))
			}
		}
	case 180:
		dst = image.NewGray(image.Rect(0, 0, sw, sh))
		for y := 0; y < sh; y++ {
			for x := 0; x < sw; x++ {
				dst.SetGray(sw-1-x, sh-1-y, src.GrayAt(b.Min.X+x, b.Min.Y+y))
			}
		}
	case 270:
		dst = image.NewGray(image.Rect(0, 0, sh, sw))
		for y := 0; y < sh; y++ {
			for x := 0; x < sw; x++ {
				dst.SetGray(y, sw-1-x, src.GrayAt(b.Min.X+x, b.Min.Y+y))
			}
		}
	default:
		return src
	}
	return dst
}

func flipHorizontal(src *image.Gray) *image.Gray {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewGray(image.Rect(0, 0, sw, sh))
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			dst.SetGray(sw-1-x, y, src.GrayAt(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// applyOrientation applique le tag EXIF Orientation (1 à 8).
func applyOrientation(src *image.Gray, orientation int) *image.Gray {
	switch orientation {
	case 2:
		return flipHorizontal(src)
	case 3:
		return rotateGray(src, 180)
	case 4:
		return rotateGray(flipHorizontal(src), 180)
	case 5:
		return rotateGray(flipHorizontal(src), 90)
	case 6:
		return rotateGray(src, 90)
	case 7:
		return rotateGray(flipHorizontal(src), 270)
	case 8:
		return rotateGray(src, 270)
	default:
		return src
	}
}

func applyLevels(img *image.Gray, brightness, contrast int) {
	if brightness == 0 && contrast == 0 {
		return
	}
	c := 1 + float64(clampI(contrast, -100, 100))/100
	b := float64(clampI(brightness, -100, 100)) * 1.28
	for i, v := range img.Pix {
		img.Pix[i] = uint8(clampF((float64(v)-128)*c+128+b, 0, 255))
	}
}

// ditherFloydSteinberg diffuse l'erreur en serpentin : sur un écran 1 bit,
// c'est ce qui donne l'illusion des niveaux de gris d'une photo.
func ditherFloydSteinberg(img *image.Gray) []byte {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	buf := make([]byte, RawSize)
	rowBytes := ScreenWidth / 8

	errBuf := make([]float32, w*h)
	for i, v := range img.Pix {
		errBuf[i] = float32(v)
	}

	diffuse := func(x, y int, e float32, f float32) {
		if x < 0 || x >= w || y < 0 || y >= h {
			return
		}
		errBuf[y*w+x] += e * f
	}

	for y := 0; y < h; y++ {
		leftToRight := y%2 == 0
		for i := 0; i < w; i++ {
			x := i
			if !leftToRight {
				x = w - 1 - i
			}

			old := errBuf[y*w+x]
			var newVal float32 = 255
			if old < 128 {
				newVal = 0
				buf[y*rowBytes+x/8] |= 0x80 >> uint(x%8) // bit à 1 = noir
			}

			e := old - newVal
			if leftToRight {
				diffuse(x+1, y, e, 7.0/16)
				diffuse(x-1, y+1, e, 3.0/16)
				diffuse(x, y+1, e, 5.0/16)
				diffuse(x+1, y+1, e, 1.0/16)
			} else {
				diffuse(x-1, y, e, 7.0/16)
				diffuse(x+1, y+1, e, 3.0/16)
				diffuse(x, y+1, e, 5.0/16)
				diffuse(x-1, y+1, e, 1.0/16)
			}
		}
	}
	return buf
}

func threshold1bpp(img *image.Gray) []byte {
	buf := make([]byte, RawSize)
	rowBytes := ScreenWidth / 8
	for y := 0; y < ScreenHeight; y++ {
		for x := 0; x < ScreenWidth; x++ {
			if img.GrayAt(x, y).Y < 128 {
				buf[y*rowBytes+x/8] |= 0x80 >> uint(x%8)
			}
		}
	}
	return buf
}

// exifOrientation lit le tag 0x0112 dans le segment APP1 d'un JPEG. Renvoie 1 (aucune
// rotation) dès que l'image n'est pas un JPEG ou ne porte pas ce tag.
func exifOrientation(data []byte) int {
	const defaultOrientation = 1

	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return defaultOrientation
	}

	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return defaultOrientation
		}
		marker := data[i+1]
		if marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			i += 2
			continue
		}
		if marker == 0xDA || marker == 0xD9 { // début des données image : plus d'EXIF après
			return defaultOrientation
		}
		segLen := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if segLen < 2 || i+2+segLen > len(data) {
			return defaultOrientation
		}
		if marker == 0xE1 {
			seg := data[i+4 : i+2+segLen]
			if len(seg) > 6 && string(seg[:4]) == "Exif" {
				if o, ok := parseTIFFOrientation(seg[6:]); ok {
					return o
				}
			}
		}
		i += 2 + segLen
	}
	return defaultOrientation
}

func parseTIFFOrientation(tiff []byte) (int, bool) {
	if len(tiff) < 8 {
		return 0, false
	}
	var bo binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 0, false
	}

	ifdOffset := int(bo.Uint32(tiff[4:8]))
	if ifdOffset+2 > len(tiff) {
		return 0, false
	}
	count := int(bo.Uint16(tiff[ifdOffset : ifdOffset+2]))
	for n := 0; n < count; n++ {
		entry := ifdOffset + 2 + n*12
		if entry+12 > len(tiff) {
			return 0, false
		}
		if bo.Uint16(tiff[entry:entry+2]) == 0x0112 {
			v := int(bo.Uint16(tiff[entry+8 : entry+10]))
			if v >= 1 && v <= 8 {
				return v, true
			}
			return 0, false
		}
	}
	return 0, false
}

func clampF(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func clampI(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
