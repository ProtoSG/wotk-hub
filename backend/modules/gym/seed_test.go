package gym

import "testing"

// The bundled CSV is the real fixture here: it is the file that actually
// ships, and its quirks (quoted comma-lists, "None" sentinels) are exactly
// what the parser exists to handle.
func TestParseExercisesCSVBundled(t *testing.T) {
	exercises, err := parseExercisesCSV(exercisesCSV)
	if err != nil {
		t.Fatalf("parse bundled csv: %v", err)
	}
	if len(exercises) != 413 {
		t.Fatalf("parsed %d exercises, want 413", len(exercises))
	}

	seen := map[string]bool{}
	for _, e := range exercises {
		if e.Name == "" {
			t.Fatal("parsed an exercise with an empty name")
		}
		if seen[e.Name] {
			t.Fatalf("duplicate exercise name %q — name is the seeder's conflict key", e.Name)
		}
		seen[e.Name] = true

		if e.PrimaryMuscle == "" {
			t.Fatalf("%q has an empty primary muscle", e.Name)
		}
		// "None" is the CSV's null sentinel and must never reach the DB.
		if e.Equipment == nullSentinel || e.SecondaryMuscle == nullSentinel ||
			e.MediaURL == nullSentinel || e.MediaType == nullSentinel {
			t.Fatalf("%q kept a %q sentinel: %+v", e.Name, nullSentinel, e)
		}
	}
}

func TestParseExercisesCSVFields(t *testing.T) {
	// Row 2 carries a quoted comma-list in secondary_muscle — the case that
	// breaks a naive strings.Split on the raw line.
	csv := []byte(`name,equipment,primary_muscle,secondary_muscle,source,sourceType
21s Bicep Curl,Barbell,Biceps,None,None,None
Bench Press (Barbell),Barbell,Chest,"Triceps, Shoulders",https://example.com/bench.mp4,video
`)

	exercises, err := parseExercisesCSV(csv)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(exercises) != 2 {
		t.Fatalf("parsed %d exercises, want 2", len(exercises))
	}

	curl := exercises[0]
	if curl.Name != "21s Bicep Curl" {
		t.Errorf("name = %q, want %q", curl.Name, "21s Bicep Curl")
	}
	if curl.SecondaryMuscle != "" || curl.MediaURL != "" || curl.MediaType != "" {
		t.Errorf("None sentinels not normalized: %+v", curl)
	}

	bench := exercises[1]
	if bench.SecondaryMuscle != "Triceps, Shoulders" {
		t.Errorf("secondary muscle = %q, want %q", bench.SecondaryMuscle, "Triceps, Shoulders")
	}
	if bench.MediaType != "video" {
		t.Errorf("media type = %q, want %q", bench.MediaType, "video")
	}
}

// A row missing primary_muscle falls back to "Other" rather than violating the
// column's NOT NULL DEFAULT contract with an empty string.
func TestParseExercisesCSVDefaultsPrimaryMuscle(t *testing.T) {
	csv := []byte(`name,equipment,primary_muscle,secondary_muscle,source,sourceType
Mystery Move,Barbell,,None,None,None
`)
	exercises, err := parseExercisesCSV(csv)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := exercises[0].PrimaryMuscle; got != "Other" {
		t.Errorf("primary muscle = %q, want %q", got, "Other")
	}
}

func TestParseExercisesCSVRejectsWrongColumnCount(t *testing.T) {
	csv := []byte(`name,equipment,primary_muscle,secondary_muscle,source,sourceType
Broken Row,Barbell,Biceps
`)
	if _, err := parseExercisesCSV(csv); err == nil {
		t.Fatal("expected an error on a short row, got nil")
	}
}
