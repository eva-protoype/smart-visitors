package handlers

import (
	"database/sql"
	"strings"
	"testing"

	"smart-visitors/internal/cryptoutil"
)

func TestEncryptVisitorInputEncryptsPIIAndCreatesBlindIndexes(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	phone := "  +1 555 0100 "
	idNote := "Passport 1234"
	input := visitorInput{Name: " Ada Visitor ", Phone: &phone, IDNote: &idNote}

	encrypted, err := encryptVisitorInput(key, "visitor-id", input)
	if err != nil {
		t.Fatal(err)
	}
	for _, plaintext := range []string{input.Name, phone, idNote} {
		if strings.Contains(encrypted.name, strings.TrimSpace(plaintext)) ||
			(encrypted.phone != nil && strings.Contains(encrypted.phone.(string), strings.TrimSpace(plaintext))) ||
			(encrypted.idNote != nil && strings.Contains(encrypted.idNote.(string), strings.TrimSpace(plaintext))) {
			t.Fatalf("visitor PII was stored as plaintext")
		}
	}
	if !strings.HasPrefix(encrypted.name, visitorCipherPrefix) ||
		encrypted.phone == nil || !strings.HasPrefix(encrypted.phone.(string), visitorCipherPrefix) ||
		encrypted.idNote == nil || !strings.HasPrefix(encrypted.idNote.(string), visitorCipherPrefix) {
		t.Fatalf("expected all visitor PII fields to be encrypted")
	}
	if encrypted.nameHash != cryptoutil.TokenHash(key, "ada visitor") ||
		encrypted.phoneHash != cryptoutil.TokenHash(key, "+1 555 0100") {
		t.Fatalf("visitor blind indexes do not match normalized PII")
	}

	name, err := decryptVisitorName(key, "visitor-id", encrypted.name)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Ada Visitor" {
		t.Fatalf("decrypted name = %q, want %q", name, "Ada Visitor")
	}
	decryptedPhone, err := decryptOptionalVisitorValue(key, "visitor-id", "phone", nullStringValue(encrypted.phone))
	if err != nil {
		t.Fatal(err)
	}
	if decryptedPhone == nil || *decryptedPhone != "+1 555 0100" {
		t.Fatalf("decrypted phone = %v, want normalized phone", decryptedPhone)
	}
}

func TestOpenVisitorValueRejectsCiphertextFromAnotherVisitor(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	encrypted, err := sealVisitorValue(key, "visitor-a", "name", "Ada Visitor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decryptVisitorName(key, "visitor-b", encrypted); err == nil {
		t.Fatal("expected associated visitor ID mismatch to fail")
	}
}

func nullStringValue(value any) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: value.(string), Valid: true}
}
