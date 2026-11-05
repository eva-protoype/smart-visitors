package handlers

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestVisitorPhotoIsResizedAndEncryptedAtRest(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 960, 480))
	for y := 0; y < 480; y++ {
		for x := 0; x < 960; x++ {
			source.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), A: 255})
		}
	}
	var input bytes.Buffer
	if err := jpeg.Encode(&input, source, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}

	resized, err := resizeVisitorPhoto(input.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(resized))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != visitorPhotoMaxSide || decoded.Bounds().Dy() != visitorPhotoMaxSide/2 {
		t.Fatalf("resized dimensions = %v, want 480x240", decoded.Bounds())
	}

	dir := t.TempDir()
	key := []byte("0123456789abcdef0123456789abcdef")
	visitorID := "visitor-id"
	ref := visitorID + ".jpg"
	if err := storeEncryptedVisitorPhoto(dir, key, visitorID, ref, resized); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(dir, ref))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(stored, []byte{0xff, 0xd8}) {
		t.Fatal("photo file is still plaintext JPEG")
	}
	if mode := fileMode(t, filepath.Join(dir, ref)); mode.Perm() != 0600 {
		t.Fatalf("photo file permissions = %o, want 600", mode.Perm())
	}
	restored, err := readEncryptedVisitorPhoto(dir, key, visitorID, ref)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, resized) {
		t.Fatal("decrypted photo does not match resized photo")
	}
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode()
}
