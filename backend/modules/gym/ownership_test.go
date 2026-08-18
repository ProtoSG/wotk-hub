package gym

import (
	"net/http"
	"strconv"
	"testing"
)

// Gym had zero created_by scoping before this slice: exercises/routines/
// workout_sessions all stamped created_by on insert but no query anywhere
// filtered by it (per the full-schema audit) — any authenticated user could
// read/edit/delete any other user's routine or session. These tests cover
// the boundary now enforced via scopeToOwner (see helpers.go).

func TestRoutines_GuestCannotSeeOrMutateAnotherUsersRoutine(t *testing.T) {
	db := setupTestDB(t)
	resetGymTables(t, db)
	seedGuestUser(t, db)

	w := doAs(t, db, 1, "admin", http.MethodPost, "/routines", map[string]any{
		"name": "Empuje", "notes": "", "color": "#fff", "icon": "dumbbell", "exercises": []any{},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("admin create routine: %d %s", w.Code, w.Body.String())
	}
	var created Routine
	mustDecode(t, w, &created)

	// Guest's own list doesn't include admin's routine.
	w = doAs(t, db, 2, "guest", http.MethodGet, "/routines", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("guest list routines: %d %s", w.Code, w.Body.String())
	}
	var listResp listRoutinesResponse
	mustDecode(t, w, &listResp)
	for _, rt := range listResp.Routines {
		if rt.ID == created.ID {
			t.Fatalf("guest saw admin's routine: %+v", rt)
		}
	}

	// Guest can't read, update, or delete it directly by id either.
	w = doAs(t, db, 2, "guest", http.MethodGet, "/routines/"+strconv.Itoa(int(created.ID)), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("guest get admin's routine: status = %d, want 404 (body %s)", w.Code, w.Body.String())
	}
	w = doAs(t, db, 2, "guest", http.MethodDelete, "/routines/"+strconv.Itoa(int(created.ID)), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("guest delete admin's routine: status = %d, want 404 (body %s)", w.Code, w.Body.String())
	}

	// Admin's routine is untouched.
	w = doAs(t, db, 1, "admin", http.MethodGet, "/routines/"+strconv.Itoa(int(created.ID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("admin's routine was affected by the guest's requests: %d %s", w.Code, w.Body.String())
	}
}

func TestSessions_GuestCannotSeeOrMutateAnotherUsersSession(t *testing.T) {
	db := setupTestDB(t)
	resetGymTables(t, db)
	seedGuestUser(t, db)

	w := doAs(t, db, 1, "admin", http.MethodPost, "/sessions", map[string]any{
		"name": "", "occurredOn": "2026-08-01", "notes": "",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("admin create session: %d %s", w.Code, w.Body.String())
	}
	var created Session
	mustDecode(t, w, &created)

	// Guest's list is empty — this session isn't theirs.
	w = doAs(t, db, 2, "guest", http.MethodGet, "/sessions", nil)
	var listResp listSessionsResponse
	mustDecode(t, w, &listResp)
	if len(listResp.Sessions) != 0 {
		t.Fatalf("guest saw another user's sessions: %+v", listResp.Sessions)
	}

	// Guest doesn't see it as "active" either, even though it's in progress.
	w = doAs(t, db, 2, "guest", http.MethodGet, "/sessions/active", nil)
	var activeResp activeSessionResponse
	mustDecode(t, w, &activeResp)
	if activeResp.Session != nil {
		t.Fatalf("guest saw admin's in-progress session as their own active one: %+v", activeResp.Session)
	}

	// Guest can start their OWN session concurrently — the "one workout at a
	// time" conflict check must be per-user, not app-wide.
	w = doAs(t, db, 2, "guest", http.MethodPost, "/sessions", map[string]any{
		"name": "", "occurredOn": "2026-08-01", "notes": "",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("guest create own session while admin's is in progress: %d %s", w.Code, w.Body.String())
	}

	// Guest can't finish or delete admin's session.
	w = doAs(t, db, 2, "guest", http.MethodPost, "/sessions/"+strconv.Itoa(int(created.ID))+"/finish", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("guest finish admin's session: status = %d, want 404 (body %s)", w.Code, w.Body.String())
	}
	w = doAs(t, db, 2, "guest", http.MethodDelete, "/sessions/"+strconv.Itoa(int(created.ID)), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("guest delete admin's session: status = %d, want 404 (body %s)", w.Code, w.Body.String())
	}
}

func TestExercises_CustomExerciseOwnershipEnforced(t *testing.T) {
	db := setupTestDB(t)
	resetGymTables(t, db)
	seedGuestUser(t, db)

	w := doAs(t, db, 1, "admin", http.MethodPost, "/exercises", map[string]any{
		"name": "Admin's custom lift", "primaryMuscle": "pecho", "equipment": "", "secondaryMuscle": "",
		"description": "", "trackingType": "weight_reps",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("admin create custom exercise: %d %s", w.Code, w.Body.String())
	}
	var created Exercise
	mustDecode(t, w, &created)

	// Both users can still see it (shared catalog, read is unscoped).
	w = doAs(t, db, 2, "guest", http.MethodGet, "/exercises", nil)
	var listResp listExercisesResponse
	mustDecode(t, w, &listResp)
	found := false
	for _, e := range listResp.Exercises {
		if e.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("guest didn't see admin's custom exercise in the shared catalog")
	}

	// But guest can't edit or delete admin's custom exercise.
	w = doAs(t, db, 2, "guest", http.MethodPut, "/exercises/"+strconv.Itoa(int(created.ID)), map[string]any{
		"name": "renamed", "primaryMuscle": "pecho", "equipment": "", "secondaryMuscle": "",
		"description": "", "trackingType": "weight_reps",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("guest update admin's custom exercise: status = %d, want 404 (body %s)", w.Code, w.Body.String())
	}
	w = doAs(t, db, 2, "guest", http.MethodDelete, "/exercises/"+strconv.Itoa(int(created.ID)), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("guest delete admin's custom exercise: status = %d, want 404 (body %s)", w.Code, w.Body.String())
	}
}
