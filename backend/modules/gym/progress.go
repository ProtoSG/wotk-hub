package gym

import (
	"log"
	"net/http"
	"strconv"
	"time"
	"workhub/httpx"
)

// ExerciseProgress returns one point per session that logged this exercise,
// oldest first.
//
// Every metric ignores warmups and uncompleted sets: they are not work done,
// and counting them would make a heavy session look identical to a planned one
// that never happened.
//
// @Summary Get an exercise's progress over time
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Param id path int true "Exercise ID"
// @Param from query string false "Earliest date (YYYY-MM-DD)"
// @Param to query string false "Latest date (YYYY-MM-DD)"
// @Success 200 {object} exerciseProgressResponse
// @Failure 400 {object} httpx.APIError
// @Router /gym/progress/exercises/{id} [get]
func (h *handler) ExerciseProgress(w http.ResponseWriter, r *http.Request) {
	exerciseID, err := parseID(r, "id")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	where := ""
	args := []any{exerciseID}
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

	// `worked` narrows to this exercise's real sets once; `top` picks the
	// heaviest set per session (DISTINCT ON), which is what the estimated 1RM
	// is computed from. Epley: weight x (1 + reps/30).
	rows, err := h.db.Query(`
		WITH worked AS (
			SELECT s.id AS session_id, s.occurred_on, s.started_at, st.reps, st.weight_grams
			FROM exercise_sets st
			JOIN session_exercises se ON se.id = st.session_exercise_id
			JOIN workout_sessions s ON s.id = se.session_id
			WHERE se.exercise_id = $1 AND st.is_warmup = false AND st.completed`+where+`
		),
		top AS (
			SELECT DISTINCT ON (session_id) session_id, reps, weight_grams
			FROM worked
			ORDER BY session_id, weight_grams DESC, reps DESC
		)
		SELECT w.session_id,
		       to_char(w.occurred_on, 'YYYY-MM-DD'),
		       MAX(w.weight_grams),
		       SUM(w.reps),
		       SUM(w.reps * w.weight_grams),
		       t.reps,
		       t.weight_grams,
		       ROUND(t.weight_grams * (1 + t.reps / 30.0))
		FROM worked w
		JOIN top t ON t.session_id = w.session_id
		GROUP BY w.session_id, w.occurred_on, w.started_at, t.reps, t.weight_grams
		ORDER BY w.occurred_on, w.started_at`, args...)
	if err != nil {
		log.Printf("gym: exercise progress failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	points := []ProgressPoint{}
	for rows.Next() {
		var p ProgressPoint
		if err := rows.Scan(
			&p.SessionID, &p.OccurredOn, &p.MaxWeightGrams, &p.TotalReps, &p.TotalVolumeGrams,
			&p.TopSet.Reps, &p.TopSet.WeightGrams, &p.Estimated1RMGrams,
		); err != nil {
			log.Printf("gym: scan progress point failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		log.Printf("gym: iterate progress failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, exerciseProgressResponse{Points: points})
}

// ProgressSummary returns the headline numbers for the progress screen.
//
// @Summary Get training summary
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Success 200 {object} ProgressSummaryResponse
// @Router /gym/progress/summary [get]
func (h *handler) ProgressSummary(w http.ResponseWriter, r *http.Request) {
	var summary ProgressSummaryResponse

	err := h.db.QueryRow(`
		SELECT COUNT(DISTINCT s.id),
		       COALESCE(SUM(CASE WHEN st.is_warmup = false AND st.completed
		                         THEN st.reps * st.weight_grams ELSE 0 END), 0)
		FROM workout_sessions s
		LEFT JOIN session_exercises se ON se.session_id = s.id
		LEFT JOIN exercise_sets st ON st.session_exercise_id = se.id
		WHERE date_trunc('month', s.occurred_on) = date_trunc('month', CURRENT_DATE)`,
	).Scan(&summary.SessionsThisMonth, &summary.VolumeThisMonthGrams)
	if err != nil {
		log.Printf("gym: progress summary failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	// Streak counts consecutive weeks with at least one session, not days:
	// rest days are part of training, so a day-based streak would break every
	// single week and mean nothing. row_number() against the week index gives
	// a constant per unbroken run; the run containing this week (or the last
	// finished one) is the current streak.
	err = h.db.QueryRow(`
		WITH weeks AS (
			SELECT DISTINCT date_trunc('week', occurred_on)::date AS week
			FROM workout_sessions
			WHERE occurred_on <= CURRENT_DATE
		),
		runs AS (
			SELECT week,
			       week - (row_number() OVER (ORDER BY week))::int * 7 AS run_key
			FROM weeks
		)
		SELECT COALESCE(COUNT(*), 0)
		FROM runs
		WHERE run_key = (
			SELECT run_key FROM runs
			WHERE week >= date_trunc('week', CURRENT_DATE)::date - 7
			ORDER BY week DESC LIMIT 1
		)`,
	).Scan(&summary.WeekStreak)
	if err != nil {
		log.Printf("gym: week streak failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	// Most-trained muscle over the trailing 90 days — long enough to describe
	// a training block, short enough to reflect what is being worked now.
	var topMuscle *string
	err = h.db.QueryRow(`
		SELECT e.primary_muscle
		FROM exercise_sets st
		JOIN session_exercises se ON se.id = st.session_exercise_id
		JOIN workout_sessions s ON s.id = se.session_id
		JOIN exercises e ON e.id = se.exercise_id
		WHERE st.is_warmup = false AND st.completed
		  AND s.occurred_on >= CURRENT_DATE - INTERVAL '90 days'
		GROUP BY e.primary_muscle
		ORDER BY COUNT(*) DESC, e.primary_muscle
		LIMIT 1`,
	).Scan(&topMuscle)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Printf("gym: top muscle failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if topMuscle != nil {
		summary.TopMuscle = *topMuscle
	}

	httpx.WriteJSON(w, http.StatusOK, summary)
}

// LoggedExercises lists the exercises that actually have logged sets, so the
// progress picker offers the handful worth charting instead of all 413.
//
// @Summary List exercises with logged sets
// @Tags gym
// @Produce json
// @Security CookieAuth
// @Success 200 {object} listExercisesResponse
// @Router /gym/progress/exercises [get]
func (h *handler) LoggedExercises(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT e.id, e.name, e.equipment, e.primary_muscle, e.secondary_muscle,
		       e.description, e.media_url, e.media_type, e.is_custom
		FROM exercises e
		WHERE EXISTS (
			SELECT 1
			FROM session_exercises se
			JOIN exercise_sets st ON st.session_exercise_id = se.id
			WHERE se.exercise_id = e.id AND st.is_warmup = false AND st.completed
		)
		ORDER BY e.name`)
	if err != nil {
		log.Printf("gym: logged exercises failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	exercises := []Exercise{}
	for rows.Next() {
		e, err := scanExercise(rows)
		if err != nil {
			log.Printf("gym: scan logged exercise failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		exercises = append(exercises, e)
	}
	if err := rows.Err(); err != nil {
		log.Printf("gym: iterate logged exercises failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, listExercisesResponse{Exercises: exercises, Total: int64(len(exercises))})
}
