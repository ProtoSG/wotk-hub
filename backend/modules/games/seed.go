package games

import "database/sql"

type seedMovie struct {
	emojiStr   string
	answer     string
	difficulty string
}

// Answers are in Spanish (LatAm release titles), matching the app's UI
// language — see games.Guess, which compares the guess against this field
// case-insensitively.
var seedMovies = []seedMovie{
	{"🦇🧛‍♂️🌃", "Batman", "easy"},
	{"🚢🧊💔", "Titanic", "easy"},
	{"🦁👑🌍", "El Rey León", "easy"},
	{"🤠🚀🧸", "Toy Story", "easy"},
	{"❄️👸⛄", "Frozen", "easy"},
	{"⭐⚔️🚀", "Star Wars", "easy"},
	{"🐠🌊🔍", "Buscando a Nemo", "easy"},
	{"🟢🧅👹", "Shrek", "easy"},
	{"🏎️🏁🔧", "Cars", "easy"},
	{"🎈🏠👴", "Up", "easy"},
	{"🦈🌊🚤", "Tiburón", "easy"},
	{"👽🚲🌕", "E.T. el Extraterrestre", "easy"},
	{"🕷️🕸️🧑", "El Hombre Araña", "medium"},
	{"⚡🧙‍♂️🦉", "Harry Potter", "medium"},
	{"🦖🏝️🧬", "Parque Jurásico", "medium"},
	{"💊🕶️🖥️", "Matrix", "medium"},
	{"💙🌳👽", "Avatar", "medium"},
	{"🏴‍☠️⚓🦜", "Piratas del Caribe", "medium"},
	{"🌀🛌🎩", "Origen", "medium"},
	{"🦇🃏🌃", "Batman: El Caballero de la Noche", "medium"},
	{"🦸‍♂️🦸‍♀️🛡️", "Los Vengadores", "medium"},
	{"🐆👑🌍", "Pantera Negra", "medium"},
	{"🚗⚡⏰", "Volver al Futuro", "medium"},
	{"⚔️🛡️🏛️", "Gladiador", "medium"},
	{"🐴🔫🍝", "El Padrino", "hard"},
	{"🌌🚀⏳", "Interstellar", "hard"},
	{"🦸‍♀️🛡️⚡", "Mujer Maravilla", "hard"},
	{"⛓️🕳️📜", "Sueño de Fuga", "hard"},
	{"🥊🧼😵", "El Club de la Pelea", "hard"},
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
