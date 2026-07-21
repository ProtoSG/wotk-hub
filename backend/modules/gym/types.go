package gym

import (
	"fmt"
	"time"
)

// dateLayout is the wire format for every date this module accepts or emits.
const dateLayout = "2006-01-02"

// maxSetsPerExercise bounds a single bulk replace. Far above any real workout,
// low enough that a malformed client can't insert an unbounded batch.
const maxSetsPerExercise = 100

// Exercise is a catalog entry. Seeded rows (is_custom = false) come from the
// bundled CSV; user-created ones are flagged is_custom = true so the seeder
// never touches them.
type Exercise struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Equipment       string `json:"equipment"`
	PrimaryMuscle   string `json:"primaryMuscle"`
	SecondaryMuscle string `json:"secondaryMuscle"`
	MediaURL        string `json:"mediaUrl"`
	MediaType       string `json:"mediaType"`
	IsCustom        bool   `json:"isCustom"`
}

type listExercisesResponse struct {
	Exercises []Exercise `json:"exercises"`
	Total     int64      `json:"total"`
}

// exerciseFiltersResponse feeds the picker's dropdowns with the values that
// actually exist in the catalog, instead of a hardcoded frontend list that
// would drift as custom exercises are added.
type exerciseFiltersResponse struct {
	Muscles   []string `json:"muscles"`
	Equipment []string `json:"equipment"`
}

// ExerciseSet is one logged set. Weight is in grams for the same reason
// finances stores cents — 2.5 kg increments stay exact with no float drift.
type ExerciseSet struct {
	ID          int64 `json:"id"`
	SetNumber   int   `json:"setNumber"`
	Reps        int   `json:"reps"`
	WeightGrams int64 `json:"weightGrams"`
	IsWarmup    bool  `json:"isWarmup"`
	Completed   bool  `json:"completed"`
}

// SessionExercise is one exercise performed in a session, with the catalog
// entry inlined so the client doesn't have to join it back itself.
type SessionExercise struct {
	ID         int64         `json:"id"`
	ExerciseID int64         `json:"exerciseId"`
	Position   int           `json:"position"`
	Notes      string        `json:"notes"`
	Exercise   Exercise      `json:"exercise"`
	Sets       []ExerciseSet `json:"sets"`
}

// Session is a training day with its full contents.
type Session struct {
	ID         int64             `json:"id"`
	RoutineID  *int64            `json:"routineId"`
	Name       string            `json:"name"`
	OccurredOn string            `json:"occurredOn"`
	StartedAt  string            `json:"startedAt"`
	FinishedAt *string           `json:"finishedAt"`
	Notes      string            `json:"notes"`
	Exercises  []SessionExercise `json:"exercises"`
}

// SessionSummary is the history-list shape: totals instead of nested contents,
// aggregated in SQL so the list costs one query.
type SessionSummary struct {
	ID               int64   `json:"id"`
	RoutineID        *int64  `json:"routineId"`
	Name             string  `json:"name"`
	OccurredOn       string  `json:"occurredOn"`
	StartedAt        string  `json:"startedAt"`
	FinishedAt       *string `json:"finishedAt"`
	Notes            string  `json:"notes"`
	ExerciseCount    int     `json:"exerciseCount"`
	TotalReps        int64   `json:"totalReps"`
	TotalVolumeGrams int64   `json:"totalVolumeGrams"`
}

type listSessionsResponse struct {
	Sessions []SessionSummary `json:"sessions"`
}

// activeSessionResponse carries a null session when nothing is in progress —
// "no active workout" is an expected state, not a 404.
type activeSessionResponse struct {
	Session *Session `json:"session"`
}

type lastSetsResponse struct {
	Sets []ExerciseSet `json:"sets"`
	// Date of the session the sets came from. Empty when there are none.
	OccurredOn string `json:"occurredOn,omitempty"`
}

type sessionRequest struct {
	Name       string `json:"name"`
	OccurredOn string `json:"occurredOn"`
	Notes      string `json:"notes"`
}

func (r sessionRequest) validate() error {
	if _, err := time.Parse(dateLayout, r.OccurredOn); err != nil {
		return fmt.Errorf("occurredOn must be YYYY-MM-DD")
	}
	return nil
}

type sessionExerciseRequest struct {
	ExerciseID int64  `json:"exerciseId"`
	Notes      string `json:"notes"`
}

type setInput struct {
	Reps        int   `json:"reps"`
	WeightGrams int64 `json:"weightGrams"`
	IsWarmup    bool  `json:"isWarmup"`
	Completed   bool  `json:"completed"`
}

type replaceSetsRequest struct {
	Sets []setInput `json:"sets"`
}

// validate mirrors the CHECK constraints on exercise_sets so a bad payload
// fails with a useful message instead of a Postgres error string.
func (r replaceSetsRequest) validate() error {
	if len(r.Sets) > maxSetsPerExercise {
		return fmt.Errorf("too many sets (max %d)", maxSetsPerExercise)
	}
	for i, s := range r.Sets {
		if s.Reps < 0 {
			return fmt.Errorf("set %d: reps cannot be negative", i+1)
		}
		if s.WeightGrams < 0 {
			return fmt.Errorf("set %d: weight cannot be negative", i+1)
		}
	}
	return nil
}
