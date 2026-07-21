package gym

import (
	"strings"
	"testing"
)

func TestSessionRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     sessionRequest
		wantErr bool
	}{
		{"valid date", sessionRequest{OccurredOn: "2026-07-20"}, false},
		{"empty date", sessionRequest{OccurredOn: ""}, true},
		{"wrong format", sessionRequest{OccurredOn: "20/07/2026"}, true},
		{"not a date", sessionRequest{OccurredOn: "2026-13-45"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSessionRequestValidateRoutineID(t *testing.T) {
	valid := int64(3)
	zero := int64(0)
	negative := int64(-1)

	if err := (sessionRequest{OccurredOn: "2026-07-20", RoutineID: &valid}).validate(); err != nil {
		t.Errorf("valid routineId rejected: %v", err)
	}
	// Freestyle sessions carry no routine at all.
	if err := (sessionRequest{OccurredOn: "2026-07-20"}).validate(); err != nil {
		t.Errorf("absent routineId rejected: %v", err)
	}
	if err := (sessionRequest{OccurredOn: "2026-07-20", RoutineID: &zero}).validate(); err == nil {
		t.Error("zero routineId accepted")
	}
	if err := (sessionRequest{OccurredOn: "2026-07-20", RoutineID: &negative}).validate(); err == nil {
		t.Error("negative routineId accepted")
	}
}

func TestRoutineRequestValidate(t *testing.T) {
	valid := routineRequest{
		Name: "Día de Pecho",
		Exercises: []routineExerciseInput{
			{ExerciseID: 1, TargetSets: 4, TargetReps: 8},
			{ExerciseID: 2, TargetSets: 3, TargetReps: 12},
		},
	}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid routine rejected: %v", err)
	}

	// An empty template is legal — exercises get added to it later.
	if err := (routineRequest{Name: "Nueva"}).validate(); err != nil {
		t.Errorf("empty exercise list rejected: %v", err)
	}

	tests := []struct {
		name string
		req  routineRequest
	}{
		{"blank name", routineRequest{Name: "   "}},
		{"missing exercise id", routineRequest{Name: "X", Exercises: []routineExerciseInput{{TargetSets: 3, TargetReps: 8}}}},
		{"zero sets", routineRequest{Name: "X", Exercises: []routineExerciseInput{{ExerciseID: 1, TargetSets: 0, TargetReps: 8}}}},
		{"zero reps", routineRequest{Name: "X", Exercises: []routineExerciseInput{{ExerciseID: 1, TargetSets: 3, TargetReps: 0}}}},
		{"too many exercises", routineRequest{Name: "X", Exercises: make([]routineExerciseInput, maxExercisesPerRoutine+1)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.validate(); err == nil {
				t.Error("invalid routine accepted")
			}
		})
	}
}

func TestReplaceSetsRequestValidate(t *testing.T) {
	// A bodyweight set (0 g) and a failed set (0 reps) are both legal — the
	// validation only rejects negatives.
	valid := replaceSetsRequest{Sets: []setInput{
		{Reps: 8, WeightGrams: 80000, Completed: true},
		{Reps: 0, WeightGrams: 0, Completed: false},
		{Reps: 12, WeightGrams: 0, IsWarmup: true, Completed: true},
	}}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid sets rejected: %v", err)
	}

	// An empty list is how the UI clears an exercise's sets.
	if err := (replaceSetsRequest{Sets: []setInput{}}).validate(); err != nil {
		t.Fatalf("empty set list rejected: %v", err)
	}

	if err := (replaceSetsRequest{Sets: []setInput{{Reps: -1}}}).validate(); err == nil {
		t.Error("negative reps accepted")
	}
	if err := (replaceSetsRequest{Sets: []setInput{{Reps: 5, WeightGrams: -1}}}).validate(); err == nil {
		t.Error("negative weight accepted")
	}
}

func TestReplaceSetsRequestRejectsOversizedBatch(t *testing.T) {
	sets := make([]setInput, maxSetsPerExercise+1)
	err := (replaceSetsRequest{Sets: sets}).validate()
	if err == nil {
		t.Fatal("oversized batch accepted")
	}
	if !strings.Contains(err.Error(), "too many sets") {
		t.Errorf("error = %q, want it to mention the set limit", err)
	}
}

// The set number is assigned from payload order, so a client that deletes a
// middle row never has to renumber before saving.
func TestReplaceSetsRequestErrorNumbersSetsFromOne(t *testing.T) {
	err := (replaceSetsRequest{Sets: []setInput{{Reps: 5}, {Reps: -3}}}).validate()
	if err == nil {
		t.Fatal("expected an error for the negative second set")
	}
	if !strings.HasPrefix(err.Error(), "set 2:") {
		t.Errorf("error = %q, want it to point at set 2", err)
	}
}

func TestExerciseRequestValidate(t *testing.T) {
	valid := exerciseRequest{Name: "Remo con toalla", PrimaryMuscle: "Upper Back"}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid exercise rejected: %v", err)
	}
	// Equipment, secondary muscle and description are all optional.
	if err := (exerciseRequest{Name: "X", PrimaryMuscle: "Chest"}).validate(); err != nil {
		t.Errorf("minimal exercise rejected: %v", err)
	}

	tests := []struct {
		name string
		req  exerciseRequest
	}{
		{"blank name", exerciseRequest{Name: "   ", PrimaryMuscle: "Chest"}},
		{"missing primary muscle", exerciseRequest{Name: "X"}},
		{"blank primary muscle", exerciseRequest{Name: "X", PrimaryMuscle: " "}},
		{"name too long", exerciseRequest{
			Name:          strings.Repeat("a", maxExerciseNameLength+1),
			PrimaryMuscle: "Chest",
		}},
		{"description too long", exerciseRequest{
			Name:          "X",
			PrimaryMuscle: "Chest",
			Description:   strings.Repeat("a", maxExerciseDescriptionLength+1),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.validate(); err == nil {
				t.Error("invalid exercise accepted")
			}
		})
	}
}

// Length is counted in runes, not bytes: a name of accented characters would
// otherwise be rejected at roughly half the advertised limit.
func TestExerciseRequestNameLengthCountsRunes(t *testing.T) {
	req := exerciseRequest{
		Name:          strings.Repeat("á", maxExerciseNameLength),
		PrimaryMuscle: "Chest",
	}
	if err := req.validate(); err != nil {
		t.Fatalf("name of %d accented runes rejected: %v", maxExerciseNameLength, err)
	}
}

func TestDescriptionRequestValidate(t *testing.T) {
	// Clearing a description is legitimate.
	if err := (descriptionRequest{Description: ""}).validate(); err != nil {
		t.Errorf("empty description rejected: %v", err)
	}
	if err := (descriptionRequest{Description: strings.Repeat("a", maxExerciseDescriptionLength+1)}).validate(); err == nil {
		t.Error("oversized description accepted")
	}
}
