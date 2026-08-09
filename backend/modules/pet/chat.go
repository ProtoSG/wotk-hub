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

// opencodeChatURL is OpenCode Zen's OpenAI-chat-completions-compatible
// endpoint. Not configurable — unlike the API key/model, this app only ever
// talks to this one gateway, so there's nothing an env var would buy us.
const opencodeChatURL = "https://opencode.ai/zen/v1/chat/completions"

// chatMessageMaxRunes caps the user's message length. Generous enough for a
// real message, small enough that nobody's stuffing a novel (or an abuse
// payload) into every hour's 20-message allowance.
const chatMessageMaxRunes = 500

// chatRateLimitPerHour is how many chat messages a team may send per rolling
// hour, enforced against pet_chat_log (see Chat and Speak — both draw from
// this same bucket via countAndLogChatMessage below). 20/hour is generous
// for a couple chatting with their pet, stingy enough to bound OpenCode
// Zen/ElevenLabs spend if something starts hammering the endpoints.
const chatRateLimitPerHour = 20

// errChatRateLimited signals countAndLogChatMessage found the team already
// at chatRateLimitPerHour for the rolling hour — a sentinel rather than a
// bare error so both Chat and Speak can tell "over quota" apart from a real
// DB failure and write the right status code for each.
var errChatRateLimited = errors.New("pet: chat rate limit exceeded")

// countAndLogChatMessage enforces chatRateLimitPerHour against
// pet_chat_log, shared by Chat and Speak (speak.go) — TTS playback rides on
// the exact same hourly bucket as chat messages rather than getting a
// second allowance stacked on top: a couple sending 20 chat messages an
// hour is already the intended ceiling, and TTS is just another way of
// consuming a chat reply, not a separate resource.
//
// Checked and logged BEFORE the caller's slow external call (OpenCode Zen
// or ElevenLabs) — a concurrent double request racing the check-then-insert
// would otherwise let both through, same class of race INSERT ... ON
// CONFLICT sidesteps for care actions. Here there's no natural conflict key
// to dedupe on (every message/speak call is legitimately distinct), so
// instead the log row is inserted immediately after the count passes,
// before the slow external call — a losing race just means an occasional
// request over quota by one, not unbounded spend, which is an acceptable
// tradeoff against the complexity of a transactional count-and-insert here.
func (h *handler) countAndLogChatMessage(teamID int64) error {
	var count int
	if err := h.db.QueryRow(
		`SELECT COUNT(*) FROM pet_chat_log WHERE team_id = $1 AND created_at > now() - interval '1 hour'`,
		teamID,
	).Scan(&count); err != nil {
		return fmt.Errorf("count chat log: %w", err)
	}
	if count >= chatRateLimitPerHour {
		return errChatRateLimited
	}
	if _, err := h.db.Exec(`INSERT INTO pet_chat_log (team_id) VALUES ($1)`, teamID); err != nil {
		return fmt.Errorf("log chat message: %w", err)
	}
	return nil
}

// chatRequest/chatResponse are colocated with the Chat handler rather than
// living in types.go — that file holds the shared State/ActionStatus shapes
// the whole module reads, while request/response structs for a single
// handler already live next to that handler elsewhere in this package (see
// renameRequest next to Rename in handlers.go). This follows that same
// convention instead of the literal file split originally sketched for it.
type chatRequest struct {
	Message string `json:"message"`
}

type chatResponse struct {
	Response string `json:"response"`
}

// opencodeChatCompletionsRequest/opencodeChatCompletionsResponse mirror the
// OpenAI chat-completions wire shape OpenCode Zen speaks. Only the fields
// this handler actually needs are modeled — no reason to round-trip a
// larger struct for a one-shot, non-streaming call.
type opencodeChatCompletionsRequest struct {
	Model    string                `json:"model"`
	Messages []opencodeChatMessage `json:"messages"`
}

type opencodeChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type opencodeChatCompletionsResponse struct {
	Choices []struct {
		Message opencodeChatMessage `json:"message"`
	} `json:"choices"`
}

// opencodeHTTPClient is a package-level client with a fixed timeout, same
// idiom cmd/workhubctl already uses for outbound calls (see health.go/
// user.go/auth.go's local `http.Client{Timeout: 5 * time.Second}`) — here as
// a package var since chat.go has no other natural place to construct it
// once per call, and there's nothing request-specific about it.
var opencodeHTTPClient = &http.Client{Timeout: 15 * time.Second}

// petChatSystemPromptTemplate is the pet's chat personality — informal
// Peruvian Spanish (not Rioplatense voseo: "tú", not "vos") with local
// jerga, matching the couple's actual locale (see SPEECH_LANG/limaLoc — this
// app targets Lima, not Buenos Aires) and the same warm couple-app tone the
// rest of the pet feature's UI copy already uses (see MascotaTab.tsx's
// Spanish strings). Kept as a Go string template rather than a
// prompt-building function since every placeholder is a plain substitution,
// no branching logic worth a builder for.
const petChatSystemPromptTemplate = `Eres %s, una mascota virtual pixel art que vive en una app de pareja.

ESTADO ACTUAL:
- Nombre: %s
- Estado de ánimo: %s (happy/neutral/sad/hungry)
- Racha de cuidado: %d días
- Chispas: %d
- Hoy ya la cuidaron: %s

REGLAS:
- Hablas como una mascota bebé/niña traviesa, con "tú" (nada de voseo
  argentino), tierna e inocentona — no como un adulto haciendo chistes
- Usas jerga peruana con naturalidad, sin forzarla: "pe", "causa", "bacán",
  "chévere", "al toque", "asu", "fren", "mostro", "tono" — mezclada con
  hablar infantil (diminutivos como "-ito/-ita", frases cortitas y simples,
  algo de trabalenguas/onomatopeyas), como una mascota criolla chiquita, no
  como caricatura de adulto
- Eres cariñosa, juguetona y un poco tontita/inocente, no irónica ni sarcástica
- Respondes corto (1-2 oraciones máximo)
- Si tu mood es "sad" o "hungry", haces pucheros / te quejas como niña chiquita
- Si tu mood es "happy", estás re contenta, saltas de la emoción
- Si no la cuidaron hoy, se lo recuerdas de forma sutil y tierna, sin culpar
- Usas emojis de pixel art occasionally
- NO haces preguntas, solo respondes
- NO mencionas que eres una IA o un bot`

// buildPetChatSystemPrompt fills the template with the pet's live state.
// name falls back to "tu mascota" when the pet hasn't been named yet
// (p.name == "") — an empty name in the prompt would read like a typo to
// the model, "tu mascota" is what the frontend already falls back to.
func buildPetChatSystemPrompt(p *petRow, caredToday bool) string {
	name := p.name
	if name == "" {
		name = "tu mascota"
	}
	caredStr := "no"
	if caredToday {
		caredStr = "sí"
	}
	return fmt.Sprintf(petChatSystemPromptTemplate, name, name, moodFor(p.careScore), p.streak, p.sparks, caredStr)
}

// Chat sends the user's message, alongside a system prompt built from the
// pet's live state, to OpenCode Zen and returns its reply. Rate-limited per
// team via pet_chat_log — see the insert-before-call comment below for why
// the quota row lands before the external call, not after.
func (h *handler) Chat(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := middleware.UserFromContext(r.Context()); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	var req chatRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	message := strings.TrimSpace(req.Message)
	if message == "" || len([]rune(message)) > chatMessageMaxRunes {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "message must be 1-500 characters")
		return
	}

	// Fail fast before touching the DB at all if chat isn't configured —
	// same "nice-to-have, disabled when unset" stance as Minio/VAPID, except
	// this route stays mounted (see config.OpencodeAPIKey's doc comment) so
	// it 503s per-request instead of 404ing.
	if h.opencodeAPIKey == "" {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable, "pet chat is not configured")
		return
	}

	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("pet: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	// Rate limit: count this team's chat messages in the last rolling hour.
	// Shared with Speak (speak.go) via countAndLogChatMessage — see that
	// function's doc comment for why TTS draws from the same bucket instead
	// of getting its own allowance.
	if err := h.countAndLogChatMessage(teamID); err != nil {
		if errors.Is(err, errChatRateLimited) {
			// httpx.CodeServiceUnavailable + 429 mirrors ytdlp's Download
			// handler ("server busy, try again in a moment") — this
			// codebase's existing convention for a 429, there's no separate
			// rate-limit code.
			httpx.WriteError(w, http.StatusTooManyRequests, httpx.CodeServiceUnavailable, "too many messages, try again later")
			return
		}
		log.Printf("pet: rate limit check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	p, err := h.loadOrCreate(teamID)
	if err != nil {
		log.Printf("pet: load state failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	doneToday, err := h.todayActionAttribution(teamID)
	if err != nil {
		log.Printf("pet: load today's care log failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	systemPrompt := buildPetChatSystemPrompt(p, len(doneToday) > 0)
	content, err := h.callOpencodeChat(systemPrompt, message)
	if err != nil {
		log.Printf("pet: chat completion failed: %v", err)
		httpx.WriteError(w, http.StatusBadGateway, httpx.CodeServiceUnavailable, "pet chat is unavailable right now")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, chatResponse{Response: content})
}

// callOpencodeChat sends the system+user messages to OpenCode Zen and
// returns the trimmed reply text. Any network error, non-2xx status, or
// empty choices list comes back as a plain error — the caller logs the
// detail and returns a generic message to the client, never the raw
// OpenCode error body (it could leak upstream implementation details, and
// isn't meaningful to a couple chatting with their pet anyway).
func (h *handler) callOpencodeChat(systemPrompt, userMessage string) (string, error) {
	body, err := json.Marshal(opencodeChatCompletionsRequest{
		Model: h.opencodeModel,
		Messages: []opencodeChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, opencodeChatURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.opencodeAPIKey)

	resp, err := opencodeHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call opencode: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read (and discard past a small cap) so the connection can be
		// reused, but never forward this body to the client — see the
		// doc comment above.
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("opencode returned status %d: %s", resp.StatusCode, string(errBody))
	}

	var parsed opencodeChatCompletionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("opencode returned no choices")
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("opencode returned empty content")
	}
	return content, nil
}
