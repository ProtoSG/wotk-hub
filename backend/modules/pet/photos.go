package pet

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	chi "github.com/go-chi/chi/v5"
	"workhub/httpx"
	"workhub/middleware"
	"workhub/shared/team"
	"workhub/storage"
)

// maxPhotoUploadBytes caps a single photo upload. httpx.DefaultMaxBodyBytes
// (1MB) is too small for a photo, so this handler defines its own local
// limit instead of changing the shared default.
const maxPhotoUploadBytes int64 = 15 << 20 // 15MB

// newPhotoObjectKey generates a fresh random ID — never trust a client
// filename. crypto/rand + hex instead of a UUID lib since the project has
// no existing UUID dependency to reuse.
func newPhotoObjectKey() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := hex.EncodeToString(buf)
	return fmt.Sprintf("pet/%s.jpg", id), nil
}

// photoResponse is returned after a successful pet photo upload.
type photoResponse struct {
	URL string `json:"url"`
}

// UploadPhoto stores a pet photo in MinIO and records it in pet_photos.
func (h *handler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable, "photo storage not configured")
		return
	}
	if _, _, ok := middleware.UserFromContext(r.Context()); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("pet: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPhotoUploadBytes)
	if err := r.ParseMultipartForm(maxPhotoUploadBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "file too large or invalid form")
		return
	}
	file, header, err := r.FormFile("photo")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "missing photo file")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	allowedTypes := map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true, "image/gif": true}
	if !allowedTypes[contentType] {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "unsupported image type")
		return
	}

	raw, err := io.ReadAll(file)
	if err != nil {
		log.Printf("pet: read uploaded photo failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	objectKey, err := newPhotoObjectKey()
	if err != nil {
		log.Printf("pet: object key generation failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := h.storage.Upload(r.Context(), objectKey, bytes.NewReader(raw), int64(len(raw)), contentType); err != nil {
		log.Printf("pet: photo upload to storage failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	var id int64
	var createdAt time.Time
	err = h.db.QueryRow(
		`INSERT INTO pet_photos (team_id, object_key, created_at) VALUES ($1, $2, now()) RETURNING id, created_at`,
		teamID, objectKey,
	).Scan(&id, &createdAt)
	if err != nil {
		log.Printf("pet: insert photo row failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	url, err := h.storage.PresignedGetURL(r.Context(), objectKey)
	if err != nil {
		log.Printf("pet: presign after upload failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	log.Printf("pet: photo %d saved for team %d", id, teamID)
	httpx.WriteJSON(w, http.StatusCreated, photoResponse{URL: url})
}

// ListPetPhotos lists all pet photos for the team, newest first.
func (h *handler) ListPetPhotos(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable, "photo storage not configured")
		return
	}
	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("pet: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	rows, err := h.db.Query(
		`SELECT id, object_key, created_at FROM pet_photos WHERE team_id = $1 ORDER BY created_at DESC`,
		teamID,
	)
	if err != nil {
		log.Printf("pet: list photos failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	type petPhoto struct {
		ID        int64  `json:"id"`
		URL       string `json:"url"`
		CreatedAt string `json:"createdAt"`
	}
	var photos []petPhoto
	for rows.Next() {
		var id int64
		var objectKey string
		var createdAt time.Time
		if err := rows.Scan(&id, &objectKey, &createdAt); err != nil {
			log.Printf("pet: scan photo row failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		url, err := h.storage.PresignedGetURL(r.Context(), objectKey)
		if err != nil {
			log.Printf("pet: presign photo failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		photos = append(photos, petPhoto{ID: id, URL: url, CreatedAt: createdAt.Format(time.RFC3339)})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]interface{}{"photos": photos})
}

// DeletePetPhoto deletes a pet photo and its MinIO object.
func (h *handler) DeletePetPhoto(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable, "photo storage not configured")
		return
	}
	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("pet: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	idStr := chi.URLParam(r, "photoId")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid photo id")
		return
	}

	var objectKey string
	err = h.db.QueryRow(
		`DELETE FROM pet_photos WHERE team_id = $1 AND id = $2 RETURNING object_key`,
		teamID, id,
	).Scan(&objectKey)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "photo not found")
		return
	}
	if err != nil {
		log.Printf("pet: delete photo lookup failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.storage.Delete(ctx, objectKey); err != nil {
		log.Printf("pet: minio delete failed for %q, removing DB row anyway: %v", objectKey, err)
	}
	httpx.WriteSuccess(w, http.StatusOK)
}
