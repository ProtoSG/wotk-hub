package gym

import (
	"log"
	"net/http"
	"strconv"
	"workhub/httpx"
)

const (
	defaultLimit = 50
	maxLimit     = 200
)

func scanExercise(row interface{ Scan(...any) error }) (Exercise, error) {
	var e Exercise
	err := row.Scan(&e.ID, &e.Name, &e.Equipment, &e.PrimaryMuscle, &e.SecondaryMuscle, &e.MediaURL, &e.MediaType, &e.IsCustom)
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
	query := `SELECT id, name, equipment, primary_muscle, secondary_muscle, media_url, media_type, is_custom
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
