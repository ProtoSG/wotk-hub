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
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"workhub/httpx"
	"workhub/middleware"
	"workhub/storage"
)

// maxVideoUploadBytes caps a single video upload. Way bigger than
// maxPhotoUploadBytes — video is inherently heavier — so this handler
// defines its own local limit, same reasoning as photos.go's own constant.
const maxVideoUploadBytes int64 = 500 << 20 // 500MB

// maxVideosPerDate caps how many videos a single couple date can carry —
// enforced in UploadVideo by counting existing rows before insert.
const maxVideosPerDate = 10

// allowedVideoTypes is the set of accepted upload Content-Type values.
// Anything else is rejected with 400.
var allowedVideoTypes = map[string]bool{
	"video/mp4":       true,
	"video/quicktime": true,
	"video/x-msvideo": true,
	"video/webm":      true,
}

// newVideoObjectKeys generates a fresh random ID — never trust a client
// filename — and returns the two object keys derived from it: the 720p
// transcode and its JPEG thumbnail. Both live under the same dateID folder
// and share the same random ID, mirroring newPhotoObjectKeys in photos.go.
func newVideoObjectKeys(dateID int64) (videoKey, thumbKey string, err error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	id := hex.EncodeToString(buf)
	return fmt.Sprintf("couple-dates/%d/%s.mp4", dateID, id),
		fmt.Sprintf("couple-dates/%d/%s-thumb.jpg", dateID, id),
		nil
}

// randomTempPath returns a path under the OS temp dir with a random name
// and the given suffix (e.g. ".mp4"). It doesn't create the file — callers
// either write to it directly (ffmpeg output) or create+write it themselves
// (the raw upload copy below).
func randomTempPath(suffix string) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return filepath.Join(os.TempDir(), "couple-video-"+hex.EncodeToString(buf)+suffix), nil
}

// UploadVideo uploads a video attached to a couple date. The raw upload is
// saved to a temp file first (ffmpeg/ffprobe need a real file path, and a
// multipart file part isn't guaranteed to be disk-backed), then ffmpeg
// generates a 720p H.264 transcode and a JPEG thumbnail, and ffprobe reads
// the duration — all three run as subprocesses via exec.CommandContext so
// they're cancelled if the client disconnects. Every temp file is removed
// via defer regardless of outcome.
//
// @Summary Upload a video for a couple date
// @Tags couple
// @Accept multipart/form-data
// @Produce json
// @Security CookieAuth
// @Param id path int true "Date ID"
// @Param video formData file true "Video file (mp4, mov, avi or webm)"
// @Success 201 {object} Video
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Failure 401 {object} httpx.APIError
// @Failure 503 {object} httpx.APIError
// @Router /couple/dates/{id}/videos [post]
func (h *handler) UploadVideo(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable, "video storage not configured")
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

	var videoCount int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM couple_date_videos WHERE date_id = $1`, dateID).Scan(&videoCount); err != nil {
		log.Printf("couple: video count check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if videoCount >= maxVideosPerDate {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "date already has the maximum of 10 videos")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxVideoUploadBytes)
	// 32MB in-memory threshold for the multipart parser — anything past it
	// (i.e. essentially the whole video) is spooled by the stdlib straight to
	// its own temp file automatically. We copy from that part into our own
	// temp file below regardless, since ffmpeg/ffprobe need a stable path and
	// the stdlib's spooled file isn't guaranteed to have one we can rely on.
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "file too large or invalid form")
		return
	}
	file, header, err := r.FormFile("video")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "missing video file")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if !allowedVideoTypes[contentType] {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "unsupported video type")
		return
	}

	inPath, err := randomTempPath("-in")
	if err != nil {
		log.Printf("couple: temp path generation failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	inFile, err := os.Create(inPath)
	if err != nil {
		log.Printf("couple: create temp input file failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer os.Remove(inPath)
	if _, err := io.Copy(inFile, file); err != nil {
		inFile.Close()
		log.Printf("couple: write temp input file failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if err := inFile.Close(); err != nil {
		log.Printf("couple: close temp input file failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	thumbPath, err := randomTempPath("-thumb.jpg")
	if err != nil {
		log.Printf("couple: temp path generation failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer os.Remove(thumbPath)
	outPath, err := randomTempPath("-out.mp4")
	if err != nil {
		log.Printf("couple: temp path generation failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer os.Remove(outPath)

	ctx := r.Context()

	durationSeconds, err := probeDurationSeconds(ctx, inPath)
	if err != nil {
		log.Printf("couple: ffprobe duration failed: %v", err)
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid video file")
		return
	}

	if err := generateThumbnail(ctx, inPath, thumbPath); err != nil {
		log.Printf("couple: ffmpeg thumbnail generation failed: %v", err)
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid video file")
		return
	}

	if err := transcode720p(ctx, inPath, outPath); err != nil {
		log.Printf("couple: ffmpeg transcode failed: %v", err)
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid video file")
		return
	}

	videoKey, thumbKey, err := newVideoObjectKeys(dateID)
	if err != nil {
		log.Printf("couple: object key generation failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := uploadFileToStorage(ctx, h.storage, outPath, videoKey, "video/mp4"); err != nil {
		log.Printf("couple: video upload to storage failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if err := uploadFileToStorage(ctx, h.storage, thumbPath, thumbKey, "image/jpeg"); err != nil {
		log.Printf("couple: video thumbnail upload to storage failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	var id int64
	var createdAt time.Time
	err = h.db.QueryRow(
		`INSERT INTO couple_date_videos (date_id, object_key_720p, object_key_thumb, original_filename, duration_seconds, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at`,
		dateID, videoKey, thumbKey, header.Filename, durationSeconds, userID,
	).Scan(&id, &createdAt)
	if err != nil {
		log.Printf("couple: insert video row failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	url, err := h.storage.PresignedGetURL(ctx, videoKey)
	if err != nil {
		log.Printf("couple: presign video after upload failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	thumbnailURL, err := h.storage.PresignedGetURL(ctx, thumbKey)
	if err != nil {
		log.Printf("couple: presign video thumbnail after upload failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, Video{
		ID:               id,
		URL:              url,
		ThumbnailURL:     thumbnailURL,
		DurationSeconds:  durationSeconds,
		OriginalFilename: header.Filename,
		CreatedAt:        createdAt.Format(time.RFC3339),
	})
}

// probeDurationSeconds shells out to ffprobe to read the container duration
// (in seconds, as a float) and rounds it to the nearest whole second.
func probeDurationSeconds(ctx context.Context, inPath string) (int, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("parse ffprobe duration %q: %w", string(out), err)
	}
	return int(math.Round(seconds)), nil
}

// generateThumbnail shells out to ffmpeg to grab a single frame at the 1s
// mark and save it as a JPEG.
func generateThumbnail(ctx context.Context, inPath, thumbPath string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inPath,
		"-ss", "00:00:01",
		"-frames:v", "1",
		"-q:v", "2",
		thumbPath,
		"-y",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg thumbnail: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// transcode720p shells out to ffmpeg to produce a 720p H.264/AAC MP4 with
// faststart metadata (so playback can begin before the whole file
// downloads) regardless of the source codec/container.
func transcode720p(ctx context.Context, inPath, outPath string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inPath,
		"-vf", "scale=-2:720",
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "28",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		outPath,
		"-y",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg transcode: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// uploadFileToStorage streams path's contents straight to MinIO rather than
// buffering it in memory first — the 720p output can still be tens of MB,
// and storage.Client.Upload only needs an io.Reader plus the known size.
func uploadFileToStorage(ctx context.Context, client *storage.Client, path, key, contentType string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	return client.Upload(ctx, key, f, info.Size(), contentType)
}

// ListVideos lists videos attached to a couple date, generating fresh
// presigned GET URLs for both the video and its thumbnail — mirrors
// ListPhotos.
//
// @Summary List videos for a couple date
// @Tags couple
// @Produce json
// @Security CookieAuth
// @Param id path int true "Date ID"
// @Success 200 {object} listVideosResponse
// @Failure 400 {object} httpx.APIError
// @Failure 503 {object} httpx.APIError
// @Router /couple/dates/{id}/videos [get]
func (h *handler) ListVideos(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable, "video storage not configured")
		return
	}
	dateID, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	rows, err := h.db.Query(
		`SELECT id, object_key_720p, object_key_thumb, original_filename, duration_seconds, created_at
		 FROM couple_date_videos WHERE date_id = $1 ORDER BY created_at DESC, id DESC`,
		dateID,
	)
	if err != nil {
		log.Printf("couple: list videos failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	videos := []Video{}
	for rows.Next() {
		var id int64
		var videoKey, thumbKey, filename string
		var duration int
		var createdAt time.Time
		if err := rows.Scan(&id, &videoKey, &thumbKey, &filename, &duration, &createdAt); err != nil {
			log.Printf("couple: scan video failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		url, err := h.storage.PresignedGetURL(r.Context(), videoKey)
		if err != nil {
			log.Printf("couple: presign video failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		thumbnailURL, err := h.storage.PresignedGetURL(r.Context(), thumbKey)
		if err != nil {
			log.Printf("couple: presign video thumbnail failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		videos = append(videos, Video{
			ID:               id,
			URL:              url,
			ThumbnailURL:     thumbnailURL,
			DurationSeconds:  duration,
			OriginalFilename: filename,
			CreatedAt:        createdAt.Format(time.RFC3339),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, listVideosResponse{Videos: videos})
}

// DeleteVideo deletes a video attached to a couple date. Admin-only (see
// routes.go). Same MinIO-first, DB-second, log-and-continue-on-storage-
// error policy as DeletePhoto.
//
// @Summary Delete a video from a couple date
// @Tags couple
// @Produce json
// @Security CookieAuth
// @Param id path int true "Date ID"
// @Param videoId path int true "Video ID"
// @Success 200 {object} httpx.SuccessResponse
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Failure 503 {object} httpx.APIError
// @Router /couple/dates/{id}/videos/{videoId} [delete]
func (h *handler) DeleteVideo(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable, "video storage not configured")
		return
	}
	dateID, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	videoID, err := parseVideoID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	var videoKey, thumbKey string
	err = h.db.QueryRow(`SELECT object_key_720p, object_key_thumb FROM couple_date_videos WHERE id = $1 AND date_id = $2`, videoID, dateID).Scan(&videoKey, &thumbKey)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "video not found")
		return
	}
	if err != nil {
		log.Printf("couple: lookup video failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := h.storage.Delete(r.Context(), videoKey); err != nil {
		log.Printf("couple: minio delete failed for object %q, deleting DB row anyway (orphan left in storage): %v", videoKey, err)
	}
	if err := h.storage.Delete(r.Context(), thumbKey); err != nil {
		log.Printf("couple: minio delete failed for object %q, deleting DB row anyway (orphan left in storage): %v", thumbKey, err)
	}

	if _, err := h.db.Exec(`DELETE FROM couple_date_videos WHERE id = $1`, videoID); err != nil {
		log.Printf("couple: delete video row failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}

// deleteVideoObjectsForDate best-effort deletes every MinIO object for a
// date's videos. Called from DeleteDate before the SQL DELETE cascades the
// couple_date_videos rows away — mirrors deletePhotoObjectsForDate in
// photos.go.
func (h *handler) deleteVideoObjectsForDate(ctx context.Context, dateID int64) {
	rows, err := h.db.Query(`SELECT object_key_720p, object_key_thumb FROM couple_date_videos WHERE date_id = $1`, dateID)
	if err != nil {
		log.Printf("couple: list video objects for date delete failed: %v", err)
		return
	}
	var keys []string
	for rows.Next() {
		var videoKey, thumbKey string
		if err := rows.Scan(&videoKey, &thumbKey); err != nil {
			log.Printf("couple: scan video object key for date delete failed: %v", err)
			continue
		}
		keys = append(keys, videoKey, thumbKey)
	}
	rows.Close()

	for _, key := range keys {
		if err := h.storage.Delete(ctx, key); err != nil {
			log.Printf("couple: minio delete failed for object %q during date delete (orphan left in storage): %v", key, err)
		}
	}
}
