package gym

import (
	"database/sql"
	"log"
	"net/http"
	"workhub/httpx"
	"workhub/middleware"
)

// ListRoutines returns the templates with their exercise counts, so the list
// screen doesn't have to load every template's contents.
//
// @Summary List routines
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Success 200 {object} listRoutinesResponse
// @Router /gym/routines [get]
func (h *handler) ListRoutines(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT r.id, r.name, r.notes, r.color, r.icon, r.archived, COUNT(re.id)
		FROM routines r
		LEFT JOIN routine_exercises re ON re.routine_id = r.id
		WHERE r.archived = false
		GROUP BY r.id
		ORDER BY r.name`)
	if err != nil {
		log.Printf("gym: list routines failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	routines := []RoutineSummary{}
	for rows.Next() {
		var rt RoutineSummary
		if err := rows.Scan(&rt.ID, &rt.Name, &rt.Notes, &rt.Color, &rt.Icon, &rt.Archived, &rt.ExerciseCount); err != nil {
			log.Printf("gym: scan routine failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		routines = append(routines, rt)
	}
	if err := rows.Err(); err != nil {
		log.Printf("gym: iterate routines failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, listRoutinesResponse{Routines: routines})
}

// GetRoutine returns a template with its ordered exercises.
//
// @Summary Get a routine
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Param id path int true "Routine ID"
// @Success 200 {object} Routine
// @Failure 404 {object} httpx.APIError
// @Router /gym/routines/{id} [get]
func (h *handler) GetRoutine(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	routine, err := h.loadRoutine(id)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "routine not found")
		return
	}
	if err != nil {
		log.Printf("gym: get routine failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, routine)
}

// CreateRoutine writes a template and its exercise list in one transaction.
//
// @Summary Create a routine
// @Tags gym
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body routineRequest true "Routine details"
// @Success 201 {object} Routine
// @Failure 400 {object} httpx.APIError
// @Router /gym/routines [post]
func (h *handler) CreateRoutine(w http.ResponseWriter, r *http.Request) {
	var req routineRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	userID, _, _ := middleware.UserFromContext(r.Context())

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("gym: create routine begin failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	var id int64
	if err := tx.QueryRow(
		`INSERT INTO routines (name, notes, color, icon, created_by) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		req.Name, req.Notes, defaultTo(req.Color, defaultRoutineColor), defaultTo(req.Icon, defaultRoutineIcon), userID,
	).Scan(&id); err != nil {
		log.Printf("gym: create routine failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := insertRoutineExercises(tx, id, req.Exercises); err != nil {
		log.Printf("gym: create routine exercises failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("gym: create routine commit failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	h.writeRoutine(w, id, http.StatusCreated)
}

// UpdateRoutine replaces the routine's fields and its whole exercise list.
//
// Full replace instead of diffing: the builder edits the list as a unit, and
// re-inserting keeps `position` contiguous without reconciling adds, removes
// and reorders separately. Sessions already logged are unaffected — they hold
// their own copies of the exercises (see workout_sessions).
//
// @Summary Update a routine
// @Tags gym
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path int true "Routine ID"
// @Param body body routineRequest true "Routine details"
// @Success 200 {object} Routine
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /gym/routines/{id} [put]
func (h *handler) UpdateRoutine(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	var req routineRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("gym: update routine begin failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`UPDATE routines SET name = $1, notes = $2, color = $3, icon = $4 WHERE id = $5`,
		req.Name, req.Notes, defaultTo(req.Color, defaultRoutineColor), defaultTo(req.Icon, defaultRoutineIcon), id,
	)
	if err != nil {
		log.Printf("gym: update routine failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "routine not found")
		return
	}

	if _, err := tx.Exec(`DELETE FROM routine_exercises WHERE routine_id = $1`, id); err != nil {
		log.Printf("gym: clear routine exercises failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if err := insertRoutineExercises(tx, id, req.Exercises); err != nil {
		log.Printf("gym: update routine exercises failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("gym: update routine commit failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	h.writeRoutine(w, id, http.StatusOK)
}

// DeleteRoutine removes a template. Its routine_exercises cascade; sessions
// started from it keep their contents and their snapshotted name, and only
// lose the back-reference (routine_id ON DELETE SET NULL).
//
// @Summary Delete a routine
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Param id path int true "Routine ID"
// @Success 200 {object} httpx.SuccessResponse
// @Failure 404 {object} httpx.APIError
// @Router /gym/routines/{id} [delete]
func (h *handler) DeleteRoutine(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	res, err := h.db.Exec(`DELETE FROM routines WHERE id = $1`, id)
	if err != nil {
		log.Printf("gym: delete routine failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "routine not found")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}

// insertRoutineExercises writes the ordered list. position comes from the
// payload order, so the client never sends explicit positions and the
// UNIQUE (routine_id, position) constraint can't be violated by a gap.
func insertRoutineExercises(tx *sql.Tx, routineID int64, exercises []routineExerciseInput) error {
	for i, e := range exercises {
		if _, err := tx.Exec(
			`INSERT INTO routine_exercises (routine_id, exercise_id, position, target_sets, target_reps, notes)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			routineID, e.ExerciseID, i+1, e.TargetSets, e.TargetReps, e.Notes,
		); err != nil {
			return err
		}
	}
	return nil
}

func (h *handler) loadRoutine(id int64) (*Routine, error) {
	var rt Routine
	err := h.db.QueryRow(
		`SELECT id, name, notes, color, icon, archived FROM routines WHERE id = $1`, id,
	).Scan(&rt.ID, &rt.Name, &rt.Notes, &rt.Color, &rt.Icon, &rt.Archived)
	if err != nil {
		return nil, err
	}

	rows, err := h.db.Query(`
		SELECT re.id, re.exercise_id, re.position, re.target_sets, re.target_reps, re.notes,
		       e.name, e.equipment, e.primary_muscle, e.secondary_muscle, e.description, e.tracking_type, e.media_url, e.media_type, e.is_custom
		FROM routine_exercises re
		JOIN exercises e ON e.id = re.exercise_id
		WHERE re.routine_id = $1
		ORDER BY re.position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rt.Exercises = []RoutineExercise{}
	for rows.Next() {
		var re RoutineExercise
		if err := rows.Scan(
			&re.ID, &re.ExerciseID, &re.Position, &re.TargetSets, &re.TargetReps, &re.Notes,
			&re.Exercise.Name, &re.Exercise.Equipment, &re.Exercise.PrimaryMuscle,
			&re.Exercise.SecondaryMuscle, &re.Exercise.Description, &re.Exercise.TrackingType,
			&re.Exercise.MediaURL, &re.Exercise.MediaType, &re.Exercise.IsCustom,
		); err != nil {
			return nil, err
		}
		re.Exercise.ID = re.ExerciseID
		rt.Exercises = append(rt.Exercises, re)
	}
	return &rt, rows.Err()
}

func (h *handler) writeRoutine(w http.ResponseWriter, id int64, status int) {
	routine, err := h.loadRoutine(id)
	if err != nil {
		log.Printf("gym: load routine failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, status, routine)
}
