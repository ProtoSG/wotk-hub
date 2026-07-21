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
