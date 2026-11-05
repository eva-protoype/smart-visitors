package cryptoutil

import (
	"bytes"
	"testing"
)

func TestSealAndOpen(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	plaintext := []byte("private visitor details")
	sealed, err := Seal(key, plaintext, []byte("pass-1"))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := Open(key, sealed, []byte("pass-1"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plaintext, opened) {
		t.Fatal("decrypted payload differs from original")
	}
	if _, err := Open(key, sealed, []byte("pass-2")); err == nil {
		t.Fatal("ciphertext opened with the wrong associated data")
	}
	sealed[len(sealed)-1] ^= 0xff
	if _, err := Open(key, sealed, []byte("pass-1")); err == nil {
		t.Fatal("tampered ciphertext opened")
	}
}
