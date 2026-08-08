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
	"workhub/modules/push"
	"workhub/shared/team"
)

// limaLoc anchors every "what day is it" calculation for the riddle game to
// Peru local time, regardless of the host/container's own TZ (prod runs in
// a Debian slim image with no tzdata installed, and even with tzdata
// present, relying on ambient server locale is fragile). Fixed offset, not
// IANA LoadLocation — Peru abolished DST in 1990, so UTC-5 never changes
// and this never needs a tzdata lookup that could fail to find one.
var limaLoc = time.FixedZone("America/Lima", -5*60*60)

// todayLima returns "today" as a Lima calendar date (YYYY-MM-DD). All
// daily_riddles.published_on comparisons and the seed's date assignment use
// this, not time.Now().Format(...), so the riddle's day boundary is always
// Lima midnight — matching what the countdown/expiry logic promises the
// user, instead of whatever timezone the process happens to be running in.
func todayLima() string {
	return time.Now().In(limaLoc).Format("2006-01-02")
}

// riddleExpiresAt returns the absolute instant (RFC3339) a riddle published
// on the given Lima calendar date stops being guessable — Lima midnight of
// the following day. Computed once here so both the countdown display and
// any future expiry check agree on the exact same instant.
func riddleExpiresAt(publishedOnLima string) string {
	d, err := time.ParseInLocation("2006-01-02", publishedOnLima, limaLoc)
	if err != nil {
		return ""
	}
	return d.Add(24 * time.Hour).Format(time.RFC3339)
}

// isBonusDay reports whether the given Lima calendar date is a "doble o
// nada" bonus day — once a week, every Sunday. A wrong guess on a bonus day
// doesn't cost a life (see SubmitRiddleGuess) — it's a side bet on points,
// separate from the normal lives/streak mechanic.
func isBonusDay(dateLima string) bool {
	d, err := time.ParseInLocation("2006-01-02", dateLima, limaLoc)
	if err != nil {
		return false
	}
	return d.Weekday() == time.Sunday
}

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

var accentReplacer = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
)

var spanishArticlePrefixes = []string{"el ", "la ", "los ", "las ", "un ", "una ", "unos ", "unas "}

// normalizeAnswer lowercases, strips accents, and drops a single leading
// Spanish article — so "El agujero" and "agujero" compare equal. Most seeded
// answers are "El X"/"La X" nouns; requiring the article on top of the noun
// made an already-hard riddle needlessly harder to type correctly.
func normalizeAnswer(s string) string {
	s = accentReplacer.Replace(strings.ToLower(strings.TrimSpace(s)))
	for _, art := range spanishArticlePrefixes {
		if rest, ok := strings.CutPrefix(s, art); ok {
			s = rest
			break
		}
	}
	return strings.TrimSpace(s)
}

// answerMatches checks a guess against a stored answer, which may hold
// multiple comma-separated valid alternatives (e.g. "Radar, nivel, kayak,
// oteo") — matches if the normalized guess equals ANY alternative.
func answerMatches(guess, stored string) bool {
	guessNorm := normalizeAnswer(guess)
	for _, alt := range strings.Split(stored, ",") {
		if normalizeAnswer(alt) == guessNorm {
			return true
		}
	}
	return false
}

// GetRiddleToday returns today's riddle (question, hint, difficulty only).
func (h *handler) GetRiddleToday(w http.ResponseWriter, r *http.Request) {
	today := todayLima()
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
	rid.ExpiresAt = riddleExpiresAt(today)
	rid.IsBonus = isBonusDay(today)
	httpx.WriteJSON(w, http.StatusOK, riddleTodayResponse{Riddle: &rid})
}

// GetRiddleSession returns (or creates) the shared team's riddle session.
// The caller's own identity doesn't matter beyond "are they logged in" —
// team_id is the shared couple id (see shared/team.ResolveTeamID), not per-caller.
func (h *handler) GetRiddleSession(w http.ResponseWriter, r *http.Request) {
	_, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	today := todayLima()

	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("games: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	// Expire any session left dangling 'active' from a day the team never
	// came back to finish — lives/status get updated in place.
	justGameoverID, err := h.expireStaleSessions(teamID, today)
	if err != nil {
		log.Printf("games: expire stale sessions failed: %v", err)
		// non-fatal — worst case a session outlives its day by one more poll
	}
	if justGameoverID != 0 {
		// Surface the loss on its own — don't silently roll straight into a
		// freshly reset session in the same response. The next call (the
		// user reopening the app, or a "jugar de nuevo" retry) will see this
		// same session already 'gameover' from a prior request and proceed
		// past this branch into the carry-over logic below, which resets to
		// 3/0/0 and creates today's session normally.
		s, err := h.loadRiddleSession(justGameoverID)
		if err != nil {
			log.Printf("games: load just-gameover session failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		h.populateAnswerIfRevealed(s)
		httpx.WriteJSON(w, http.StatusOK, riddleSessionResponse{Session: s})
		return
	}

	var todayRiddleID sql.NullInt64
	if err := h.db.QueryRow(
		`SELECT id FROM daily_riddles WHERE published_on = $1`, today,
	).Scan(&todayRiddleID); err != nil && err != sql.ErrNoRows {
		log.Printf("games: get riddle session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if !todayRiddleID.Valid {
		// No riddle exists for today at all — nothing to attach a session to.
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "no riddle available for today")
		return
	}

	// Lives, scores and streak carry forward day to day — the "game" is the
	// run across sessions, not any single day's session. A fresh 3/0/0/0
	// start only happens the very first time a team plays, or right after a
	// gameover (the expiry sweep above may have just produced one).
	carryLives, carryP1, carryP2, carryStreak := 3, 0, 0, 0
	var lastStatus string
	err = h.db.QueryRow(
		`SELECT lives_remaining, p1_score, p2_score, status, streak FROM riddle_game_sessions
		 WHERE team_id = $1 ORDER BY id DESC LIMIT 1`, teamID,
	).Scan(&carryLives, &carryP1, &carryP2, &lastStatus, &carryStreak)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("games: get riddle session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if lastStatus == "gameover" {
		carryLives, carryP1, carryP2, carryStreak = 3, 0, 0, 0
	}

	// Upsert session for the shared team + today's riddle.
	//
	// (team_id, current_riddle_id) is UNIQUE — ON CONFLICT reuses the day's
	// existing session instead of inserting a new blank one on every call
	// (this used to insert unconditionally, so every poll tick spawned a
	// fresh 'active' row and buried any solved/scored state under it).
	if _, err := h.db.Exec(
		`INSERT INTO riddle_game_sessions (team_id, partner_id, current_riddle_id, status, lives_remaining, p1_score, p2_score, streak)
		 VALUES ($1, $1, $2, 'active', $3, $4, $5, $6)
		 ON CONFLICT (team_id, current_riddle_id) DO NOTHING`,
		teamID, todayRiddleID.Int64, carryLives, carryP1, carryP2, carryStreak,
	); err != nil {
		log.Printf("games: get riddle session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	var sessionID int64
	if err := h.db.QueryRow(
		`SELECT id FROM riddle_game_sessions WHERE team_id = $1 AND current_riddle_id = $2`,
		teamID, todayRiddleID.Int64,
	).Scan(&sessionID); err != nil {
		log.Printf("games: get riddle session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	s, err := h.loadRiddleSession(sessionID)
	if err != nil {
		log.Printf("games: get riddle session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	h.populateAnswerIfRevealed(s)
	httpx.WriteJSON(w, http.StatusOK, riddleSessionResponse{Session: s})
}

// loadRiddleSession fetches a session row by id in full.
func (h *handler) loadRiddleSession(sessionID int64) (*RiddleGameSession, error) {
	srow := h.db.QueryRow(
		`SELECT id, team_id, partner_id, lives_remaining, p1_score, p2_score,
		        current_riddle_id, status, created_at, streak
		 FROM riddle_game_sessions WHERE id = $1`, sessionID)
	var s RiddleGameSession
	var currentRiddleID sql.NullInt64
	var createdAt time.Time
	if err := srow.Scan(&s.ID, &s.TeamID, &s.PartnerID, &s.LivesRemaining, &s.P1Score, &s.P2Score,
		&currentRiddleID, &s.Status, &createdAt, &s.Streak); err != nil {
		return nil, err
	}
	if currentRiddleID.Valid {
		s.CurrentRiddleID = &currentRiddleID.Int64
	}
	s.CreatedAt = createdAt.Format(time.RFC3339)
	return &s, nil
}

// expireStaleSessions closes out any 'active' session belonging to team
// whose riddle's day has already passed (published_on < today) — i.e. the
// team never guessed before the 24h window closed. Losing a life; hitting 0
// lives ends the run at 'gameover' instead of 'expired'. In practice there's
// at most one such row at a time (a new session for "today" is only ever
// created when GetRiddleSession is actually called for that day), but this
// loops rather than assuming that invariant holds forever.
//
// Returns the id of a session that JUST flipped to 'gameover' in this call
// (0 if none) — the caller uses this to show the loss screen on its own
// beat instead of silently rolling straight into a freshly reset session
// within the same response.
func (h *handler) expireStaleSessions(teamID int64, today string) (int64, error) {
	rows, err := h.db.Query(
		`SELECT rgs.id, rgs.lives_remaining
		 FROM riddle_game_sessions rgs
		 JOIN daily_riddles dr ON dr.id = rgs.current_riddle_id
		 WHERE rgs.team_id = $1 AND rgs.status = 'active' AND dr.published_on < $2`,
		teamID, today,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type stale struct {
		id    int64
		lives int
	}
	var toExpire []stale
	for rows.Next() {
		var s stale
		if err := rows.Scan(&s.id, &s.lives); err != nil {
			return 0, err
		}
		toExpire = append(toExpire, s)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	var justGameoverID int64
	for _, s := range toExpire {
		newLives := s.lives - 1
		newStatus := "expired"
		if newLives <= 0 {
			newLives = 0
			newStatus = "gameover"
			justGameoverID = s.id
		}
		// A missed day breaks the streak regardless of whether it also ends
		// the run — reset it here rather than only on gameover, so a single
		// missed day always costs the streak, not just the one that empties
		// the last life.
		if _, err := h.db.Exec(
			`UPDATE riddle_game_sessions SET status = $1, lives_remaining = $2, streak = 0 WHERE id = $3`,
			newStatus, newLives, s.id,
		); err != nil {
			return 0, err
		}
	}
	return justGameoverID, nil
}

// populateAnswerIfRevealed fills s.Answer once the round is over (solved or
// gameover) — never while active, so a poll mid-round can't leak it. Errors
// are logged but non-fatal: the session itself is still valid without the
// answer attached, the client just won't have it to display yet.
func (h *handler) populateAnswerIfRevealed(s *RiddleGameSession) {
	if s.Status == "active" || s.CurrentRiddleID == nil {
		return
	}
	var answer string
	if err := h.db.QueryRow(
		`SELECT answer FROM daily_riddles WHERE id = $1`, *s.CurrentRiddleID,
	).Scan(&answer); err != nil {
		log.Printf("games: populate answer failed: %v", err)
		return
	}
	s.Answer = &answer
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

	today := todayLima()

	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("games: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	// Get session
	var s RiddleGameSession
	var currentRiddleID sql.NullInt64
	var createdAt time.Time
	err = h.db.QueryRow(
		`SELECT id, team_id, partner_id, lives_remaining, p1_score, p2_score,
		        current_riddle_id, status, created_at, streak
		 FROM riddle_game_sessions WHERE team_id = $1 AND status = 'active'
		 ORDER BY id DESC LIMIT 1`,
		teamID).Scan(&s.ID, &s.TeamID, &s.PartnerID, &s.LivesRemaining, &s.P1Score, &s.P2Score,
		&currentRiddleID, &s.Status, &createdAt, &s.Streak)
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
	// Which side of the team solved (team has p1 and p2 — caller is
	// whichever matches). Computed up front since the bonus-day wrong-guess
	// path below needs it too, not just the correct-guess path.
	isP1 := userID == s.TeamID

	if s.Status != "active" {
		h.populateAnswerIfRevealed(&s)
		httpx.WriteJSON(w, http.StatusOK, riddleGuessResponse{Correct: false, PointsEarned: 0, Session: s})
		return
	}

	// Get today's riddle answer
	var rid DailyRiddle
	err = h.db.QueryRow(
		`SELECT id, question, answer, hint, difficulty
		 FROM daily_riddles WHERE published_on = $1`, today).
		Scan(&rid.ID, &rid.Question, &rid.Answer, &rid.Hint, &rid.Difficulty)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "no riddle for today")
		return
	}
	if err != nil {
		log.Printf("games: submit guess riddle lookup failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if !answerMatches(guess, rid.Answer) {
		// "Doble o nada" — a wrong guess on the weekly bonus day wipes the
		// solver's own score (never lives/streak, that's a separate
		// mechanic; this is purely a side bet on points). Naturally
		// idempotent: once busted to 0, retrying wipes 0 to 0 — no need to
		// track "already lost this round" separately, and the session stays
		// active so they can keep trying for the correct answer.
		if isBonusDay(today) {
			if isP1 && s.P1Score > 0 {
				s.P1Score = 0
			} else if !isP1 && s.P2Score > 0 {
				s.P2Score = 0
			}
			if _, err := h.db.Exec(
				`UPDATE riddle_game_sessions SET p1_score = $1, p2_score = $2 WHERE id = $3`,
				s.P1Score, s.P2Score, s.ID,
			); err != nil {
				log.Printf("games: bonus-day score wipe failed: %v", err)
			}
		}
		httpx.WriteJSON(w, http.StatusOK, riddleGuessResponse{Correct: false, PointsEarned: 0, Session: s})
		return
	}

	var points int
	if isBonusDay(today) {
		// "Doble o nada" — correct answer doubles whatever the solver
		// currently has (0 if they already busted a wrong attempt this
		// round). No speed tiers, no streak bonus on top — it's a separate
		// side bet on points, deliberately not stacked with the normal
		// scoring mechanic. Streak itself still increments below: they did
		// solve today's riddle, and streak tracks lives/lives-mechanic
		// consistency, not points.
		current := s.P2Score
		if isP1 {
			current = s.P1Score
		}
		points = current
	} else {
		// Calculate points based on elapsed time since riddle was published
		// (published at Lima midnight, so elapsed = hours since then).
		//
		// Deliberately NOT parsing the driver's `publishedOn` string here: a
		// DATE column comes back as full RFC3339 ("2026-08-08T00:00:00Z")
		// which is UTC midnight of that calendar date, not Lima midnight —
		// a 5h gap. (This used to also fail outright: parsing that string
		// with a bare "2006-01-02" layout errored silently, zeroing
		// publishedTime and scoring every correct guess 0 — both bugs
		// shared the same root cause, client/server disagreeing on a
		// timezone without an explicit anchor.) `today` is already the Lima
		// calendar date, so anchor to that directly.
		publishedTime, err := time.ParseInLocation("2006-01-02", today, limaLoc)
		if err != nil {
			log.Printf("games: parse today %q failed: %v", today, err)
		}
		elapsedHours := time.Since(publishedTime).Hours()
		points = calcPoints(elapsedHours)

		// Streak bonus scales with the NEW streak (so day 1 already earns
		// something, not just day 2+), capped at +50 so it stays a nice
		// add-on rather than dwarfing the speed-tier points above. Bonus
		// days skip this — see branch above.
		streakBonus := min(s.Streak+1, 10) * 5
		points += streakBonus
	}
	s.Streak++

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
		`UPDATE riddle_game_sessions SET status = 'solved', p1_score = $1, p2_score = $2, streak = $3 WHERE id = $4`,
		s.P1Score, s.P2Score, s.Streak, s.ID)
	s.Status = "solved"
	s.Answer = &rid.Answer

	// Let the partner know — fired in the background so a slow/unreachable
	// push service can't add latency to the guess response itself. Skipped
	// entirely when push isn't configured (see routes.go).
	if h.vapidPublicKey != "" && h.vapidPrivateKey != "" {
		var solverName string
		if err := h.db.QueryRow(`SELECT name FROM users WHERE id = $1`, userID).Scan(&solverName); err != nil {
			log.Printf("games: load solver name for notification failed: %v", err)
		} else {
			go push.NotifyPartnerSolved(h.db, h.vapidPublicKey, h.vapidPrivateKey, h.vapidSubject, userID, solverName)
		}
	}

	httpx.WriteJSON(w, http.StatusOK, riddleGuessResponse{Correct: true, PointsEarned: points, Session: s})
}

// GetRiddleHistory returns past solved/expired rounds for the team.
func (h *handler) GetRiddleHistory(w http.ResponseWriter, r *http.Request) {
	_, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("games: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
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
		 LIMIT 30`, teamID)
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
