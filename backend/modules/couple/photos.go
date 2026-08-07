package couple

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
	"time"
	"workhub/httpx"
	"workhub/middleware"
)

// maxPhotoUploadBytes caps a single photo upload. httpx.DefaultMaxBodyBytes
// (1MB) is way too small for a photo, so this handler defines its own local
// limit instead of changing the shared default — other endpoints rely on
// staying small.
const maxPhotoUploadBytes int64 = 15 << 20 // 15MB

// allowedPhotoTypes is the set of accepted upload Content-Type values.
// Anything else is rejected with 400. Every accepted format is decoded (see
// decodeImage) and re-encoded as JPEG on upload — see image_resize.go — so,
// unlike before, this no longer needs to carry a file extension: both stored
// variants are always ".jpg" regardless of the original upload format.
var allowedPhotoTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

// newPhotoObjectKeys generates a fresh random ID — never trust a client
// filename — and returns the two object keys derived from it: the full-size
// variant and the thumbnail variant. Both live under the same dateID folder
// and share the same random ID, so a photo's two stored objects are visibly
// paired when browsing the bucket directly. crypto/rand + hex instead of a
// UUID lib since the project has no existing UUID dependency to reuse.
func newPhotoObjectKeys(dateID int64) (fullKey, thumbKey string, err error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	id := hex.EncodeToString(buf)
	return fmt.Sprintf("couple-dates/%d/%s.jpg", dateID, id),
		fmt.Sprintf("couple-dates/%d/%s-thumb.jpg", dateID, id),
		nil
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
	if !allowedPhotoTypes[contentType] {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "unsupported image type")
		return
	}

	// Read fully into memory first — EXIF orientation and pixel decoding
	// each need their own read of the same bytes, and an http.FormFile isn't
	// seekable. Bounded by maxPhotoUploadBytes (15MB) above, so this is fine
	// at this scale.
	raw, err := io.ReadAll(file)
	if err != nil {
		log.Printf("couple: read uploaded photo failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	img, err := decodeImage(bytes.NewReader(raw), contentType)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid image file")
		return
	}
	// Phone cameras record pixels in the sensor's native orientation and
	// store which way to rotate for display as separate EXIF metadata
	// instead of rotating the pixels themselves. resizeToJPEG's output never
	// carries EXIF (image/jpeg.Encode doesn't write any), so without this
	// correction the photo comes out sideways/upside-down whenever it wasn't
	// shot in that native orientation — see applyOrientation.
	img = applyOrientation(img, exifOrientation(raw))

	fullBytes, err := resizeToJPEG(img, fullMaxDim)
	if err != nil {
		log.Printf("couple: full-size resize failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	thumbBytes, err := resizeToJPEG(img, thumbMaxDim)
	if err != nil {
		log.Printf("couple: thumbnail resize failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	fullKey, thumbKey, err := newPhotoObjectKeys(dateID)
	if err != nil {
		log.Printf("couple: object key generation failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := h.storage.Upload(r.Context(), fullKey, bytes.NewReader(fullBytes), int64(len(fullBytes)), "image/jpeg"); err != nil {
		log.Printf("couple: full-size photo upload to storage failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if err := h.storage.Upload(r.Context(), thumbKey, bytes.NewReader(thumbBytes), int64(len(thumbBytes)), "image/jpeg"); err != nil {
		log.Printf("couple: thumbnail photo upload to storage failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	var id int64
	var createdAt time.Time
	err = h.db.QueryRow(
		`INSERT INTO couple_date_photos (date_id, object_key, thumbnail_object_key, created_by) VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		dateID, fullKey, thumbKey, userID,
	).Scan(&id, &createdAt)
	if err != nil {
		log.Printf("couple: insert photo row failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	url, err := h.storage.PresignedGetURL(r.Context(), fullKey)
	if err != nil {
		log.Printf("couple: presign after upload failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	thumbnailURL, err := h.storage.PresignedGetURL(r.Context(), thumbKey)
	if err != nil {
		log.Printf("couple: presign thumbnail after upload failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, Photo{ID: id, URL: url, ThumbnailURL: thumbnailURL, CreatedAt: createdAt.Format(time.RFC3339)})
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
		`SELECT id, object_key, thumbnail_object_key, created_at FROM couple_date_photos WHERE date_id = $1 ORDER BY created_at DESC, id DESC`,
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
		var thumbnailObjectKey sql.NullString
		var createdAt time.Time
		if err := rows.Scan(&id, &objectKey, &thumbnailObjectKey, &createdAt); err != nil {
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
		// Rows created before thumbnail_object_key existed have NULL here —
		// fall back to presigning the main object as the thumbnail too, so
		// old rows still render (just without the size savings) instead of
		// erroring or showing a broken image.
		thumbKey := objectKey
		if thumbnailObjectKey.Valid {
			thumbKey = thumbnailObjectKey.String
		}
		thumbnailURL, err := h.storage.PresignedGetURL(r.Context(), thumbKey)
		if err != nil {
			log.Printf("couple: presign thumbnail failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		photos = append(photos, Photo{ID: id, URL: url, ThumbnailURL: thumbnailURL, CreatedAt: createdAt.Format(time.RFC3339)})
	}
	httpx.WriteJSON(w, http.StatusOK, listPhotosResponse{Photos: photos})
}

// ListGalleryPhotos lists every photo across every couple date in one query
// — a join instead of the client calling ListPhotos once per date (would be
// an N+1 fetch for a gallery spanning many dates). Grouping by date is done
// client-side from this flat, date-sorted list.
//
// @Summary List every couple-date photo, across all dates
// @Tags couple
// @Produce json
// @Security CookieAuth
// @Success 200 {object} listGalleryPhotosResponse
// @Failure 503 {object} httpx.APIError
// @Router /couple/photos [get]
func (h *handler) ListGalleryPhotos(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable, "photo storage not configured")
		return
	}
	rows, err := h.db.Query(
		`SELECT p.id, p.object_key, p.thumbnail_object_key, p.created_at, d.id, d.occurred_on, d.place
		 FROM couple_date_photos p
		 JOIN couple_dates d ON d.id = p.date_id
		 ORDER BY d.occurred_on DESC, p.created_at DESC`,
	)
	if err != nil {
		log.Printf("couple: list gallery photos failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	photos := []GalleryPhoto{}
	for rows.Next() {
		var id, dateID int64
		var objectKey, datePlace string
		var thumbnailObjectKey sql.NullString
		var createdAt, dateOccurredOn time.Time
		if err := rows.Scan(&id, &objectKey, &thumbnailObjectKey, &createdAt, &dateID, &dateOccurredOn, &datePlace); err != nil {
			log.Printf("couple: scan gallery photo failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		url, err := h.storage.PresignedGetURL(r.Context(), objectKey)
		if err != nil {
			log.Printf("couple: presign gallery photo failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		// Same NULL-thumbnail fallback as ListPhotos — rows from before
		// thumbnails existed still render, just without the size savings.
		thumbKey := objectKey
		if thumbnailObjectKey.Valid {
			thumbKey = thumbnailObjectKey.String
		}
		thumbnailURL, err := h.storage.PresignedGetURL(r.Context(), thumbKey)
		if err != nil {
			log.Printf("couple: presign gallery thumbnail failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		photos = append(photos, GalleryPhoto{
			ID:             id,
			URL:            url,
			ThumbnailURL:   thumbnailURL,
			CreatedAt:      createdAt.Format(time.RFC3339),
			DateID:         dateID,
			DateOccurredOn: dateOccurredOn.Format(dateLayout),
			DatePlace:      datePlace,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, listGalleryPhotosResponse{Photos: photos})
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
	var thumbnailObjectKey sql.NullString
	err = h.db.QueryRow(`SELECT object_key, thumbnail_object_key FROM couple_date_photos WHERE id = $1 AND date_id = $2`, photoID, dateID).Scan(&objectKey, &thumbnailObjectKey)
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
	// thumbnail_object_key is NULL for rows created before thumbnails
	// existed — nothing to delete for those.
	if thumbnailObjectKey.Valid {
		if err := h.storage.Delete(r.Context(), thumbnailObjectKey.String); err != nil {
			log.Printf("couple: minio delete failed for thumbnail object %q, deleting DB row anyway (orphan left in storage): %v", thumbnailObjectKey.String, err)
		}
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
	rows, err := h.db.Query(`SELECT object_key, thumbnail_object_key FROM couple_date_photos WHERE date_id = $1`, dateID)
	if err != nil {
		log.Printf("couple: list photo objects for date delete failed: %v", err)
		return
	}
	var keys []string
	for rows.Next() {
		var key string
		var thumbKey sql.NullString
		if err := rows.Scan(&key, &thumbKey); err != nil {
			log.Printf("couple: scan photo object key for date delete failed: %v", err)
			continue
		}
		keys = append(keys, key)
		// thumbnail_object_key is NULL for rows created before thumbnails
		// existed — nothing to delete for those.
		if thumbKey.Valid {
			keys = append(keys, thumbKey.String)
		}
	}
	rows.Close()

	for _, key := range keys {
		if err := h.storage.Delete(ctx, key); err != nil {
			log.Printf("couple: minio delete failed for object %q during date delete (orphan left in storage): %v", key, err)
		}
	}
}
