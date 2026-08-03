package games

import "database/sql"

type seedMovie struct {
	emojiStr   string
	answer     string
	difficulty string
}

var seedMovies = []seedMovie{
	{"🦇🧛‍♂️🌃", "Batman", "easy"},
	{"🚢🧊💔", "Titanic", "easy"},
	{"🦁👑🌍", "The Lion King", "easy"},
	{"🤠🚀🧸", "Toy Story", "easy"},
	{"❄️👸⛄", "Frozen", "easy"},
	{"⭐⚔️🚀", "Star Wars", "easy"},
	{"🐠🌊🔍", "Finding Nemo", "easy"},
	{"🟢🧅👹", "Shrek", "easy"},
	{"🏎️🏁🔧", "Cars", "easy"},
	{"🎈🏠👴", "Up", "easy"},
	{"🦈🌊🚤", "Jaws", "easy"},
	{"👽🚲🌕", "E.T.", "easy"},
	{"🕷️🕸️🧑", "Spider-Man", "medium"},
	{"⚡🧙‍♂️🦉", "Harry Potter", "medium"},
	{"🦖🏝️🧬", "Jurassic Park", "medium"},
	{"💊🕶️🖥️", "The Matrix", "medium"},
	{"💙🌳👽", "Avatar", "medium"},
	{"🏴‍☠️⚓🦜", "Pirates of the Caribbean", "medium"},
	{"🌀🛌🎩", "Inception", "medium"},
	{"🦇🃏🌃", "The Dark Knight", "medium"},
	{"🦸‍♂️🦸‍♀️🛡️", "Avengers", "medium"},
	{"🐆👑🌍", "Black Panther", "medium"},
	{"🚗⚡⏰", "Back to the Future", "medium"},
	{"⚔️🛡️🏛️", "Gladiator", "medium"},
	{"🐴🔫🍝", "The Godfather", "hard"},
	{"🌌🚀⏳", "Interstellar", "hard"},
	{"🦸‍♀️🛡️⚡", "Wonder Woman", "hard"},
	{"⛓️🕳️📜", "The Shawshank Redemption", "hard"},
	{"🥊🧼😵", "Fight Club", "hard"},
	{"🏃‍♂️🍫🎗️", "Forrest Gump", "hard"},
}

// Seed populates emoji_movies with the starter puzzle set on first run
// only — if the table already has any rows, it's a no-op so replays of
// this call (every backend startup) don't create duplicates.
func Seed(db *sql.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM emoji_movies`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	for _, m := range seedMovies {
		if _, err := db.Exec(
			`INSERT INTO emoji_movies (emoji_str, answer, difficulty) VALUES ($1, $2, $3)`,
			m.emojiStr, m.answer, m.difficulty,
		); err != nil {
			return err
		}
	}
	return nil
}
