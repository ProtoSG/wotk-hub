package games

import (
	"database/sql"
	"log"
	"net/http"
	"slices"
	"strings"
	"workhub/httpx"
	"workhub/middleware"
)

func (h *handler) ListMovies(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, emoji_str, answer, difficulty, created_at FROM emoji_movies ORDER BY id`)
	if err != nil {
		log.Printf("games: list movies failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	movies := []EmojiMovie{}
	for rows.Next() {
		m, err := scanMovie(rows)
		if err != nil {
			log.Printf("games: scan movie failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		movies = append(movies, m)
	}
	httpx.WriteJSON(w, http.StatusOK, listMoviesResponse{Movies: movies})
}

func (h *handler) CreateMovie(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	var req createMovieRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	req.EmojiStr = strings.TrimSpace(req.EmojiStr)
	req.Answer = strings.TrimSpace(req.Answer)
	if req.EmojiStr == "" || req.Answer == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "emojiStr and answer are required")
		return
	}
	if !slices.Contains(difficulties, req.Difficulty) {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid difficulty")
		return
	}

	row := h.db.QueryRow(
		`INSERT INTO emoji_movies (emoji_str, answer, difficulty, created_by)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, emoji_str, answer, difficulty, created_at`,
		req.EmojiStr, req.Answer, req.Difficulty, userID,
	)
	m, err := scanMovie(row)
	if err != nil {
		log.Printf("games: create movie failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, m)
}

func (h *handler) RandomMovie(w http.ResponseWriter, r *http.Request) {
	difficulty := r.URL.Query().Get("difficulty")
	if difficulty != "" && !slices.Contains(difficulties, difficulty) {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid difficulty")
		return
	}
	m, err := pickRandomMovie(h.db, difficulty)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "no movies available")
		return
	}
	if err != nil {
		log.Printf("games: random movie failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, puzzleResponse{ID: m.ID, EmojiStr: m.EmojiStr, Difficulty: m.Difficulty})
}

func (h *handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	var req createSessionRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if req.MovieDifficulty != "" && !slices.Contains(difficulties, req.MovieDifficulty) {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid movieDifficulty")
		return
	}

	movie, err := pickRandomMovie(h.db, req.MovieDifficulty)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "no movies available")
		return
	}
	if err != nil {
		log.Printf("games: create session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	row := h.db.QueryRow(
		`INSERT INTO emoji_game_sessions (player1_id, current_emoji, current_answer, status)
		 VALUES ($1, $2, $3, 'waiting')
		 RETURNING `+sessionColumns,
		userID, movie.EmojiStr, movie.Answer,
	)
	s, err := scanSession(row)
	if err != nil {
		log.Printf("games: create session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, s)
}

func (h *handler) GetSession(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	row := h.db.QueryRow(`SELECT `+sessionColumns+` FROM emoji_game_sessions WHERE id = $1`, id)
	s, err := scanSession(row)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "session not found")
		return
	}
	if err != nil {
		log.Printf("games: get session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s)
}

// JoinSession assigns the caller as player2 and flips the session to
// active. The WHERE clause enforces both invariants atomically (still
// waiting, not joining your own session) so two concurrent joins can't both
// succeed.
func (h *handler) JoinSession(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	id, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	row := h.db.QueryRow(
		`UPDATE emoji_game_sessions
		 SET player2_id = $1, status = 'active'
		 WHERE id = $2 AND status = 'waiting' AND player1_id != $1
		 RETURNING `+sessionColumns,
		userID, id,
	)
	s, err := scanSession(row)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "session is not joinable")
		return
	}
	if err != nil {
		log.Printf("games: join session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s)
}

// Guess checks the caller's guess against the active puzzle. A correct
// guess scores the caller and immediately loads the next puzzle (or ends
// the session at winningScore) — a wrong guess changes nothing and leaks
// no hint, per the plan's "Nop, otra vez" feedback.
func (h *handler) Guess(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	id, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	var req guessRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	guess := strings.TrimSpace(req.Guess)
	if guess == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "guess is required")
		return
	}

	row := h.db.QueryRow(`SELECT `+sessionColumns+` FROM emoji_game_sessions WHERE id = $1`, id)
	s, err := scanSession(row)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "session not found")
		return
	}
	if err != nil {
		log.Printf("games: guess failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if s.Status != "active" {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "session is not active")
		return
	}
	if !isPlayer(s, userID) {
		httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden, "forbidden")
		return
	}

	if !strings.EqualFold(guess, s.CurrentAnswer) {
		httpx.WriteJSON(w, http.StatusOK, guessResponse{Correct: false, Session: s})
		return
	}

	p1Score, p2Score := s.P1Score, s.P2Score
	if userID == s.Player1ID {
		p1Score++
	} else {
		p2Score++
	}

	status := s.Status
	nextEmoji, nextAnswer := s.CurrentEmoji, s.CurrentAnswer
	if p1Score >= winningScore || p2Score >= winningScore {
		status = "finished"
	} else {
		movie, err := pickRandomMovie(h.db, "")
		if err != nil {
			log.Printf("games: guess next puzzle failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		nextEmoji, nextAnswer = movie.EmojiStr, movie.Answer
	}

	row = h.db.QueryRow(
		`UPDATE emoji_game_sessions
		 SET p1_score = $1, p2_score = $2, current_emoji = $3, current_answer = $4, status = $5
		 WHERE id = $6
		 RETURNING `+sessionColumns,
		p1Score, p2Score, nextEmoji, nextAnswer, status, id,
	)
	updated, err := scanSession(row)
	if err != nil {
		log.Printf("games: guess update failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, guessResponse{Correct: true, Session: updated})
}

// Reveal shows the current answer without scoring anyone, then advances to
// the next puzzle — an escape hatch for when both players are stuck.
func (h *handler) Reveal(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	id, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	row := h.db.QueryRow(`SELECT `+sessionColumns+` FROM emoji_game_sessions WHERE id = $1`, id)
	s, err := scanSession(row)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "session not found")
		return
	}
	if err != nil {
		log.Printf("games: reveal failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if s.Status != "active" {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "session is not active")
		return
	}
	if !isPlayer(s, userID) {
		httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden, "forbidden")
		return
	}

	answer := s.CurrentAnswer
	movie, err := pickRandomMovie(h.db, "")
	if err != nil {
		log.Printf("games: reveal next puzzle failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	row = h.db.QueryRow(
		`UPDATE emoji_game_sessions
		 SET current_emoji = $1, current_answer = $2
		 WHERE id = $3
		 RETURNING `+sessionColumns,
		movie.EmojiStr, movie.Answer, id,
	)
	updated, err := scanSession(row)
	if err != nil {
		log.Printf("games: reveal update failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, revealResponse{Answer: answer, Session: updated})
}

func (h *handler) ActiveSessions(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	rows, err := h.db.Query(
		`SELECT `+sessionColumns+`
		 FROM emoji_game_sessions
		 WHERE (player1_id = $1 OR player2_id = $1) AND status IN ('waiting', 'active')
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		log.Printf("games: active sessions failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	sessions := []EmojiGameSession{}
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			log.Printf("games: scan session failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		sessions = append(sessions, s)
	}
	httpx.WriteJSON(w, http.StatusOK, listSessionsResponse{Sessions: sessions})
}
