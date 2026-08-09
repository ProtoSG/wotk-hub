package pet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"workhub/httpx"
	"workhub/middleware"
	"workhub/shared/team"
)

// elevenLabsTTSURLTemplate is ElevenLabs' text-to-speech endpoint — the
// voice ID is part of the path, so unlike opencodeChatURL this needs an
// interpolation slot rather than being a single fixed string.
const elevenLabsTTSURLTemplate = "https://api.elevenlabs.io/v1/text-to-speech/%s"

// elevenLabsModelID selects which ElevenLabs model renders the speech.
// eleven_multilingual_v2 handles the pet's Spanish rioplatense replies (see
// chat.go's petChatSystemPromptTemplate) — not configurable, same "one true
// value, no env var would buy anything" reasoning as opencodeChatURL.
const elevenLabsModelID = "eleven_multilingual_v2"

// elevenLabsHTTPClient is a package-level client with a fixed timeout, same
// idiom opencodeHTTPClient (chat.go) already follows for this package's
// other outbound call.
var elevenLabsHTTPClient = &http.Client{Timeout: 15 * time.Second}

// speakRequest is colocated with the Speak handler, same convention
// chatRequest/chatResponse follow next to Chat (see chat.go's doc comment
// on that choice). No speakResponse struct exists — success streams raw
// audio bytes, not JSON, so there's nothing to model.
type speakRequest struct {
	Text string `json:"text"`
}

// elevenLabsTTSRequest is the JSON body ElevenLabs' text-to-speech endpoint
// expects. Only the fields this handler actually sets are modeled, same
// "no reason to round-trip a larger struct" stance opencodeChatCompletionsRequest
// takes.
type elevenLabsTTSRequest struct {
	Text    string `json:"text"`
	ModelID string `json:"model_id"`
}

// Speak sends the pet's reply text to ElevenLabs and streams the resulting
// MP3 audio straight through to the client. Draws from the exact same
// pet_chat_log rate-limit bucket as Chat — see countAndLogChatMessage
// (chat.go) for why TTS doesn't get its own allowance.
func (h *handler) Speak(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := middleware.UserFromContext(r.Context()); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	var req speakRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	text := strings.TrimSpace(req.Text)
	// Reuses chatMessageMaxRunes (chat.go) rather than a second magic number
	// — this is speaking the same pet replies Chat already caps the length
	// of, there's no separate limit worth inventing for TTS.
	if text == "" || len([]rune(text)) > chatMessageMaxRunes {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "text must be 1-500 characters")
		return
	}

	// Fail fast before touching the DB at all if voice isn't configured —
	// same "nice-to-have, disabled when unset" stance Chat takes for
	// h.opencodeAPIKey above.
	if h.elevenLabsAPIKey == "" || h.elevenLabsVoiceID == "" {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable, "pet voice is not configured")
		return
	}

	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("pet: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := h.countAndLogChatMessage(teamID); err != nil {
		if errors.Is(err, errChatRateLimited) {
			httpx.WriteError(w, http.StatusTooManyRequests, httpx.CodeServiceUnavailable, "too many messages, try again later")
			return
		}
		log.Printf("pet: rate limit check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	resp, err := h.callElevenLabsTTS(text)
	if err != nil {
		log.Printf("pet: text-to-speech failed: %v", err)
		httpx.WriteError(w, http.StatusBadGateway, httpx.CodeServiceUnavailable, "pet voice is unavailable right now")
		return
	}
	defer resp.Body.Close()

	// httpx's helpers (WriteJSON/WriteError) only ever write JSON — this
	// response is raw MP3 audio, so it bypasses httpx entirely and writes
	// the header/status/body directly instead, mirroring what those helpers
	// do internally just without the JSON encode step.
	w.Header().Set("Content-Type", "audio/mpeg")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, resp.Body); err != nil {
		// Response is already committed at this point (status + header
		// written above) — nothing left to do but log, the client just gets
		// a truncated stream.
		log.Printf("pet: stream tts audio failed: %v", err)
	}
}

// callElevenLabsTTS posts text to ElevenLabs' text-to-speech endpoint and
// returns the raw *http.Response for the caller to stream through. Unlike
// callOpencodeChat, the success body IS the payload (raw MP3 bytes, not
// JSON) — there's nothing to decode here, so this returns the response
// itself rather than a parsed value. The caller owns closing resp.Body once
// it's done copying it to the client.
func (h *handler) callElevenLabsTTS(text string) (*http.Response, error) {
	body, err := json.Marshal(elevenLabsTTSRequest{Text: text, ModelID: elevenLabsModelID})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf(elevenLabsTTSURLTemplate, h.elevenLabsVoiceID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", h.elevenLabsAPIKey)

	resp, err := elevenLabsHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call elevenlabs: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		// Read (and discard past a small cap) so the connection can be
		// reused, but never forward this body to the client — same stance
		// callOpencodeChat takes for OpenCode's error body.
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("elevenlabs returned status %d: %s", resp.StatusCode, string(errBody))
	}

	return resp, nil
}
