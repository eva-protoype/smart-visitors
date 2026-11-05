package handlers

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"smart-visitors/internal/auth"
	"smart-visitors/internal/config"
	"smart-visitors/internal/cryptoutil"
	"smart-visitors/internal/db"

	"github.com/go-chi/chi/v5"
)

const maxVisitorPhotoBytes = 2 << 20
const visitorPhotoMaxSide = 480

func VisitorPhotoUpload(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !publicLimits.allow(clientIP(r), cfg.RateLimitPublic, time.Hour) {
			w.Header().Set("Retry-After", "3600")
			writeErr(w, http.StatusTooManyRequests, "Too many public requests. Try again later.")
			return
		}
		token := chi.URLParam(r, "token")
		if !validOpaqueToken(token) {
			writeErr(w, http.StatusGone, "This invitation is invalid or has already been used.")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxVisitorPhotoBytes+(64<<10))
		if err := r.ParseMultipartForm(maxVisitorPhotoBytes); err != nil {
			writeErr(w, http.StatusRequestEntityTooLarge, "Photo must be a JPEG no larger than 2 MB.")
			return
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		file, _, err := r.FormFile("photo")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "A photo file is required.")
			return
		}
		defer file.Close()
		photo, err := io.ReadAll(io.LimitReader(file, maxVisitorPhotoBytes+1))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "Could not read photo.")
			return
		}
		if len(photo) == 0 || len(photo) > maxVisitorPhotoBytes {
			writeErr(w, http.StatusRequestEntityTooLarge, "Photo must be a JPEG no larger than 2 MB.")
			return
		}
		resized, err := resizeVisitorPhoto(photo)
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, "Photo must be a valid JPEG image.")
			return
		}

		ctx := r.Context()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not save visitor photo.")
			return
		}
		defer tx.Rollback()
		var visitorID string
		var previousPhoto sql.NullString
		err = tx.QueryRowContext(ctx, `
			SELECT p.visitor_id::text, v.photo_ref
			FROM passes p JOIN visitors v ON v.id = p.visitor_id
			WHERE p.invite_token_hash = $1 AND p.status = 'draft'
			FOR UPDATE OF p, v`, cryptoutil.TokenHash(cfg.MasterKey, token)).
			Scan(&visitorID, &previousPhoto)
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusGone, "This invitation is invalid or has already been used.")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not save visitor photo.")
			return
		}
		photoRef := visitorID + ".jpg"
		var previousData []byte
		if previousPhoto.Valid {
			if !validPhotoRef(visitorID, previousPhoto.String) {
				writeErr(w, http.StatusInternalServerError, "Could not replace visitor photo.")
				return
			}
			previousData, err = os.ReadFile(filepath.Join(cfg.PhotosDir, previousPhoto.String))
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				writeErr(w, http.StatusInternalServerError, "Could not replace visitor photo.")
				return
			}
		}
		if err := storeEncryptedVisitorPhoto(cfg.PhotosDir, cfg.MasterKey, visitorID, photoRef, resized); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not save visitor photo.")
			return
		}
		committed := false
		defer func() {
			if !committed {
				if len(previousData) > 0 {
					if err := os.WriteFile(filepath.Join(cfg.PhotosDir, photoRef), previousData, 0600); err != nil {
						slog.Error("failed to restore visitor photo after database error", "visitor_id", visitorID, "error", err)
					}
				} else {
					if err := os.Remove(filepath.Join(cfg.PhotosDir, photoRef)); err != nil && !errors.Is(err, os.ErrNotExist) {
						slog.Error("failed to remove uncommitted visitor photo", "visitor_id", visitorID, "error", err)
					}
				}
			}
		}()
		if _, err := tx.ExecContext(ctx, `UPDATE visitors SET photo_ref = $1 WHERE id = $2`, photoRef, visitorID); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not save visitor photo.")
			return
		}
		if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
			Action: "visitor.photo.set", Target: visitorID,
			Meta: map[string]any{"source": "invite"},
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record visitor photo update.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not save visitor photo.")
			return
		}
		committed = true
		writeJSON(w, http.StatusOK, map[string]string{"photo_ref": photoRef})
	}
}

func VisitorPhotoGet(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		visitorID := chi.URLParam(r, "id")
		var photoRef sql.NullString
		var allowed bool
		err := database.QueryRowContext(r.Context(), `
			SELECT v.photo_ref,
				($2 = 'admin' OR
				 EXISTS (SELECT 1 FROM passes p WHERE p.visitor_id = v.id AND
					(($2 = 'host' AND p.host_id::text = $3) OR
					 ($2 = 'guard' AND p.status = 'active' AND now() BETWEEN p.valid_from AND p.valid_until))))
			FROM visitors v WHERE v.id = $1`, visitorID, actor.Role, actor.ID).
			Scan(&photoRef, &allowed)
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "Visitor photo not found.")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load visitor photo.")
			return
		}
		if !allowed {
			writeErr(w, http.StatusForbidden, "You cannot view this visitor photo.")
			return
		}
		if !photoRef.Valid {
			writeErr(w, http.StatusNotFound, "Visitor photo not found.")
			return
		}
		if err := appendStandaloneAudit(r.Context(), database, db.AuditEvent{
			ActorID: &actor.ID, Action: "visitor.photo.view", Target: visitorID,
			Meta: map[string]any{"role": actor.Role},
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record visitor photo access.")
			return
		}
		photo, err := readEncryptedVisitorPhoto(cfg.PhotosDir, cfg.MasterKey, visitorID, photoRef.String)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load visitor photo.")
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "private, no-store")
		_, _ = w.Write(photo)
	}
}

func resizeVisitorPhoto(data []byte) ([]byte, error) {
	config, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode visitor JPEG header: %w", err)
	}
	if config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 ||
		int64(config.Width)*int64(config.Height) > 16_000_000 {
		return nil, errors.New("visitor photo dimensions exceed limits")
	}
	source, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode visitor JPEG: %w", err)
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	targetWidth, targetHeight := width, height
	if width > visitorPhotoMaxSide || height > visitorPhotoMaxSide {
		scale := float64(visitorPhotoMaxSide) / float64(max(width, height))
		targetWidth = max(1, int(float64(width)*scale))
		targetHeight = max(1, int(float64(height)*scale))
	}
	target := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			sourceX := bounds.Min.X + x*width/targetWidth
			sourceY := bounds.Min.Y + y*height/targetHeight
			target.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	var output bytes.Buffer
	if err := jpeg.Encode(&output, target, &jpeg.Options{Quality: 80}); err != nil {
		return nil, fmt.Errorf("encode visitor JPEG: %w", err)
	}
	return output.Bytes(), nil
}

func storeEncryptedVisitorPhoto(directory string, key []byte, visitorID, ref string, photo []byte) error {
	if !validPhotoRef(visitorID, ref) {
		return errors.New("invalid visitor photo reference")
	}
	sealed, err := cryptoutil.Seal(key, photo, []byte("visitor-photo:"+visitorID))
	if err != nil {
		return fmt.Errorf("encrypt visitor photo: %w", err)
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("create visitor photo directory: %w", err)
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return fmt.Errorf("secure visitor photo directory: %w", err)
	}
	file, err := os.CreateTemp(directory, ".visitor-photo-*")
	if err != nil {
		return fmt.Errorf("create visitor photo temporary file: %w", err)
	}
	tempName := file.Name()
	defer os.Remove(tempName)
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return fmt.Errorf("secure visitor photo temporary file: %w", err)
	}
	if _, err := file.Write(sealed); err != nil {
		_ = file.Close()
		return fmt.Errorf("write encrypted visitor photo: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync encrypted visitor photo: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close encrypted visitor photo: %w", err)
	}
	if err := os.Rename(tempName, filepath.Join(directory, ref)); err != nil {
		return fmt.Errorf("install encrypted visitor photo: %w", err)
	}
	return nil
}

func readEncryptedVisitorPhoto(directory string, key []byte, visitorID, ref string) ([]byte, error) {
	if !validPhotoRef(visitorID, ref) {
		return nil, errors.New("invalid visitor photo reference")
	}
	sealed, err := os.ReadFile(filepath.Join(directory, ref))
	if err != nil {
		return nil, fmt.Errorf("read encrypted visitor photo: %w", err)
	}
	photo, err := cryptoutil.Open(key, sealed, []byte("visitor-photo:"+visitorID))
	if err != nil {
		return nil, fmt.Errorf("decrypt visitor photo: %w", err)
	}
	return photo, nil
}

func validPhotoRef(visitorID, ref string) bool {
	return ref == visitorID+".jpg" && filepath.Base(ref) == ref && !strings.Contains(ref, string(filepath.Separator))
}
