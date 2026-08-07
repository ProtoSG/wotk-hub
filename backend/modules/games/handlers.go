package games

import (
	"database/sql"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"
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

// ─── Riddle Game Handlers ─────────────────────────────────────────────────────

// scoring windows in hours → points
var scoringWindows = []struct {
	hoursMax int
	points   int
}{
	{1, 100},
	{6, 75},
	{12, 60},
	{24, 50},
}

func calcPoints(elapsedHours float64) int {
	for _, w := range scoringWindows {
		if elapsedHours < float64(w.hoursMax) {
			return w.points
		}
	}
	return 0
}

// GetRiddleToday returns today's riddle (question, hint, difficulty only).
func (h *handler) GetRiddleToday(w http.ResponseWriter, r *http.Request) {
	today := time.Now().Format("2006-01-02")
	row := h.db.QueryRow(
		`SELECT id, question, hint, difficulty, published_on, created_at
		 FROM daily_riddles WHERE published_on = $1`, today)
	var rid DailyRiddle
	var publishedOn string
	var createdAt time.Time
	err := row.Scan(&rid.ID, &rid.Question, &rid.Hint, &rid.Difficulty, &publishedOn, &createdAt)
	if err == sql.ErrNoRows {
		httpx.WriteJSON(w, http.StatusOK, riddleTodayResponse{Riddle: nil})
		return
	}
	if err != nil {
		log.Printf("games: get riddle today failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	rid.PublishedOn = publishedOn
	rid.CreatedAt = createdAt.Format(time.RFC3339)
	httpx.WriteJSON(w, http.StatusOK, riddleTodayResponse{Riddle: &rid})
}

// GetRiddleSession returns (or creates) the current team's riddle session.
func (h *handler) GetRiddleSession(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	today := time.Now().Format("2006-01-02")

	// Upsert session for this team. team_id = userID's "team" (treated as userID
	// for now — the couple relationship is out of scope, so each user is their own team).
	// partner_id = same user (placeholder until couple module provides the link).
	var sessionID int64
	err := h.db.QueryRow(
		`INSERT INTO riddle_game_sessions (team_id, partner_id, current_riddle_id, status)
		 SELECT $1, $1, id, 'active'
		 FROM daily_riddles WHERE published_on = $2
		 ON CONFLICT DO NOTHING
		 RETURNING id`,
		userID, today).Scan(&sessionID)
	if err == sql.ErrNoRows {
		// No row returned — session already exists, fetch it
		err = h.db.QueryRow(
			`SELECT id FROM riddle_game_sessions WHERE team_id = $1 ORDER BY id DESC LIMIT 1`,
			userID).Scan(&sessionID)
	}
	if err != nil {
		log.Printf("games: get riddle session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	// Load full session
	srow := h.db.QueryRow(
		`SELECT id, team_id, partner_id, lives_remaining, p1_score, p2_score,
		        current_riddle_id, status, created_at
		 FROM riddle_game_sessions WHERE id = $1`, sessionID)
	var s RiddleGameSession
	var currentRiddleID sql.NullInt64
	var createdAt time.Time
	err = srow.Scan(&s.ID, &s.TeamID, &s.PartnerID, &s.LivesRemaining, &s.P1Score, &s.P2Score,
		&currentRiddleID, &s.Status, &createdAt)
	if err != nil {
		log.Printf("games: get riddle session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if currentRiddleID.Valid {
		s.CurrentRiddleID = &currentRiddleID.Int64
	}
	s.CreatedAt = createdAt.Format(time.RFC3339)
	httpx.WriteJSON(w, http.StatusOK, riddleSessionResponse{Session: &s})
}

// SubmitRiddleGuess checks a guess against today's riddle.
func (h *handler) SubmitRiddleGuess(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	var req riddleGuessRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	guess := strings.TrimSpace(req.Guess)
	if guess == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "guess is required")
		return
	}

	today := time.Now().Format("2006-01-02")

	// Get session
	var s RiddleGameSession
	var currentRiddleID sql.NullInt64
	var createdAt time.Time
	err := h.db.QueryRow(
		`SELECT id, team_id, partner_id, lives_remaining, p1_score, p2_score,
		        current_riddle_id, status, created_at
		 FROM riddle_game_sessions WHERE team_id = $1 AND status = 'active'
		 ORDER BY id DESC LIMIT 1`,
		userID).Scan(&s.ID, &s.TeamID, &s.PartnerID, &s.LivesRemaining, &s.P1Score, &s.P2Score,
		&currentRiddleID, &s.Status, &createdAt)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "no active session")
		return
	}
	if err != nil {
		log.Printf("games: submit guess session lookup failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if currentRiddleID.Valid {
		s.CurrentRiddleID = &currentRiddleID.Int64
	}
	s.CreatedAt = createdAt.Format(time.RFC3339)

	if s.Status != "active" {
		httpx.WriteJSON(w, http.StatusOK, riddleGuessResponse{Correct: false, PointsEarned: 0, Session: s})
		return
	}

	// Get today's riddle answer
	var rid DailyRiddle
	var publishedOn string
	err = h.db.QueryRow(
		`SELECT id, question, answer, hint, difficulty, published_on
		 FROM daily_riddles WHERE published_on = $1`, today).
		Scan(&rid.ID, &rid.Question, &rid.Answer, &rid.Hint, &rid.Difficulty, &publishedOn)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "no riddle for today")
		return
	}
	if err != nil {
		log.Printf("games: submit guess riddle lookup failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if !strings.EqualFold(guess, rid.Answer) {
		httpx.WriteJSON(w, http.StatusOK, riddleGuessResponse{Correct: false, PointsEarned: 0, Session: s})
		return
	}

	// Correct! Calculate points based on elapsed time since riddle was published
	// (published at midnight, so elapsed = hours since midnight)
	publishedTime, _ := time.Parse("2006-01-02", publishedOn)
	elapsedHours := time.Since(publishedTime).Hours()
	points := calcPoints(elapsedHours)

	// Determine who solved (team has p1 and p2 — caller is whichever matches)
	isP1 := userID == s.TeamID
	if isP1 {
		s.P1Score += points
	} else {
		s.P2Score += points
	}

	// Record attempt and update session to solved
	h.db.Exec(
		`INSERT INTO riddle_attempts (session_id, riddle_id, solver_id, points_earned)
		 VALUES ($1, $2, $3, $4)`,
		s.ID, rid.ID, userID, points)
	h.db.Exec(
		`UPDATE riddle_game_sessions SET status = 'solved', p1_score = $1, p2_score = $2 WHERE id = $3`,
		s.P1Score, s.P2Score, s.ID)
	s.Status = "solved"

	httpx.WriteJSON(w, http.StatusOK, riddleGuessResponse{Correct: true, PointsEarned: points, Session: s})
}

// GetRiddleHistory returns past solved/expired rounds for the team.
func (h *handler) GetRiddleHistory(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	rows, err := h.db.Query(
		`SELECT ra.riddle_id, dr.question, dr.answer, u.name, ra.solved_at, ra.points_earned
		 FROM riddle_attempts ra
		 JOIN riddle_game_sessions rs ON ra.session_id = rs.id
		 JOIN daily_riddles dr ON ra.riddle_id = dr.id
		 JOIN users u ON ra.solver_id = u.id
		 WHERE rs.team_id = $1
		 ORDER BY ra.solved_at DESC
		 LIMIT 30`, userID)
	if err != nil {
		log.Printf("games: get riddle history failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	var history []riddleHistoryItem
	for rows.Next() {
		var h riddleHistoryItem
		var solvedAt time.Time
		if err := rows.Scan(&h.RiddleID, &h.Question, &h.Answer, &h.SolvedBy, &solvedAt, &h.PointsEarned); err != nil {
			log.Printf("games: scan riddle history row failed: %v", err)
			continue
		}
		h.SolvedAt = solvedAt.Format(time.RFC3339)
		h.Expired = false
		history = append(history, h)
	}
	if history == nil {
		history = []riddleHistoryItem{}
	}
	httpx.WriteJSON(w, http.StatusOK, riddleHistoryResponse{History: history})
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
