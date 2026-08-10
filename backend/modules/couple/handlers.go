package couple

import (
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"
	"workhub/httpx"
	"workhub/middleware"
	"workhub/shared/team"
)

func scanDate(row interface{ Scan(...any) error }) (Date, error) {
	var d Date
	var occurredOn, createdAt time.Time
	var costCents sql.NullInt64
	var rating sql.NullInt64
	err := row.Scan(&d.ID, &occurredOn, &d.Place, &d.Category, &d.Notes, &costCents, &rating, &d.TiktokURL, &d.Status, &createdAt)
	if err != nil {
		return d, err
	}
	d.OccurredOn = occurredOn.Format(dateLayout)
	d.CreatedAt = createdAt.Format(time.RFC3339)
	if costCents.Valid {
		d.CostCents = &costCents.Int64
	}
	if rating.Valid {
		r := int(rating.Int64)
		d.Rating = &r
	}
	return d, nil
}

// ListDates returns every couple date, shared between both roles.
//
// @Summary List couple dates
// @Tags couple
// @Produce json
// @Security CookieAuth
// @Success 200 {object} listDatesResponse
// @Failure 401 {object} httpx.APIError
// @Router /couple/dates [get]
func (h *handler) ListDates(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, occurred_on, place, category, notes, cost_cents, rating, tiktok_url, status, created_at
		FROM couple_dates ORDER BY occurred_on DESC, id DESC`)
	if err != nil {
		log.Printf("couple: list dates failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	dates := []Date{}
	for rows.Next() {
		d, err := scanDate(rows)
		if err != nil {
			log.Printf("couple: scan date failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		dates = append(dates, d)
	}
	httpx.WriteJSON(w, http.StatusOK, listDatesResponse{Dates: dates})
}

// CreateDate stamps created_by for provenance only (who logged it) — Citas
// stays a shared view for both roles, never filtered by it.
// CreateDate creates a new couple date entry.
//
// @Summary Create a couple date
// @Tags couple
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body dateRequest true "Date details"
// @Success 201 {object} Date
// @Failure 400 {object} httpx.APIError
// @Failure 401 {object} httpx.APIError
// @Router /couple/dates [post]
func (h *handler) CreateDate(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	var req dateRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if req.Status == "" {
		req.Status = "done"
	}
	occurredOn, err := req.validate()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	row := h.db.QueryRow(
		`INSERT INTO couple_dates (occurred_on, place, category, notes, cost_cents, rating, tiktok_url, status, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, occurred_on, place, category, notes, cost_cents, rating, tiktok_url, status, created_at`,
		occurredOn, req.Place, req.Category, req.Notes, req.CostCents, req.Rating, req.TiktokURL, req.Status, userID,
	)
	d, err := scanDate(row)
	if err != nil {
		log.Printf("couple: create date failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, d)
}

// UpdateDate updates an existing couple date entry.
//
// @Summary Update a couple date
// @Tags couple
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path int true "Date ID"
// @Param body body dateRequest true "Date details"
// @Success 200 {object} Date
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /couple/dates/{id} [put]
func (h *handler) UpdateDate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	var req dateRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	occurredOn, err := req.validate()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	row := h.db.QueryRow(
		`UPDATE couple_dates
		 SET occurred_on = $1, place = $2, category = $3, notes = $4, cost_cents = $5, rating = $6, tiktok_url = $7, status = $8
		 WHERE id = $9
		 RETURNING id, occurred_on, place, category, notes, cost_cents, rating, tiktok_url, status, created_at`,
		occurredOn, req.Place, req.Category, req.Notes, req.CostCents, req.Rating, req.TiktokURL, req.Status, id,
	)
	d, err := scanDate(row)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "date not found")
		return
	}
	if err != nil {
		log.Printf("couple: update date failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}

// DeleteDate deletes a couple date entry.
//
// @Summary Delete a couple date
// @Tags couple
// @Produce json
// @Security CookieAuth
// @Param id path int true "Date ID"
// @Success 200 {object} httpx.SuccessResponse
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /couple/dates/{id} [delete]
func (h *handler) DeleteDate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	// Delete each photo's and video's MinIO objects before the SQL DELETE
	// below cascades away the couple_date_photos/couple_date_videos rows —
	// ON DELETE CASCADE only reaches the DB, so skipping this would orphan
	// objects in MinIO forever with nothing left pointing at them.
	if h.storage != nil {
		h.deletePhotoObjectsForDate(r.Context(), id)
		h.deleteVideoObjectsForDate(r.Context(), id)
	}

	res, err := h.db.Exec(`DELETE FROM couple_dates WHERE id = $1`, id)
	if err != nil {
		log.Printf("couple: delete date failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "date not found")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}

// ListPoems returns all poems for the couple team, with isMine and isSeen
// computed per-request from the JWT. It also auto-marks all unseen poems
// written by the other partner as seen in the same request.
func (h *handler) ListPoems(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("couple: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	// Auto-mark unseen poems from the other partner as seen.
	_, err = h.db.Exec(`UPDATE couple_poems SET seen_at = now()
		WHERE team_id = $1 AND author_id != $2 AND seen_at IS NULL`, teamID, userID)
	if err != nil {
		log.Printf("couple: mark poems seen failed: %v", err)
	}

	rows, err := h.db.Query(`SELECT id, author_id, content, created_at, seen_at
		FROM couple_poems WHERE team_id = $1 ORDER BY created_at ASC`, teamID)
	if err != nil {
		log.Printf("couple: list poems failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	poems := []Poem{}
	for rows.Next() {
		var p Poem
		var authorID int64
		var createdAt time.Time
		var seenAt sql.NullTime
		if err := rows.Scan(&p.ID, &authorID, &p.Content, &createdAt, &seenAt); err != nil {
			log.Printf("couple: scan poem failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		p.IsMine = authorID == userID
		p.IsSeen = seenAt.Valid
		p.CreatedAt = createdAt.Format(time.RFC3339)
		poems = append(poems, p)
	}
	httpx.WriteJSON(w, http.StatusOK, listPoemsResponse{Poems: poems})
}

// CreatePoem writes a new poem from the authenticated user.
func (h *handler) CreatePoem(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("couple: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	var req poemRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	req.Content = trimString(req.Content)
	if req.Content == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "content is required")
		return
	}
	if len(req.Content) > 1000 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "content must be 1000 characters or fewer")
		return
	}

	var p Poem
	var createdAt time.Time
	err = h.db.QueryRow(
		`INSERT INTO couple_poems (author_id, team_id, content) VALUES ($1, $2, $3)
		 RETURNING id, content, created_at`,
		userID, teamID, req.Content,
	).Scan(&p.ID, &p.Content, &createdAt)
	if err != nil {
		log.Printf("couple: create poem failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	p.IsMine = true
	p.IsSeen = false
	p.CreatedAt = createdAt.Format(time.RFC3339)
	httpx.WriteJSON(w, http.StatusCreated, p)
}

// DeletePoem deletes a poem, but only if the caller is its author.
func (h *handler) DeletePoem(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	id, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	res, err := h.db.Exec(`DELETE FROM couple_poems WHERE id = $1 AND author_id = $2`, id, userID)
	if err != nil {
		log.Printf("couple: delete poem failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "poem not found")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}

// MarkPoemSeen marks a poem as seen. Idempotent — re-calling is a no-op.
func (h *handler) MarkPoemSeen(w http.ResponseWriter, r *http.Request) {
	_, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	id, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	_, err = h.db.Exec(`UPDATE couple_poems SET seen_at = coalesce(seen_at, now()) WHERE id = $1`, id)
	if err != nil {
		log.Printf("couple: mark poem seen failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}

// GetTodayPoem returns the poem written by the partner today, if any.
// Returns 200 with null poem when none exists — caller always gets a usable
// payload regardless of data presence.
func (h *handler) GetTodayPoem(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("couple: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	var p Poem
	var authorID int64
	var createdAt time.Time
	err = h.db.QueryRow(
		`SELECT id, author_id, content, created_at
		 FROM couple_poems
		 WHERE team_id = $1 AND author_id != $2 AND DATE(created_at) = CURRENT_DATE
		 LIMIT 1`,
		teamID, userID,
	).Scan(&p.ID, &authorID, &p.Content, &createdAt)
	if err == sql.ErrNoRows {
		httpx.WriteJSON(w, http.StatusOK, todayPoemResponse{})
		return
	}
	if err != nil {
		log.Printf("couple: get today poem failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	p.IsMine = false // current user is reading the partner's poem
	p.IsSeen = true
	p.CreatedAt = createdAt.Format(time.RFC3339)
	httpx.WriteJSON(w, http.StatusOK, todayPoemResponse{Poem: &p})
}

func trimString(s string) string {
	return strings.TrimSpace(s)
}
