package gym

import (
	"database/sql"
	"log"
	"net/http"
	"workhub/httpx"
)

// AddSessionExercise appends an exercise to a session, at the end of the
// current order.
//
// @Summary Add an exercise to a session
// @Tags gym
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path int true "Session ID"
// @Param body body sessionExerciseRequest true "Exercise to add"
// @Success 201 {object} Session
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /gym/sessions/{id}/exercises [post]
func (h *handler) AddSessionExercise(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parseID(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	var req sessionExerciseRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if req.ExerciseID <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "exerciseId is required")
		return
	}

	if err := h.sessionExists(sessionID); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "session not found")
		return
	} else if err != nil {
		log.Printf("gym: session lookup failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	var exists bool
	if err := h.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM exercises WHERE id = $1)`, req.ExerciseID).Scan(&exists); err != nil {
		log.Printf("gym: exercise lookup failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if !exists {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "exercise not found")
		return
	}

	// COALESCE(MAX(position), 0) + 1 keeps the UNIQUE (session_id, position)
	// constraint satisfied without reading the list first.
	if _, err := h.db.Exec(
		`INSERT INTO session_exercises (session_id, exercise_id, position, notes)
		 VALUES ($1, $2, (SELECT COALESCE(MAX(position), 0) + 1 FROM session_exercises WHERE session_id = $1), $3)`,
		sessionID, req.ExerciseID, req.Notes,
	); err != nil {
		log.Printf("gym: add session exercise failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	h.writeSession(w, sessionID, http.StatusCreated)
}

// RemoveSessionExercise drops an exercise and its sets, then closes the gap in
// position so the remaining order stays contiguous.
//
// @Summary Remove an exercise from a session
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Param id path int true "Session ID"
// @Param exerciseId path int true "Session exercise ID"
// @Success 200 {object} Session
// @Failure 404 {object} httpx.APIError
// @Router /gym/sessions/{id}/exercises/{exerciseId} [delete]
func (h *handler) RemoveSessionExercise(w http.ResponseWriter, r *http.Request) {
	sessionID, sessionExerciseID, err := parseSessionExerciseIDs(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("gym: remove session exercise begin failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	var position int
	err = tx.QueryRow(
		`DELETE FROM session_exercises WHERE id = $1 AND session_id = $2 RETURNING position`,
		sessionExerciseID, sessionID,
	).Scan(&position)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "exercise not found in session")
		return
	}
	if err != nil {
		log.Printf("gym: remove session exercise failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if _, err := tx.Exec(
		`UPDATE session_exercises SET position = position - 1 WHERE session_id = $1 AND position > $2`,
		sessionID, position,
	); err != nil {
		log.Printf("gym: repack positions failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("gym: remove session exercise commit failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	h.writeSession(w, sessionID, http.StatusOK)
}

// ReplaceSets swaps an exercise's whole set list for the submitted one.
//
// Bulk replace rather than per-set writes: the logging UI edits a small grid
// locally and saves the block, so this is one round trip per exercise instead
// of one per set — which matters mid-workout, exactly when the connection is
// worst. set_number is assigned from the payload order, so the client never
// has to renumber after removing a row.
//
// @Summary Replace an exercise's sets
// @Tags gym
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path int true "Session ID"
// @Param exerciseId path int true "Session exercise ID"
// @Param body body replaceSetsRequest true "The full set list"
// @Success 200 {object} Session
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /gym/sessions/{id}/exercises/{exerciseId}/sets [put]
func (h *handler) ReplaceSets(w http.ResponseWriter, r *http.Request) {
	sessionID, sessionExerciseID, err := parseSessionExerciseIDs(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	var req replaceSetsRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	// One transaction: a failed replace must leave the previous set list
	// exactly as it was, never half-written.
	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("gym: replace sets begin failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM session_exercises WHERE id = $1 AND session_id = $2)`,
		sessionExerciseID, sessionID,
	).Scan(&exists); err != nil {
		log.Printf("gym: session exercise lookup failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if !exists {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "exercise not found in session")
		return
	}

	if _, err := tx.Exec(`DELETE FROM exercise_sets WHERE session_exercise_id = $1`, sessionExerciseID); err != nil {
		log.Printf("gym: clear sets failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	for i, set := range req.Sets {
		if _, err := tx.Exec(
			`INSERT INTO exercise_sets (session_exercise_id, set_number, reps, weight_grams, is_warmup, completed)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			sessionExerciseID, i+1, set.Reps, set.WeightGrams, set.IsWarmup, set.Completed,
		); err != nil {
			log.Printf("gym: insert set failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("gym: replace sets commit failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	h.writeSession(w, sessionID, http.StatusOK)
}

// LastSets returns the sets logged for an exercise in the most recent session
// that recorded any — what the logging UI prefills so a repeat workout is a
// few taps instead of retyping every number.
//
// @Summary Get an exercise's most recent sets
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Param id path int true "Exercise ID"
// @Success 200 {object} lastSetsResponse
// @Failure 400 {object} httpx.APIError
// @Router /gym/exercises/{id}/last-sets [get]
func (h *handler) LastSets(w http.ResponseWriter, r *http.Request) {
	exerciseID, err := parseID(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	var sessionExerciseID int64
	var occurredOn string
	err = h.db.QueryRow(`
		SELECT se.id, to_char(s.occurred_on, 'YYYY-MM-DD')
		FROM session_exercises se
		JOIN workout_sessions s ON s.id = se.session_id
		WHERE se.exercise_id = $1 AND EXISTS (SELECT 1 FROM exercise_sets st WHERE st.session_exercise_id = se.id)
		ORDER BY s.occurred_on DESC, s.started_at DESC
		LIMIT 1`, exerciseID).Scan(&sessionExerciseID, &occurredOn)
	if err == sql.ErrNoRows {
		httpx.WriteJSON(w, http.StatusOK, lastSetsResponse{Sets: []ExerciseSet{}})
		return
	}
	if err != nil {
		log.Printf("gym: last sets lookup failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	rows, err := h.db.Query(
		`SELECT id, set_number, reps, weight_grams, is_warmup, completed
		 FROM exercise_sets WHERE session_exercise_id = $1 ORDER BY set_number`, sessionExerciseID)
	if err != nil {
		log.Printf("gym: last sets failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	sets := []ExerciseSet{}
	for rows.Next() {
		var s ExerciseSet
		if err := rows.Scan(&s.ID, &s.SetNumber, &s.Reps, &s.WeightGrams, &s.IsWarmup, &s.Completed); err != nil {
			log.Printf("gym: scan last set failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		sets = append(sets, s)
	}
	if err := rows.Err(); err != nil {
		log.Printf("gym: iterate last sets failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, lastSetsResponse{Sets: sets, OccurredOn: occurredOn})
}

func (h *handler) sessionExists(id int64) error {
	var got int64
	return h.db.QueryRow(`SELECT id FROM workout_sessions WHERE id = $1`, id).Scan(&got)
}

// writeSession responds with the session's full current state. Every mutation
// returns it so the client replaces its cache with server truth instead of
// patching a local copy and drifting.
func (h *handler) writeSession(w http.ResponseWriter, id int64, status int) {
	session, err := h.loadSession(id)
	if err != nil {
		log.Printf("gym: load session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, status, session)
}
