package couple

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"
	"workhub/httpx"
	"workhub/middleware"
)

// maxPhotoUploadBytes caps a single photo upload. httpx.DefaultMaxBodyBytes
// (1MB) is way too small for a photo, so this handler defines its own local
// limit instead of changing the shared default — other endpoints rely on
// staying small.
const maxPhotoUploadBytes int64 = 15 << 20 // 15MB

// allowedPhotoTypes maps accepted Content-Type values to a file extension
// for the generated object key. Anything else is rejected with 400.
var allowedPhotoTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// newObjectKey generates a server-side object key — never trust a client
// filename. crypto/rand + hex instead of a UUID lib since the project has no
// existing UUID dependency to reuse.
func newObjectKey(dateID int64, ext string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("couple-dates/%d/%s%s", dateID, hex.EncodeToString(buf), ext), nil
}

// UploadPhoto uploads a photo attached to a couple date.
//
// @Summary Upload a photo for a couple date
// @Tags couple
// @Accept multipart/form-data
// @Produce json
// @Security CookieAuth
// @Param id path int true "Date ID"
// @Param photo formData file true "Photo file (jpeg, png, webp or gif)"
// @Success 201 {object} Photo
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Failure 401 {object} httpx.APIError
// @Failure 503 {object} httpx.APIError
// @Router /couple/dates/{id}/photos [post]
func (h *handler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable, "photo storage not configured")
		return
	}
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	dateID, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	var exists bool
	if err := h.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM couple_dates WHERE id = $1)`, dateID).Scan(&exists); err != nil {
		log.Printf("couple: date existence check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if !exists {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "date not found")
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
	ext, ok := allowedPhotoTypes[contentType]
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "unsupported image type")
		return
	}

	key, err := newObjectKey(dateID, ext)
	if err != nil {
		log.Printf("couple: object key generation failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := h.storage.Upload(r.Context(), key, file, header.Size, contentType); err != nil {
		log.Printf("couple: photo upload to storage failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	var id int64
	var createdAt time.Time
	err = h.db.QueryRow(
		`INSERT INTO couple_date_photos (date_id, object_key, created_by) VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		dateID, key, userID,
	).Scan(&id, &createdAt)
	if err != nil {
		log.Printf("couple: insert photo row failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	url, err := h.storage.PresignedGetURL(r.Context(), key)
	if err != nil {
		log.Printf("couple: presign after upload failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, Photo{ID: id, URL: url, CreatedAt: createdAt.Format(time.RFC3339)})
}

// ListPhotos lists photos attached to a couple date, generating a fresh
// presigned GET URL for each — URLs expire and are never persisted, only
// object_key is durable.
//
// @Summary List photos for a couple date
// @Tags couple
// @Produce json
// @Security CookieAuth
// @Param id path int true "Date ID"
// @Success 200 {object} listPhotosResponse
// @Failure 400 {object} httpx.APIError
// @Failure 503 {object} httpx.APIError
// @Router /couple/dates/{id}/photos [get]
func (h *handler) ListPhotos(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable, "photo storage not configured")
		return
	}
	dateID, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	rows, err := h.db.Query(
		`SELECT id, object_key, created_at FROM couple_date_photos WHERE date_id = $1 ORDER BY created_at DESC, id DESC`,
		dateID,
	)
	if err != nil {
		log.Printf("couple: list photos failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	photos := []Photo{}
	for rows.Next() {
		var id int64
		var objectKey string
		var createdAt time.Time
		if err := rows.Scan(&id, &objectKey, &createdAt); err != nil {
			log.Printf("couple: scan photo failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		url, err := h.storage.PresignedGetURL(r.Context(), objectKey)
		if err != nil {
			log.Printf("couple: presign photo failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		photos = append(photos, Photo{ID: id, URL: url, CreatedAt: createdAt.Format(time.RFC3339)})
	}
	httpx.WriteJSON(w, http.StatusOK, listPhotosResponse{Photos: photos})
}

// DeletePhoto deletes a photo attached to a couple date.
//
// Policy: the MinIO object is deleted first; if that fails, the DB row is
// still removed and the orphan is logged loudly, rather than leaving a
// stuck row the user can never clear from the UI. This is a low-stakes
// personal-photo feature — favors user-facing simplicity over perfect
// storage/DB consistency.
//
// @Summary Delete a photo from a couple date
// @Tags couple
// @Produce json
// @Security CookieAuth
// @Param id path int true "Date ID"
// @Param photoId path int true "Photo ID"
// @Success 200 {object} httpx.SuccessResponse
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Failure 503 {object} httpx.APIError
// @Router /couple/dates/{id}/photos/{photoId} [delete]
func (h *handler) DeletePhoto(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable, "photo storage not configured")
		return
	}
	dateID, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	photoID, err := parsePhotoID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	var objectKey string
	err = h.db.QueryRow(`SELECT object_key FROM couple_date_photos WHERE id = $1 AND date_id = $2`, photoID, dateID).Scan(&objectKey)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "photo not found")
		return
	}
	if err != nil {
		log.Printf("couple: lookup photo failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := h.storage.Delete(r.Context(), objectKey); err != nil {
		log.Printf("couple: minio delete failed for object %q, deleting DB row anyway (orphan left in storage): %v", objectKey, err)
	}

	if _, err := h.db.Exec(`DELETE FROM couple_date_photos WHERE id = $1`, photoID); err != nil {
		log.Printf("couple: delete photo row failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}

// deletePhotoObjectsForDate best-effort deletes every MinIO object for a
// date's photos. Called from DeleteDate before the SQL DELETE cascades the
// couple_date_photos rows away. Logs and continues on failure — a failed
// storage delete shouldn't block the user from deleting the date itself.
func (h *handler) deletePhotoObjectsForDate(ctx context.Context, dateID int64) {
	rows, err := h.db.Query(`SELECT object_key FROM couple_date_photos WHERE date_id = $1`, dateID)
	if err != nil {
		log.Printf("couple: list photo objects for date delete failed: %v", err)
		return
	}
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			log.Printf("couple: scan photo object key for date delete failed: %v", err)
			continue
		}
		keys = append(keys, key)
	}
	rows.Close()

	for _, key := range keys {
		if err := h.storage.Delete(ctx, key); err != nil {
			log.Printf("couple: minio delete failed for object %q during date delete (orphan left in storage): %v", key, err)
		}
	}
}
