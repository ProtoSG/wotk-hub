package games

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	chi "github.com/go-chi/chi/v5"
)

var difficulties = []string{"easy", "medium", "hard"}

// winningScore is the number of correct guesses needed to end a session —
// the plan's spec has no explicit finish trigger, so this is the simplest
// rule that produces the "End screen — winner announcement" flow.
const winningScore = 5

const sessionColumns = `id, player1_id, player2_id, p1_score, p2_score, current_emoji, current_answer, status, created_at`

func parseID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}

func scanMovie(row interface{ Scan(...any) error }) (EmojiMovie, error) {
	var m EmojiMovie
	var createdAt time.Time
	err := row.Scan(&m.ID, &m.EmojiStr, &m.Answer, &m.Difficulty, &createdAt)
	if err != nil {
		return m, err
	}
	m.CreatedAt = createdAt.Format(time.RFC3339)
	return m, nil
}

func scanSession(row interface{ Scan(...any) error }) (EmojiGameSession, error) {
	var s EmojiGameSession
	var player2ID sql.NullInt64
	var createdAt time.Time
	err := row.Scan(&s.ID, &s.Player1ID, &player2ID, &s.P1Score, &s.P2Score, &s.CurrentEmoji, &s.CurrentAnswer, &s.Status, &createdAt)
	if err != nil {
		return s, err
	}
	if player2ID.Valid {
		s.Player2ID = &player2ID.Int64
	}
	s.CreatedAt = createdAt.Format(time.RFC3339)
	return s, nil
}

// pickRandomMovie returns a random puzzle, optionally filtered by
// difficulty ("" means any).
func pickRandomMovie(db *sql.DB, difficulty string) (EmojiMovie, error) {
	query := `SELECT id, emoji_str, answer, difficulty, created_at FROM emoji_movies`
	args := []any{}
	if difficulty != "" {
		query += ` WHERE difficulty = $1`
		args = append(args, difficulty)
	}
	query += ` ORDER BY random() LIMIT 1`
	return scanMovie(db.QueryRow(query, args...))
}

// isPlayer reports whether userID is one of the two players in s.
func isPlayer(s EmojiGameSession, userID int64) bool {
	return userID == s.Player1ID || (s.Player2ID != nil && userID == *s.Player2ID)
}
