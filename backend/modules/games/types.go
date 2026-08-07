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
	ID         int64  `json:"id"`
	Question   string `json:"question"`
	Answer     string `json:"-"`
	Hint       string `json:"hint"`
	Difficulty string `json:"difficulty"`
	PublishedOn string `json:"publishedOn"`
	CreatedAt  string `json:"createdAt"`
}

// RiddleGameSession holds the shared state for a team's daily riddle round.
// Lives are shared between both partners; scores are individual.
type RiddleGameSession struct {
	ID              int64  `json:"id"`
	TeamID          int64  `json:"teamId"`
	PartnerID       int64  `json:"partnerId"`
	LivesRemaining  int    `json:"livesRemaining"`
	P1Score         int    `json:"p1Score"`
	P2Score         int    `json:"p2Score"`
	CurrentRiddleID *int64 `json:"currentRiddleId,omitempty"`
	Status          string `json:"status"` // active | solved | expired | gameover
	CreatedAt       string `json:"createdAt"`
}

// RiddleAttempt records a correct solve.
type RiddleAttempt struct {
	ID            int64  `json:"id"`
	SessionID     int64  `json:"sessionId"`
	RiddleID      int64  `json:"riddleId"`
	SolverID      int64  `json:"solverId"`
	SolvedAt      string `json:"solvedAt"`
	PointsEarned  int    `json:"pointsEarned"`
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
