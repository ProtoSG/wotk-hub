package gym

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"
	"workhub/httpx"
	"workhub/middleware"
)

// ListSessions returns session summaries, newest first, optionally bounded by
// ?from=/?to= (inclusive YYYY-MM-DD dates). Volume and set counts are
// aggregated in SQL so the history list needs one round trip, not one per
// session.
//
// @Summary List workout sessions
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Param from query string false "Earliest date (YYYY-MM-DD)"
// @Param to query string false "Latest date (YYYY-MM-DD)"
// @Success 200 {object} listSessionsResponse
// @Failure 400 {object} httpx.APIError
// @Router /gym/sessions [get]
func (h *handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	where := " WHERE 1 = 1"
	args := []any{}
	for _, f := range []struct {
		param string
		op    string
	}{{"from", ">="}, {"to", "<="}} {
		value := r.URL.Query().Get(f.param)
		if value == "" {
			continue
		}
		if _, err := time.Parse(dateLayout, value); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid "+f.param+" date")
			return
		}
		args = append(args, value)
		where += " AND s.occurred_on " + f.op + " $" + strconv.Itoa(len(args))
	}

	rows, err := h.db.Query(`
		SELECT s.id, s.routine_id, s.name, s.occurred_on, s.started_at, s.finished_at, s.notes,
		       COUNT(DISTINCT se.id) AS exercise_count,
		       COALESCE(SUM(CASE WHEN st.is_warmup = false AND st.completed THEN st.reps ELSE 0 END), 0) AS total_reps,
		       COALESCE(SUM(CASE WHEN st.is_warmup = false AND st.completed THEN st.reps * st.weight_grams ELSE 0 END), 0) AS total_volume_grams
		FROM workout_sessions s
		LEFT JOIN session_exercises se ON se.session_id = s.id
		LEFT JOIN exercise_sets st ON st.session_exercise_id = se.id`+where+`
		GROUP BY s.id
		ORDER BY s.occurred_on DESC, s.started_at DESC`, args...)
	if err != nil {
		log.Printf("gym: list sessions failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	sessions := []SessionSummary{}
	for rows.Next() {
		s, err := scanSessionSummary(rows)
		if err != nil {
			log.Printf("gym: scan session failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		log.Printf("gym: iterate sessions failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, listSessionsResponse{Sessions: sessions})
}

// ActiveSession returns the session still in progress (finished_at IS NULL),
// or null. The logging UI calls this on mount to decide between "resume" and
// "start a new session", so a missing session is a normal 200 with a null
// body rather than a 404.
//
// @Summary Get the in-progress session
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Success 200 {object} activeSessionResponse
// @Router /gym/sessions/active [get]
func (h *handler) ActiveSession(w http.ResponseWriter, r *http.Request) {
	var id int64
	err := h.db.QueryRow(
		`SELECT id FROM workout_sessions WHERE finished_at IS NULL ORDER BY started_at DESC LIMIT 1`,
	).Scan(&id)
	if err == sql.ErrNoRows {
		httpx.WriteJSON(w, http.StatusOK, activeSessionResponse{Session: nil})
		return
	}
	if err != nil {
		log.Printf("gym: active session lookup failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	session, err := h.loadSession(id)
	if err != nil {
		log.Printf("gym: load active session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, activeSessionResponse{Session: session})
}

// GetSession returns one session with its exercises and sets.
//
// @Summary Get a workout session
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Param id path int true "Session ID"
// @Success 200 {object} Session
// @Failure 404 {object} httpx.APIError
// @Router /gym/sessions/{id} [get]
func (h *handler) GetSession(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	session, err := h.loadSession(id)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "session not found")
		return
	}
	if err != nil {
		log.Printf("gym: get session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, session)
}

// CreateSession starts a freestyle session — exercises are added as they are
// performed. Starting from a routine template lands with routines (P3).
//
// @Summary Start a workout session
// @Tags gym
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body sessionRequest true "Session details"
// @Success 201 {object} Session
// @Failure 400 {object} httpx.APIError
// @Failure 409 {object} httpx.APIError "a session is already in progress"
// @Router /gym/sessions [post]
func (h *handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req sessionRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	// One workout at a time: a second in-progress session would leave the
	// logging UI with no single answer for "what am I doing right now".
	var openID int64
	err := h.db.QueryRow(`SELECT id FROM workout_sessions WHERE finished_at IS NULL LIMIT 1`).Scan(&openID)
	if err == nil {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "a session is already in progress")
		return
	}
	if err != sql.ErrNoRows {
		log.Printf("gym: open session check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	userID, _, _ := middleware.UserFromContext(r.Context())

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("gym: create session begin failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	name := req.Name
	if req.RoutineID != nil {
		// The routine's name is snapshotted onto the session so the history
		// still reads correctly after the template is renamed or deleted.
		var routineName string
		err := tx.QueryRow(`SELECT name FROM routines WHERE id = $1`, *req.RoutineID).Scan(&routineName)
		if err == sql.ErrNoRows {
			httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "routine not found")
			return
		}
		if err != nil {
			log.Printf("gym: routine lookup failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		if name == "" {
			name = routineName
		}
	}

	var id int64
	if err := tx.QueryRow(
		`INSERT INTO workout_sessions (routine_id, name, occurred_on, notes, created_by)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		req.RoutineID, name, req.OccurredOn, req.Notes, userID,
	).Scan(&id); err != nil {
		log.Printf("gym: create session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if req.RoutineID != nil {
		if err := materializeRoutine(tx, id, *req.RoutineID); err != nil {
			log.Printf("gym: materialize routine failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("gym: create session commit failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	session, err := h.loadSession(id)
	if err != nil {
		log.Printf("gym: load created session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, session)
}

// UpdateSession edits the session's own fields. Exercises and sets have their
// own endpoints.
//
// @Summary Update a workout session
// @Tags gym
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path int true "Session ID"
// @Param body body sessionRequest true "Session details"
// @Success 200 {object} Session
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /gym/sessions/{id} [put]
func (h *handler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	var req sessionRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	res, err := h.db.Exec(
		`UPDATE workout_sessions SET name = $1, occurred_on = $2, notes = $3 WHERE id = $4`,
		req.Name, req.OccurredOn, req.Notes, id,
	)
	if err != nil {
		log.Printf("gym: update session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "session not found")
		return
	}

	session, err := h.loadSession(id)
	if err != nil {
		log.Printf("gym: load updated session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, session)
}

// FinishSession stamps finished_at. Idempotent: finishing an already-finished
// session keeps the original timestamp, so a double-tap on a flaky connection
// can't rewrite history.
//
// @Summary Finish a workout session
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Param id path int true "Session ID"
// @Success 200 {object} Session
// @Failure 404 {object} httpx.APIError
// @Router /gym/sessions/{id}/finish [post]
func (h *handler) FinishSession(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	res, err := h.db.Exec(
		`UPDATE workout_sessions SET finished_at = now() WHERE id = $1 AND finished_at IS NULL`, id,
	)
	if err != nil {
		log.Printf("gym: finish session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	// No rows updated means either "already finished" (fine) or "no such
	// session" — loadSession below tells the two apart.
	_ = res

	session, err := h.loadSession(id)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "session not found")
		return
	}
	if err != nil {
		log.Printf("gym: load finished session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, session)
}

// DeleteSession removes a session; its exercises and sets go with it via
// ON DELETE CASCADE.
//
// @Summary Delete a workout session
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Param id path int true "Session ID"
// @Success 200 {object} httpx.SuccessResponse
// @Failure 404 {object} httpx.APIError
// @Router /gym/sessions/{id} [delete]
func (h *handler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	res, err := h.db.Exec(`DELETE FROM workout_sessions WHERE id = $1`, id)
	if err != nil {
		log.Printf("gym: delete session failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "session not found")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}

// materializeRoutine copies a template's exercises into a session and
// pre-creates the empty sets the template targets, so the logger opens with
// the day's plan already laid out and each set is one tap away.
//
// It is a copy, not a reference: from here on the session owns its contents,
// and editing the routine later changes nothing about this workout.
func materializeRoutine(tx *sql.Tx, sessionID, routineID int64) error {
	rows, err := tx.Query(
		`SELECT exercise_id, position, target_sets, target_reps, notes
		 FROM routine_exercises WHERE routine_id = $1 ORDER BY position`, routineID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type planned struct {
		exerciseID int64
		position   int
		sets       int
		reps       int
		notes      string
	}
	plan := []planned{}
	for rows.Next() {
		var p planned
		if err := rows.Scan(&p.exerciseID, &p.position, &p.sets, &p.reps, &p.notes); err != nil {
			return err
		}
		plan = append(plan, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, p := range plan {
		var sessionExerciseID int64
		if err := tx.QueryRow(
			`INSERT INTO session_exercises (session_id, exercise_id, position, notes)
			 VALUES ($1, $2, $3, $4) RETURNING id`,
			sessionID, p.exerciseID, p.position, p.notes,
		).Scan(&sessionExerciseID); err != nil {
			return err
		}

		// Target reps are prefilled but weight is not: the reps are the plan,
		// the weight is whatever the day allows. completed = false — nothing
		// has been lifted yet.
		for i := 1; i <= p.sets; i++ {
			if _, err := tx.Exec(
				`INSERT INTO exercise_sets (session_exercise_id, set_number, reps, weight_grams, is_warmup, completed)
				 VALUES ($1, $2, $3, 0, false, false)`,
				sessionExerciseID, i, p.reps,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

// loadSession reads a session and its nested exercises and sets. Two queries
// regardless of exercise count — the sets are fetched in one pass and bucketed
// in Go rather than queried per exercise.
func (h *handler) loadSession(id int64) (*Session, error) {
	var s Session
	var routineID sql.NullInt64
	var occurredOn, startedAt time.Time
	var finishedAt sql.NullTime

	err := h.db.QueryRow(
		`SELECT id, routine_id, name, occurred_on, started_at, finished_at, notes
		 FROM workout_sessions WHERE id = $1`, id,
	).Scan(&s.ID, &routineID, &s.Name, &occurredOn, &startedAt, &finishedAt, &s.Notes)
	if err != nil {
		return nil, err
	}
	if routineID.Valid {
		s.RoutineID = &routineID.Int64
	}
	s.OccurredOn = occurredOn.Format(dateLayout)
	s.StartedAt = startedAt.Format(time.RFC3339)
	if finishedAt.Valid {
		finished := finishedAt.Time.Format(time.RFC3339)
		s.FinishedAt = &finished
	}

	rows, err := h.db.Query(`
		SELECT se.id, se.exercise_id, se.position, se.notes,
		       e.name, e.equipment, e.primary_muscle, e.secondary_muscle, e.description, e.media_url, e.media_type, e.is_custom
		FROM session_exercises se
		JOIN exercises e ON e.id = se.exercise_id
		WHERE se.session_id = $1
		ORDER BY se.position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	s.Exercises = []SessionExercise{}
	byID := map[int64]int{}
	for rows.Next() {
		var se SessionExercise
		if err := rows.Scan(
			&se.ID, &se.ExerciseID, &se.Position, &se.Notes,
			&se.Exercise.Name, &se.Exercise.Equipment, &se.Exercise.PrimaryMuscle,
			&se.Exercise.SecondaryMuscle, &se.Exercise.Description,
			&se.Exercise.MediaURL, &se.Exercise.MediaType, &se.Exercise.IsCustom,
		); err != nil {
			return nil, err
		}
		se.Exercise.ID = se.ExerciseID
		se.Sets = []ExerciseSet{}
		byID[se.ID] = len(s.Exercises)
		s.Exercises = append(s.Exercises, se)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(s.Exercises) == 0 {
		return &s, nil
	}

	setRows, err := h.db.Query(`
		SELECT st.session_exercise_id, st.id, st.set_number, st.reps, st.weight_grams, st.is_warmup, st.completed
		FROM exercise_sets st
		JOIN session_exercises se ON se.id = st.session_exercise_id
		WHERE se.session_id = $1
		ORDER BY st.set_number`, id)
	if err != nil {
		return nil, err
	}
	defer setRows.Close()

	for setRows.Next() {
		var sessionExerciseID int64
		var set ExerciseSet
		if err := setRows.Scan(
			&sessionExerciseID, &set.ID, &set.SetNumber, &set.Reps, &set.WeightGrams, &set.IsWarmup, &set.Completed,
		); err != nil {
			return nil, err
		}
		if idx, ok := byID[sessionExerciseID]; ok {
			s.Exercises[idx].Sets = append(s.Exercises[idx].Sets, set)
		}
	}
	return &s, setRows.Err()
}

func scanSessionSummary(row interface{ Scan(...any) error }) (SessionSummary, error) {
	var s SessionSummary
	var routineID sql.NullInt64
	var occurredOn, startedAt time.Time
	var finishedAt sql.NullTime

	err := row.Scan(
		&s.ID, &routineID, &s.Name, &occurredOn, &startedAt, &finishedAt, &s.Notes,
		&s.ExerciseCount, &s.TotalReps, &s.TotalVolumeGrams,
	)
	if err != nil {
		return s, err
	}
	if routineID.Valid {
		s.RoutineID = &routineID.Int64
	}
	s.OccurredOn = occurredOn.Format(dateLayout)
	s.StartedAt = startedAt.Format(time.RFC3339)
	if finishedAt.Valid {
		finished := finishedAt.Time.Format(time.RFC3339)
		s.FinishedAt = &finished
	}
	return s, nil
}
