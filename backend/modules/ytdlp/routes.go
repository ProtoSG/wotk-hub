package ytdlp

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
	"workhub/httpx"

	chi "github.com/go-chi/chi/v5"
)

// Routes returns the router for the YouTube-to-MP3 module. Stateless — no DB
// needed since nothing is persisted (see handlers.go: each request downloads,
// converts, streams, then deletes its own temp dir). cookiesPath is passed to
// yt-dlp as --cookies when non-empty (see config.YtdlpCookiesPath).
func Routes(cookiesPath string) http.Handler {
	h := &handler{cookiesPath: cookiesPath}
	r := chi.NewRouter()

	r.Post("/download", h.Download)

	return r
}

// PublicRoutes mounts the same download endpoint behind a shared-secret token
// instead of JWT login, for sharing with someone who doesn't have an account
// on this app. The token travels as either `?token=` (easy to embed in a
// link) or an `Authorization: Bearer <token>` header.
func PublicRoutes(token, cookiesPath string) http.Handler {
	h := &handler{cookiesPath: cookiesPath}
	r := chi.NewRouter()
	r.Use(requireToken(token))
	r.Post("/download", h.PublicDownload)
	return r
}

// requireToken compares sha256 digests (not the raw strings) with
// subtle.ConstantTimeCompare so neither the token's length nor its content
// leaks through timing.
func requireToken(expected string) func(http.Handler) http.Handler {
	expectedHash := sha256.Sum256([]byte(expected))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := r.URL.Query().Get("token")
			if provided == "" {
				if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
					provided = strings.TrimPrefix(auth, "Bearer ")
				}
			}
			// Distinguishing "missing" from "wrong" costs nothing here (it
			// doesn't reveal anything about the real token) and saves a
			// round trip of guessing when someone's link got truncated.
			if provided == "" {
				httpx.WriteError(w, http.StatusUnauthorized, "missing token")
				return
			}
			providedHash := sha256.Sum256([]byte(provided))
			if subtle.ConstantTimeCompare(expectedHash[:], providedHash[:]) != 1 {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
