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
