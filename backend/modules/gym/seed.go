package gym

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"strings"

	_ "embed"
)

// exercisesCSV is the bundled exercise catalog. Embedded rather than read from
// disk so a deploy can never end up with a running binary and a missing file.
//
//go:embed data/exercises.csv
var exercisesCSV []byte

// csvFieldCount is the column count of data/exercises.csv:
// name,equipment,primary_muscle,secondary_muscle,source,sourceType
const csvFieldCount = 6

// nullSentinel is the literal string the source CSV uses instead of an empty
// field. It is normalized away on import for every column except
// primary_muscle, where it never appears (that column uses "Other").
const nullSentinel = "None"

// SeedExercises imports the bundled catalog into the exercises table.
//
// Idempotent by design: this project has no migration tool (see
// store.Migrate), so it runs on every boot. Rows are matched on name with
// ON CONFLICT DO NOTHING — never DO UPDATE, so a renamed or re-pointed
// exercise is not clobbered on the next deploy, and user-created rows
// (is_custom = true) are never touched.
//
// The whole import runs in one transaction: a partially seeded catalog is
// worse than an empty one.
func SeedExercises(db *sql.DB) error {
	exercises, err := parseExercisesCSV(exercisesCSV)
	if err != nil {
		return fmt.Errorf("parse exercises csv: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO exercises (name, equipment, primary_muscle, secondary_muscle, media_url, media_type, is_custom)
		 VALUES ($1, $2, $3, $4, $5, $6, false)
		 ON CONFLICT (name) DO NOTHING`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	var inserted int64
	for _, e := range exercises {
		res, err := stmt.Exec(e.Name, e.Equipment, e.PrimaryMuscle, e.SecondaryMuscle, e.MediaURL, e.MediaType)
		if err != nil {
			return fmt.Errorf("insert exercise %q: %w", e.Name, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			inserted += n
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("gym: exercise catalog seeded (%d parsed, %d inserted)", len(exercises), inserted)
	return nil
}

// parseExercisesCSV decodes the bundled catalog. It uses encoding/csv rather
// than splitting on commas because secondary_muscle is a quoted comma-list
// ("Quadriceps, Lower Back, Glutes") in ~60 of the rows.
func parseExercisesCSV(data []byte) ([]Exercise, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = csvFieldCount

	// Header.
	if _, err := r.Read(); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	exercises := []Exercise{}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		name := strings.TrimSpace(rec[0])
		if name == "" {
			continue
		}
		exercises = append(exercises, Exercise{
			Name:            name,
			Equipment:       normalizeField(rec[1]),
			PrimaryMuscle:   defaultTo(normalizeField(rec[2]), "Other"),
			SecondaryMuscle: normalizeField(rec[3]),
			MediaURL:        normalizeField(rec[4]),
			MediaType:       normalizeField(rec[5]),
		})
	}
	return exercises, nil
}

// normalizeField trims a CSV field and maps the "None" sentinel to "".
func normalizeField(s string) string {
	s = strings.TrimSpace(s)
	if s == nullSentinel {
		return ""
	}
	return s
}

func defaultTo(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
