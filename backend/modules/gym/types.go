package gym

import (
	"fmt"
	"strings"
	"time"
)

// dateLayout is the wire format for every date this module accepts or emits.
const dateLayout = "2006-01-02"

// maxSetsPerExercise bounds a single bulk replace. Far above any real workout,
// low enough that a malformed client can't insert an unbounded batch.
const maxSetsPerExercise = 100

// maxExercisesPerRoutine bounds a template for the same reason.
const maxExercisesPerRoutine = 50

// Defaults mirroring the routines table, applied when a client omits them.
const (
	defaultRoutineColor = "#3B82F6"
	defaultRoutineIcon  = "dumbbell"
)

// Exercise is a catalog entry. Seeded rows (is_custom = false) come from the
// bundled CSV; user-created ones are flagged is_custom = true so the seeder
// never touches them.
type Exercise struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Equipment       string `json:"equipment"`
	PrimaryMuscle   string `json:"primaryMuscle"`
	SecondaryMuscle string `json:"secondaryMuscle"`
	// Spanish how-to text; empty until seeded or written in the app.
	Description string `json:"description"`
	MediaURL    string `json:"mediaUrl"`
	MediaType   string `json:"mediaType"`
	IsCustom    bool   `json:"isCustom"`
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

// maxExerciseNameLength keeps a user-created name to something a list row can
// still show. Descriptions are bounded too, since the request body limit alone
// would allow a megabyte of text in one field.
const (
	maxExerciseNameLength        = 120
	maxExerciseDescriptionLength = 2000
)

type exerciseRequest struct {
	Name            string `json:"name"`
	Equipment       string `json:"equipment"`
	PrimaryMuscle   string `json:"primaryMuscle"`
	SecondaryMuscle string `json:"secondaryMuscle"`
	Description     string `json:"description"`
}

func (r exerciseRequest) validate() error {
	name := strings.TrimSpace(r.Name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if len([]rune(name)) > maxExerciseNameLength {
		return fmt.Errorf("name is too long (max %d characters)", maxExerciseNameLength)
	}
	if strings.TrimSpace(r.PrimaryMuscle) == "" {
		return fmt.Errorf("primaryMuscle is required")
	}
	if len([]rune(r.Description)) > maxExerciseDescriptionLength {
		return fmt.Errorf("description is too long (max %d characters)", maxExerciseDescriptionLength)
	}
	return nil
}

// descriptionRequest edits only the how-to text. Seeded exercises accept this
// but not a full update: their name and muscles come from the imported
// catalog, and rewriting them would silently fork it.
type descriptionRequest struct {
	Description string `json:"description"`
}

func (r descriptionRequest) validate() error {
	if len([]rune(r.Description)) > maxExerciseDescriptionLength {
		return fmt.Errorf("description is too long (max %d characters)", maxExerciseDescriptionLength)
	}
	return nil
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

// RoutineExercise is one entry of a template, with its catalog data inlined.
type RoutineExercise struct {
	ID         int64    `json:"id"`
	ExerciseID int64    `json:"exerciseId"`
	Position   int      `json:"position"`
	TargetSets int      `json:"targetSets"`
	TargetReps int      `json:"targetReps"`
	Notes      string   `json:"notes"`
	Exercise   Exercise `json:"exercise"`
}

type Routine struct {
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	Notes     string            `json:"notes"`
	Color     string            `json:"color"`
	Icon      string            `json:"icon"`
	Archived  bool              `json:"archived"`
	Exercises []RoutineExercise `json:"exercises"`
}

// RoutineSummary is the list shape: the count instead of the contents.
type RoutineSummary struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Notes         string `json:"notes"`
	Color         string `json:"color"`
	Icon          string `json:"icon"`
	Archived      bool   `json:"archived"`
	ExerciseCount int    `json:"exerciseCount"`
}

type listRoutinesResponse struct {
	Routines []RoutineSummary `json:"routines"`
}

type routineExerciseInput struct {
	ExerciseID int64  `json:"exerciseId"`
	TargetSets int    `json:"targetSets"`
	TargetReps int    `json:"targetReps"`
	Notes      string `json:"notes"`
}

type routineRequest struct {
	Name      string                 `json:"name"`
	Notes     string                 `json:"notes"`
	Color     string                 `json:"color"`
	Icon      string                 `json:"icon"`
	Exercises []routineExerciseInput `json:"exercises"`
}

// validate mirrors the routine_exercises CHECK constraints so a bad payload
// fails with a readable message instead of a Postgres error string.
func (r routineRequest) validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if len(r.Exercises) > maxExercisesPerRoutine {
		return fmt.Errorf("too many exercises (max %d)", maxExercisesPerRoutine)
	}
	for i, e := range r.Exercises {
		if e.ExerciseID <= 0 {
			return fmt.Errorf("exercise %d: exerciseId is required", i+1)
		}
		if e.TargetSets <= 0 {
			return fmt.Errorf("exercise %d: targetSets must be greater than 0", i+1)
		}
		if e.TargetReps <= 0 {
			return fmt.Errorf("exercise %d: targetReps must be greater than 0", i+1)
		}
	}
	return nil
}

type sessionRequest struct {
	// Optional: when set, the session is materialized from that template.
	RoutineID  *int64 `json:"routineId"`
	Name       string `json:"name"`
	OccurredOn string `json:"occurredOn"`
	Notes      string `json:"notes"`
}

func (r sessionRequest) validate() error {
	if _, err := time.Parse(dateLayout, r.OccurredOn); err != nil {
		return fmt.Errorf("occurredOn must be YYYY-MM-DD")
	}
	if r.RoutineID != nil && *r.RoutineID <= 0 {
		return fmt.Errorf("routineId must be a positive id")
	}
	return nil
}

// TopSet is the heaviest working set of a session — the one the estimated
// 1RM is derived from.
type TopSet struct {
	Reps        int   `json:"reps"`
	WeightGrams int64 `json:"weightGrams"`
}

// ProgressPoint is one session's worth of an exercise's progress.
type ProgressPoint struct {
	SessionID        int64  `json:"sessionId"`
	OccurredOn       string `json:"occurredOn"`
	MaxWeightGrams   int64  `json:"maxWeightGrams"`
	TotalReps        int64  `json:"totalReps"`
	TotalVolumeGrams int64  `json:"totalVolumeGrams"`
	TopSet           TopSet `json:"topSet"`
	// Epley estimate from the top set: weight x (1 + reps/30).
	Estimated1RMGrams int64 `json:"estimated1rmGrams"`
}

type exerciseProgressResponse struct {
	Points []ProgressPoint `json:"points"`
}

type ProgressSummaryResponse struct {
	SessionsThisMonth    int   `json:"sessionsThisMonth"`
	VolumeThisMonthGrams int64 `json:"volumeThisMonthGrams"`
	// Consecutive weeks with at least one session — weeks, not days, because
	// rest days are part of training.
	WeekStreak int `json:"weekStreak"`
	// Empty when nothing has been logged in the trailing 90 days.
	TopMuscle string `json:"topMuscle"`
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
