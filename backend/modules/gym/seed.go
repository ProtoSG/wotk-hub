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

	"github.com/lib/pq"
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

// descriptionsCSV holds the Spanish how-to text, keyed by exercise name. It is
// a separate file from the catalog so the imported source data and the text
// written for this app stay independently replaceable.
//
//go:embed data/descriptions.csv
var descriptionsCSV []byte

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

	if err := seedTrackingTypes(db); err != nil {
		return err
	}
	return seedDescriptions(db)
}

// isometricHolds are the seeded exercises measured by how long the position is
// held. They can't be derived from the CSV the way cardio can (primary_muscle
// = "Cardio"), so they are listed by name.
var isometricHolds = []string{
	"Dead Hang",
	"Front Lever Hold",
	"Handstand Hold",
	"Hollow Rock",
	"L-Sit Hold",
	"Plank",
	"Reverse Plank",
	"Side Plank",
	"Wall Sit",
}

// seedTrackingTypes classifies how each seeded exercise is logged: cardio by
// distance and time, isometric holds by time alone, everything else by weight
// and reps.
//
// This runs only while every row still carries the default, which makes it a
// one-time backfill for databases created before the column existed. After
// that the classification is the user's to change — re-running it on every
// boot would undo any correction they make.
func seedTrackingTypes(db *sql.DB) error {
	var classified bool
	err := db.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM exercises WHERE tracking_type <> $1)`, TrackingWeightReps,
	).Scan(&classified)
	if err != nil {
		return err
	}
	if classified {
		return nil
	}

	res, err := db.Exec(
		`UPDATE exercises SET tracking_type = $1 WHERE primary_muscle = 'Cardio' AND is_custom = false`,
		TrackingDurationDistance,
	)
	if err != nil {
		return err
	}
	cardio, _ := res.RowsAffected()

	res, err = db.Exec(
		`UPDATE exercises SET tracking_type = $1 WHERE is_custom = false AND name = ANY($2)`,
		TrackingDuration, pq.Array(isometricHolds),
	)
	if err != nil {
		return err
	}
	holds, _ := res.RowsAffected()

	log.Printf("gym: tracking types seeded (%d cardio, %d holds)", cardio, holds)
	return nil
}

// seedDescriptions fills in the how-to text for exercises that don't have one.
//
// Only empty descriptions are written, so text edited
// in the app is never overwritten on the next boot — the same reasoning behind
// the catalog's ON CONFLICT DO NOTHING.
func seedDescriptions(db *sql.DB) error {
	descriptions, err := parseDescriptionsCSV(descriptionsCSV)
	if err != nil {
		return fmt.Errorf("parse descriptions csv: %w", err)
	}
	if len(descriptions) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`UPDATE exercises SET description = $2 WHERE name = $1 AND description = ''`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	var written int64
	for name, description := range descriptions {
		res, err := stmt.Exec(name, description)
		if err != nil {
			return fmt.Errorf("describe exercise %q: %w", name, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			written += n
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("gym: exercise descriptions seeded (%d available, %d written)", len(descriptions), written)
	return nil
}

// parseDescriptionsCSV reads the name,description pairs. Names that no longer
// exist in the catalog are simply never matched by the UPDATE.
func parseDescriptionsCSV(data []byte) (map[string]string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = 2

	if _, err := r.Read(); err != nil {
		if err == io.EOF {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read header: %w", err)
	}

	descriptions := map[string]string{}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimSpace(rec[0])
		description := strings.TrimSpace(rec[1])
		if name == "" || description == "" {
			continue
		}
		descriptions[name] = description
	}
	return descriptions, nil
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
