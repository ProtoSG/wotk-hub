package gym

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"
	"workhub/httpx"
	"workhub/middleware"

	"github.com/lib/pq"
)

const (
	defaultLimit = 50
	maxLimit     = 200
)

func scanExercise(row interface{ Scan(...any) error }) (Exercise, error) {
	var e Exercise
	err := row.Scan(&e.ID, &e.Name, &e.Equipment, &e.PrimaryMuscle, &e.SecondaryMuscle, &e.Description, &e.TrackingType, &e.MediaURL, &e.MediaType, &e.IsCustom)
	return e, err
}

// ListExercises returns the catalog, filtered by name, muscle and equipment,
// and paginated. Total is the unpaginated match count so the UI can show
// "showing N of M" without a second request.
//
// @Summary List exercises
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Param q query string false "Filter by name (case-insensitive substring)"
// @Param muscle query string false "Filter by primary muscle"
// @Param equipment query string false "Filter by equipment"
// @Param limit query int false "Page size (default 50, max 200)"
// @Param offset query int false "Page offset"
// @Success 200 {object} listExercisesResponse
// @Failure 400 {object} httpx.APIError
// @Router /gym/exercises [get]
func (h *handler) ListExercises(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, offset, err := parsePaging(q.Get("limit"), q.Get("offset"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	where := " WHERE 1 = 1"
	args := []any{}
	if name := q.Get("q"); name != "" {
		args = append(args, "%"+name+"%")
		where += " AND name ILIKE $" + strconv.Itoa(len(args))
	}
	if muscle := q.Get("muscle"); muscle != "" {
		args = append(args, muscle)
		where += " AND primary_muscle = $" + strconv.Itoa(len(args))
	}
	if equipment := q.Get("equipment"); equipment != "" {
		args = append(args, equipment)
		where += " AND equipment = $" + strconv.Itoa(len(args))
	}

	var total int64
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM exercises`+where, args...).Scan(&total); err != nil {
		log.Printf("gym: count exercises failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	args = append(args, limit, offset)
	query := `SELECT id, name, equipment, primary_muscle, secondary_muscle, description, tracking_type, media_url, media_type, is_custom
		FROM exercises` + where +
		` ORDER BY name
		  LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))

	rows, err := h.db.Query(query, args...)
	if err != nil {
		log.Printf("gym: list exercises failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	exercises := []Exercise{}
	for rows.Next() {
		e, err := scanExercise(rows)
		if err != nil {
			log.Printf("gym: scan exercise failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		exercises = append(exercises, e)
	}
	if err := rows.Err(); err != nil {
		log.Printf("gym: iterate exercises failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, listExercisesResponse{Exercises: exercises, Total: total})
}

// ExerciseFilters returns the distinct muscle and equipment values present in
// the catalog, so the picker's dropdowns stay in sync with the data instead of
// hardcoding the CSV's domains in the frontend.
//
// @Summary List exercise filter values
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Success 200 {object} exerciseFiltersResponse
// @Router /gym/exercises/filters [get]
func (h *handler) ExerciseFilters(w http.ResponseWriter, r *http.Request) {
	muscles, err := h.distinctColumn(`primary_muscle`)
	if err != nil {
		log.Printf("gym: list muscles failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	equipment, err := h.distinctColumn(`equipment`)
	if err != nil {
		log.Printf("gym: list equipment failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, exerciseFiltersResponse{Muscles: muscles, Equipment: equipment})
}

// distinctColumn returns the sorted distinct non-empty values of a column.
// column is never caller-supplied — it comes from the two literals in
// ExerciseFilters — so there is no injection surface here.
func (h *handler) distinctColumn(column string) ([]string, error) {
	rows, err := h.db.Query(`SELECT DISTINCT ` + column + ` FROM exercises WHERE ` + column + ` <> '' ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

// exerciseUniqueViolation mirrors finances.categoryUniqueViolation — the same
// Postgres error code, redefined locally since that constant is unexported
// outside its package.
const exerciseUniqueViolation = "23505"

// CreateExercise adds a user-defined exercise to the catalog. It is always
// flagged is_custom, which is what keeps the CSV seeder from ever touching it.
//
// @Summary Create a custom exercise
// @Tags gym
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body exerciseRequest true "Exercise details"
// @Success 201 {object} Exercise
// @Failure 400 {object} httpx.APIError
// @Failure 409 {object} httpx.APIError "an exercise with that name already exists"
// @Router /gym/exercises [post]
func (h *handler) CreateExercise(w http.ResponseWriter, r *http.Request) {
	var req exerciseRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	userID, _, _ := middleware.UserFromContext(r.Context())
	row := h.db.QueryRow(
		`INSERT INTO exercises (name, equipment, primary_muscle, secondary_muscle, description, tracking_type, is_custom, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, true, $7)
		 RETURNING id, name, equipment, primary_muscle, secondary_muscle, description, tracking_type, media_url, media_type, is_custom`,
		strings.TrimSpace(req.Name), strings.TrimSpace(req.Equipment),
		strings.TrimSpace(req.PrimaryMuscle), strings.TrimSpace(req.SecondaryMuscle),
		strings.TrimSpace(req.Description), defaultTo(req.TrackingType, TrackingWeightReps), userID,
	)
	exercise, err := scanExercise(row)
	if pqErr, ok := err.(*pq.Error); ok && string(pqErr.Code) == exerciseUniqueViolation {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "ya existe un ejercicio con ese nombre")
		return
	}
	if err != nil {
		log.Printf("gym: create exercise failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, exercise)
}

// UpdateExercise edits a custom exercise in full.
//
// Seeded rows reject this: their name and muscles come from the imported
// catalog, and rewriting them would fork it silently on the next import.
// Their text is editable through UpdateExerciseDescription instead.
//
// @Summary Update a custom exercise
// @Tags gym
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path int true "Exercise ID"
// @Param body body exerciseRequest true "Exercise details"
// @Success 200 {object} Exercise
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Failure 409 {object} httpx.APIError
// @Router /gym/exercises/{id} [put]
func (h *handler) UpdateExercise(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	var req exerciseRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	isCustom, err := h.exerciseIsCustom(id)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "exercise not found")
		return
	}
	if err != nil {
		log.Printf("gym: update exercise lookup failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if !isCustom {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict,
			"solo se pueden editar los ejercicios propios")
		return
	}

	row := h.db.QueryRow(
		`UPDATE exercises
		 SET name = $1, equipment = $2, primary_muscle = $3, secondary_muscle = $4, description = $5,
		     tracking_type = $6
		 WHERE id = $7
		 RETURNING id, name, equipment, primary_muscle, secondary_muscle, description, tracking_type, media_url, media_type, is_custom`,
		strings.TrimSpace(req.Name), strings.TrimSpace(req.Equipment),
		strings.TrimSpace(req.PrimaryMuscle), strings.TrimSpace(req.SecondaryMuscle),
		strings.TrimSpace(req.Description), defaultTo(req.TrackingType, TrackingWeightReps), id,
	)
	exercise, err := scanExercise(row)
	if pqErr, ok := err.(*pq.Error); ok && string(pqErr.Code) == exerciseUniqueViolation {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "ya existe un ejercicio con ese nombre")
		return
	}
	if err != nil {
		log.Printf("gym: update exercise failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, exercise)
}

// UpdateExerciseDescription edits only the how-to text, and works for seeded
// exercises too. The seeder only writes descriptions that are empty, so text
// saved here survives every later boot.
//
// @Summary Update an exercise's description
// @Tags gym
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path int true "Exercise ID"
// @Param body body descriptionRequest true "Description"
// @Success 200 {object} Exercise
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /gym/exercises/{id}/description [put]
func (h *handler) UpdateExerciseDescription(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	var req descriptionRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	row := h.db.QueryRow(
		`UPDATE exercises SET description = $1 WHERE id = $2
		 RETURNING id, name, equipment, primary_muscle, secondary_muscle, description, tracking_type, media_url, media_type, is_custom`,
		strings.TrimSpace(req.Description), id,
	)
	exercise, err := scanExercise(row)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "exercise not found")
		return
	}
	if err != nil {
		log.Printf("gym: update exercise description failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, exercise)
}

// DeleteExercise removes a custom exercise, refusing when a session or a
// routine still points at it — the exercises FK has no ON DELETE action, so
// this is the check that turns a constraint violation into a clear message.
//
// @Summary Delete a custom exercise
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Param id path int true "Exercise ID"
// @Success 200 {object} httpx.SuccessResponse
// @Failure 404 {object} httpx.APIError
// @Failure 409 {object} httpx.APIError "the exercise is seeded or still in use"
// @Router /gym/exercises/{id} [delete]
func (h *handler) DeleteExercise(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	isCustom, err := h.exerciseIsCustom(id)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "exercise not found")
		return
	}
	if err != nil {
		log.Printf("gym: delete exercise lookup failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if !isCustom {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict,
			"solo se pueden eliminar los ejercicios propios")
		return
	}

	var inUse bool
	err = h.db.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM session_exercises WHERE exercise_id = $1)
		     OR EXISTS (SELECT 1 FROM routine_exercises WHERE exercise_id = $1)`, id,
	).Scan(&inUse)
	if err != nil {
		log.Printf("gym: delete exercise usage check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if inUse {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict,
			"el ejercicio está en uso en un entrenamiento o una rutina")
		return
	}

	res, err := h.db.Exec(`DELETE FROM exercises WHERE id = $1`, id)
	if err != nil {
		log.Printf("gym: delete exercise failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "exercise not found")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}

func (h *handler) exerciseIsCustom(id int64) (bool, error) {
	var isCustom bool
	err := h.db.QueryRow(`SELECT is_custom FROM exercises WHERE id = $1`, id).Scan(&isCustom)
	return isCustom, err
}
