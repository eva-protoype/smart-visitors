package handlers

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"smart-visitors/internal/cryptoutil"
)

const visitorCipherPrefix = "enc1:"

type encryptedVisitor struct {
	name      string
	phone     any
	idNote    any
	nameHash  string
	phoneHash any
}

func encryptVisitorInput(masterKey []byte, visitorID string, input visitorInput) (encryptedVisitor, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return encryptedVisitor{}, errors.New("visitor name is required")
	}
	name, err := sealVisitorValue(masterKey, visitorID, "name", input.Name)
	if err != nil {
		return encryptedVisitor{}, err
	}
	encrypted := encryptedVisitor{
		name:     name,
		nameHash: cryptoutil.TokenHash(masterKey, strings.ToLower(input.Name)),
	}
	if input.Phone != nil {
		value := strings.TrimSpace(*input.Phone)
		if value != "" {
			encrypted.phone, err = sealVisitorValue(masterKey, visitorID, "phone", value)
			if err != nil {
				return encryptedVisitor{}, err
			}
			phoneHash := cryptoutil.TokenHash(masterKey, value)
			encrypted.phoneHash = phoneHash
		}
	}
	if input.IDNote != nil {
		value := strings.TrimSpace(*input.IDNote)
		if value != "" {
			encrypted.idNote, err = sealVisitorValue(masterKey, visitorID, "id_note", value)
			if err != nil {
				return encryptedVisitor{}, err
			}
		}
	}
	return encrypted, nil
}

func sealVisitorValue(masterKey []byte, visitorID, field, value string) (string, error) {
	sealed, err := cryptoutil.Seal(masterKey, []byte(value), []byte("visitor:"+visitorID+":"+field))
	if err != nil {
		return "", fmt.Errorf("encrypt visitor %s: %w", field, err)
	}
	return visitorCipherPrefix + base64.RawStdEncoding.EncodeToString(sealed), nil
}

func openVisitorValue(masterKey []byte, visitorID, field, stored string) (string, error) {
	if !strings.HasPrefix(stored, visitorCipherPrefix) {
		return stored, nil
	}
	sealed, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(stored, visitorCipherPrefix))
	if err != nil {
		return "", fmt.Errorf("decode encrypted visitor %s: %w", field, err)
	}
	plaintext, err := cryptoutil.Open(masterKey, sealed, []byte("visitor:"+visitorID+":"+field))
	if err != nil {
		return "", fmt.Errorf("decrypt visitor %s: %w", field, err)
	}
	return string(plaintext), nil
}

func decryptVisitorName(masterKey []byte, visitorID, stored string) (string, error) {
	return openVisitorValue(masterKey, visitorID, "name", stored)
}

func decryptOptionalVisitorValue(masterKey []byte, visitorID, field string, stored sql.NullString) (*string, error) {
	if !stored.Valid {
		return nil, nil
	}
	value, err := openVisitorValue(masterKey, visitorID, field, stored.String)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func MigrateVisitorPII(ctx context.Context, database *sql.DB, masterKey []byte) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin visitor PII migration: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id::text, name, phone, id_note, name_hash, phone_hash
		FROM visitors
		FOR UPDATE`)
	if err != nil {
		return fmt.Errorf("read visitor PII for encryption: %w", err)
	}
	type visitorRow struct {
		id        string
		name      string
		phone     sql.NullString
		idNote    sql.NullString
		nameHash  sql.NullString
		phoneHash sql.NullString
	}
	var visitors []visitorRow
	for rows.Next() {
		var visitor visitorRow
		if err := rows.Scan(&visitor.id, &visitor.name, &visitor.phone, &visitor.idNote,
			&visitor.nameHash, &visitor.phoneHash); err != nil {
			_ = rows.Close()
			return fmt.Errorf("read visitor PII row: %w", err)
		}
		visitors = append(visitors, visitor)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read visitor PII rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close visitor PII rows: %w", err)
	}

	for _, visitor := range visitors {
		if visitor.name == "[shredded]" && !visitor.phone.Valid && !visitor.idNote.Valid {
			continue
		}
		if strings.HasPrefix(visitor.name, visitorCipherPrefix) &&
			(!visitor.phone.Valid || strings.HasPrefix(visitor.phone.String, visitorCipherPrefix)) &&
			(!visitor.idNote.Valid || strings.HasPrefix(visitor.idNote.String, visitorCipherPrefix)) &&
			visitor.nameHash.Valid && (!visitor.phone.Valid || visitor.phoneHash.Valid) {
			continue
		}
		name, err := openVisitorValue(masterKey, visitor.id, "name", visitor.name)
		if err != nil {
			return err
		}
		phone, err := decryptOptionalVisitorValue(masterKey, visitor.id, "phone", visitor.phone)
		if err != nil {
			return err
		}
		idNote, err := decryptOptionalVisitorValue(masterKey, visitor.id, "id_note", visitor.idNote)
		if err != nil {
			return err
		}
		input := visitorInput{Name: name, Phone: phone, IDNote: idNote}
		encrypted, err := encryptVisitorInput(masterKey, visitor.id, input)
		if err != nil {
			return fmt.Errorf("encrypt visitor %s: %w", visitor.id, err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE visitors SET name = $1, phone = $2, id_note = $3,
				name_hash = $4, phone_hash = $5
			WHERE id = $6`,
			encrypted.name, encrypted.phone, encrypted.idNote,
			encrypted.nameHash, encrypted.phoneHash, visitor.id); err != nil {
			return fmt.Errorf("save encrypted visitor %s: %w", visitor.id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit visitor PII migration: %w", err)
	}
	return nil
}
