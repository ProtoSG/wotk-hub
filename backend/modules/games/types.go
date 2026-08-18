package games

type EmojiMovie struct {
	ID         int64  `json:"id"`
	EmojiStr   string `json:"emojiStr"`
	Answer     string `json:"answer"`
	Difficulty string `json:"difficulty"`
	CreatedAt  string `json:"createdAt"`
}

// EmojiGameSession is the shared state of a two-player round. CurrentAnswer
// is never serialized (json:"-") so a player can't read it straight off
// GetSession/CreateSession/JoinSession responses — same cheat-prevention
// intent as puzzleResponse below.
type EmojiGameSession struct {
	ID            int64  `json:"id"`
	Player1ID     int64  `json:"player1Id"`
	Player2ID     *int64 `json:"player2Id,omitempty"`
	P1Score       int    `json:"p1Score"`
	P2Score       int    `json:"p2Score"`
	CurrentEmoji  string `json:"currentEmoji"`
	CurrentAnswer string `json:"-"`
	Status        string `json:"status"`
	CreatedAt     string `json:"createdAt"`
}

type listMoviesResponse struct {
	Movies []EmojiMovie `json:"movies"`
}

type listSessionsResponse struct {
	Sessions []EmojiGameSession `json:"sessions"`
}

// puzzleResponse is what GET /emoji-movies/random returns — id, emoji and
// difficulty only, so a client can't read the answer off the wire before
// guessing.
type puzzleResponse struct {
	ID         int64  `json:"id"`
	EmojiStr   string `json:"emojiStr"`
	Difficulty string `json:"difficulty"`
}

type createMovieRequest struct {
	EmojiStr   string `json:"emojiStr"`
	Answer     string `json:"answer"`
	Difficulty string `json:"difficulty"`
}

type createSessionRequest struct {
	MovieDifficulty string `json:"movieDifficulty"`
}

type guessRequest struct {
	Guess string `json:"guess"`
}

type guessResponse struct {
	Correct bool             `json:"correct"`
	Session EmojiGameSession `json:"session"`
}

type revealResponse struct {
	Answer  string           `json:"answer"`
	Session EmojiGameSession `json:"session"`
}

// ─── Riddle Game ─────────────────────────────────────────────────────────────

// DailyRiddle is the seeded puzzle shown to both partners each day. Answer is
// never serialised to the client (json:"-").
type DailyRiddle struct {
	ID          int64  `json:"id"`
	Question    string `json:"question"`
	Answer      string `json:"-"`
	Hint        string `json:"hint"`
	Difficulty  string `json:"difficulty"`
	PublishedOn string `json:"publishedOn"`
	CreatedAt   string `json:"createdAt"`
	// ExpiresAt is the absolute instant (RFC3339) this riddle's 24h window
	// closes — always Lima-local midnight of the day after PublishedOn,
	// computed server-side so the client never has to guess a timezone from
	// a bare date string.
	ExpiresAt string `json:"expiresAt"`
	// IsBonus marks the weekly "doble o nada" riddle (every Sunday, Lima
	// time) — the frontend gates the guess input behind an explicit
	// confirm on these days since a wrong guess wipes the solver's score.
	IsBonus bool `json:"isBonus"`
}

// RiddleGameSession holds the shared state for a team's daily riddle round.
// Lives are shared between both partners; scores are individual.
type RiddleGameSession struct {
	ID             int64 `json:"id"`
	TeamID         int64 `json:"teamId"`
	PartnerID      int64 `json:"partnerId"`
	LivesRemaining int   `json:"livesRemaining"`
	P1Score        int   `json:"p1Score"`
	P2Score        int   `json:"p2Score"`
	// Streak is consecutive days solved without a miss. Resets to 0 the
	// moment a day expires unsolved (see expireStaleSessions).
	Streak          int    `json:"streak"`
	CurrentRiddleID *int64 `json:"currentRiddleId,omitempty"`
	Status          string `json:"status"` // active | solved | expired | gameover
	CreatedAt       string `json:"createdAt"`
	// Answer is only populated once Status is 'solved' or 'gameover' — never
	// while active, so polling an in-progress session can't leak it.
	Answer *string `json:"answer,omitempty"`
}

// RiddleAttempt records a correct solve.
type RiddleAttempt struct {
	ID           int64  `json:"id"`
	SessionID    int64  `json:"sessionId"`
	RiddleID     int64  `json:"riddleId"`
	SolverID     int64  `json:"solverId"`
	SolvedAt     string `json:"solvedAt"`
	PointsEarned int    `json:"pointsEarned"`
}

type riddleTodayResponse struct {
	Riddle *DailyRiddle `json:"riddle,omitempty"`
}

type riddleSessionResponse struct {
	Session *RiddleGameSession `json:"session,omitempty"`
}

type riddleGuessRequest struct {
	Guess string `json:"guess"`
}

type riddleGuessResponse struct {
	Correct      bool              `json:"correct"`
	PointsEarned int               `json:"pointsEarned"`
	Session      RiddleGameSession `json:"session"`
}

type riddleHistoryItem struct {
	RiddleID     int64  `json:"riddleId"`
	Question     string `json:"question"`
	Answer       string `json:"answer"` // revealed in history after solve
	SolvedBy     string `json:"solvedBy"`
	SolvedAt     string `json:"solvedAt"`
	PointsEarned int    `json:"pointsEarned"`
	Expired      bool   `json:"expired"`
}

type riddleHistoryResponse struct {
	History []riddleHistoryItem `json:"history"`
}

// ─── WebSocket Events ──────────────────────────────────────────────────────

// Event type discriminators the frontend switches on — see
// useGameSocket.ts. One event shape per game, both carrying the fresh
// session exactly as the corresponding REST response would.
const (
	wsEventTypeEmojiMovies = "emoji_movies.session"
	wsEventTypeRiddle      = "riddle.session"
)

// wsEmojiMoviesEvent / wsRiddleEvent are pushed over the games WS hub after
// any handler mutates the corresponding session — purely how the OTHER
// player's screen updates without polling; the REST response to the acting
// player is unchanged.
type wsEmojiMoviesEvent struct {
	Type    string           `json:"type"`
	Session EmojiGameSession `json:"session"`
}

type wsRiddleEvent struct {
	Type    string            `json:"type"`
	Session RiddleGameSession `json:"session"`
}
