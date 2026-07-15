package ytdlp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"workhub/httpx"
	"workhub/middleware"
)

// downloadTimeout bounds how long a single yt-dlp invocation (download +
// audio extraction) may run before being killed.
const downloadTimeout = 10 * time.Minute

// downloadSlots caps concurrent yt-dlp processes across both the
// authenticated and public routes, bounding CPU/bandwidth regardless of how
// many requests arrive at once (the public route has no per-user limit).
var downloadSlots = make(chan struct{}, 2)

// allowedHosts is the strict allowlist of YouTube hostnames accepted in the
// download request. Anything else is rejected before yt-dlp ever runs — this
// is a command-injection-adjacent surface since we shell out to yt-dlp.
var allowedHosts = map[string]bool{
	"youtube.com":       true,
	"www.youtube.com":   true,
	"m.youtube.com":     true,
	"music.youtube.com": true,
	"youtu.be":          true,
}

type handler struct {
	cookiesPath string
	proxyURL    string
}

type downloadRequest struct {
	URL string `json:"url"`
}

// validateYouTubeURL rejects anything that isn't an http(s) URL on the
// YouTube host allowlist, case-insensitively.
func validateYouTubeURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid url")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("url must be http or https")
	}
	if !allowedHosts[strings.ToLower(u.Hostname())] {
		return nil, fmt.Errorf("url must be a youtube.com or youtu.be link")
	}
	return u, nil
}

// asciiFallbackFilename keeps only a conservative ASCII printable set, for the
// legacy `filename=` Content-Disposition param (old clients that don't
// understand `filename*`). Non-Latin titles (Japanese, Korean, emoji, etc.)
// will collapse to "audio" here — that's expected, the real name travels via
// filenameUTF8 below.
func asciiFallbackFilename(name string) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == ' ', r == '.':
			b.WriteRune(r)
		}
	}
	clean := strings.TrimSpace(b.String())
	if clean == "" {
		clean = "audio"
	}
	if strings.ToLower(ext) != ".mp3" {
		ext = ".mp3"
	}
	return clean + ext
}

// filenameUTF8 strips only what's actually dangerous in a header value —
// control characters, CR/LF (header injection), quotes, backslashes, and
// path separators — while preserving accents, CJK, and emoji so the real
// track title survives.
func filenameUTF8(name string) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	var b strings.Builder
	for _, r := range base {
		switch r {
		case '"', '\\', '/', '\r', '\n':
			continue
		}
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	clean := strings.TrimSpace(b.String())
	if clean == "" {
		clean = "audio"
	}
	if strings.ToLower(ext) != ".mp3" {
		ext = ".mp3"
	}
	return clean + ext
}

// rfc5987Encode percent-encodes s for use as the value of `filename*=UTF-8”...`
// (RFC 5987/6266). url.QueryEscape already percent-encodes every byte outside
// the unreserved set, except it turns spaces into "+" — swap those back to
// the "%20" that RFC 5987 expects.
func rfc5987Encode(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// Download is the authenticated entry point (JWT + admin/guest role, see
// main.go) — the UserFromContext check is defense-in-depth on top of that.
func (h *handler) Download(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := middleware.UserFromContext(r.Context()); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	h.doDownload(w, r)
}

// PublicDownload is the token-gated entry point (see requireToken in
// routes.go) — no user session involved, so it skips straight to doDownload.
func (h *handler) PublicDownload(w http.ResponseWriter, r *http.Request) {
	h.doDownload(w, r)
}

// doDownload validates the YouTube URL, shells out to yt-dlp to download and
// convert to MP3 in an isolated temp dir, streams the resulting file back,
// then removes the temp dir. Nothing is persisted.
func (h *handler) doDownload(w http.ResponseWriter, r *http.Request) {
	select {
	case downloadSlots <- struct{}{}:
		defer func() { <-downloadSlots }()
	default:
		httpx.WriteError(w, http.StatusTooManyRequests, "server busy, try again in a moment")
		return
	}

	var req downloadRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if _, err := validateYouTubeURL(req.URL); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// The server's global WriteTimeout (15s, see main.go) is far shorter than
	// a yt-dlp download+convert+stream can take. Extend the deadline for this
	// connection only, instead of raising the timeout for every other route.
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Now().Add(downloadTimeout + time.Minute)); err != nil {
		log.Printf("ytdlp: set write deadline failed: %v", err)
	}

	ytdlpPath, err := exec.LookPath("yt-dlp")
	if err != nil {
		log.Printf("ytdlp: yt-dlp binary not found: %v", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "download service unavailable")
		return
	}

	tempDir, err := os.MkdirTemp("", "ytdlp-*")
	if err != nil {
		log.Printf("ytdlp: mkdir temp failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer os.RemoveAll(tempDir)

	ctx, cancel := context.WithTimeout(r.Context(), downloadTimeout)
	defer cancel()

	outputTemplate := filepath.Join(tempDir, "%(title)s.%(ext)s")
	args := []string{
		"-x", "--audio-format", "mp3", "--audio-quality", "0",
		"--no-playlist",
		"--js-runtimes", "deno",
	}
	if h.cookiesPath != "" {
		// Datacenter IPs get "Sign in to confirm you're not a bot" from
		// YouTube regardless of yt-dlp version — only an authenticated
		// session's cookies reliably get past it.
		args = append(args, "--cookies", h.cookiesPath)
	}
	if h.proxyURL != "" {
		// Cookies alone don't help when it's the IP's reputation getting
		// flagged (common on datacenter VPS ranges) — route through a
		// residential proxy instead.
		args = append(args, "--proxy", h.proxyURL)
	}
	args = append(args, "-o", outputTemplate, "--", req.URL)
	cmd := exec.CommandContext(ctx, ytdlpPath, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		log.Printf("ytdlp: download timed out after %s: %s", downloadTimeout, stderr.String())
		httpx.WriteError(w, http.StatusBadGateway, "download failed")
		return
	}
	if runErr != nil {
		log.Printf("ytdlp: yt-dlp failed: %v: %s", runErr, stderr.String())
		httpx.WriteError(w, http.StatusBadGateway, "download failed")
		return
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil || len(entries) == 0 {
		log.Printf("ytdlp: no output file found (err=%v)", err)
		httpx.WriteError(w, http.StatusBadGateway, "download failed")
		return
	}

	outputName := entries[0].Name()
	f, err := os.Open(filepath.Join(tempDir, outputName))
	if err != nil {
		log.Printf("ytdlp: open output file failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer f.Close()

	asciiName := asciiFallbackFilename(outputName)
	utf8Name := filenameUTF8(outputName)
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Content-Disposition", fmt.Sprintf(
		`attachment; filename="%s"; filename*=UTF-8''%s`, asciiName, rfc5987Encode(utf8Name),
	))
	if info, err := f.Stat(); err == nil {
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	}
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, f); err != nil {
		log.Printf("ytdlp: streaming output failed: %v", err)
	}
}
