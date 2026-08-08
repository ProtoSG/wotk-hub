package pet

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

type handler struct {
	db *sql.DB
}

// limaLoc is a deliberate duplicate of the same constant already living in
// games and push — Peru's UTC-5 offset is a fixed fact (no DST since 1990),
// so this tiny duplication is the pragmatic tradeoff versus a fourth shared
// package for two lines (see push/reminder.go's comment for the fuller
// version of this reasoning; team.ResolveTeamID, by contrast, got extracted
// because it's real query logic worth keeping in one place).
var limaLoc = time.FixedZone("America/Lima", -5*60*60)

func today() string {
	return time.Now().In(limaLoc).Format("2006-01-02")
}

// dateOnly trims a driver-returned DATE value down to its bare "2006-01-02"
// prefix. Postgres DATE columns come back through database/sql as full
// RFC3339 ("2026-08-08T00:00:00Z"), not the bare date — this is the exact
// same gotcha games/handlers.go documents fixing today. An empty input
// (NULL column) stays empty.
func dateOnly(s string) string {
	if len(s) < 10 {
		return s
	}
	return s[:10]
}

// decayPerDay is how much care_score drops for each full day nobody
// interacted with the pet at all. careBoost is how much a single care
// action raises it, capped at 100. Deliberately not configurable per action
// (all 5 boost the same amount) — the 5 actions exist so there's always
// something small to do throughout the day, not to let one matter more
// than another. +6 * 5 actions = 30, same total swing as the old +10 * 3.
const (
	decayPerDay = 15
	careBoost   = 6
	maxCare     = 100
	minCare     = 0
	defaultCare = 70
	// perfectDaySparks is the shop currency payout for completing all 5
	// actions in one calendar day — awarded exactly once, at the moment the
	// 5th action of the day is newly logged (see care()).
	perfectDaySparks = 3
	// freezeCostSparks is the first shop item's price.
	freezeCostSparks = 15
	// renameCostSparks is the second shop item's price — cosmetic, priced
	// lower than the freeze since it doesn't protect anything.
	renameCostSparks = 10
	// maxNameLen keeps the name short enough to fit the pixel-font title
	// without wrapping/overflowing the card header.
	maxNameLen = 20
)

func moodFor(careScore int) string {
	switch {
	case careScore >= 75:
		return "happy"
	case careScore >= 50:
		return "neutral"
	case careScore >= 25:
		return "sad"
	default:
		return "hungry"
	}
}

// actionDef pairs a care action's key (matches pet_care_log.action and the
// route path) with the Lima hour it unlocks at. Unlock semantics: an action
// becomes available starting at its hour and stays available for the rest
// of the calendar day — it never locks back up once unlocked. That's
// deliberate, same "gets sad, never permanently lost" principle as the rest
// of this feature (see MascotaTab's reset dialog copy): a hard time window
// that locked you out entirely would contradict that if the couple opens
// the app late.
type actionDef struct {
	key        string
	unlockHour int
}

var careActions = []actionDef{
	{"bathe", 7},
	{"breakfast", 8},
	{"lunch", 12},
	{"play", 16},
	{"dinner", 19},
}

// unlockHourFor looks up an action's configured unlock hour from
// careActions — the single source of truth for these 5 numbers. Panics on
// an unknown action key since that can only mean a Bathe/Breakfast/Lunch/
// Play/Dinner handler below was typo'd, a programmer error, not a runtime
// condition to recover from.
func unlockHourFor(action string) int {
	for _, a := range careActions {
		if a.key == action {
			return a.unlockHour
		}
	}
	panic("pet: unknown care action " + action)
}

// petRow is the decay/streak-relevant slice of one team's pet_state row,
// threaded through loadOrCreate/care/Reset instead of a growing positional
// return tuple. Attribution and per-action done/locked status live in
// pet_care_log and are resolved separately in respondState — this struct
// only carries what drives the mood/streak math.
type petRow struct {
	id            int64
	teamID        int64
	careScore     int
	streak        int
	sparks        int
	streakFreezes int
	name          string
}

// loadOrCreate fetches the team's pet row, creating it with the default
// care_score if this is the first time anyone's opened the feature, then
// applies decay for every full day since it was last touched. Returns the
// caught-up row — callers building a response should read straight off
// this, not re-query.
func (h *handler) loadOrCreate(teamID int64) (*petRow, error) {
	if _, err := h.db.Exec(
		`INSERT INTO pet_state (team_id, last_updated_on) VALUES ($1, $2)
		 ON CONFLICT (team_id) DO NOTHING`,
		teamID, today(),
	); err != nil {
		return nil, err
	}

	p := petRow{teamID: teamID}
	var lastUpdatedOn string
	row := h.db.QueryRow(
		`SELECT id, care_score, streak, sparks, streak_freezes, name, last_updated_on FROM pet_state WHERE team_id = $1`, teamID,
	)
	if err := row.Scan(&p.id, &p.careScore, &p.streak, &p.sparks, &p.streakFreezes, &p.name, &lastUpdatedOn); err != nil {
		return nil, err
	}
	// DATE columns come back from the driver as full RFC3339
	// ("2026-08-08T00:00:00Z"), not a bare "2006-01-02" date — same bug
	// already hit (and fixed) in games/handlers.go today. Normalize right
	// after scanning so every downstream comparison in this file only ever
	// sees bare dates.
	lastUpdatedOn = dateOnly(lastUpdatedOn)

	// Lazy decay — same "catch up on read, no cron needed" pattern as the
	// riddle game's expireStaleSessions. One arithmetic step handles any
	// number of days absent correctly, not just one.
	//
	// Deliberately NOT using time.Truncate(24*time.Hour) here: it truncates
	// against the Unix epoch in UTC, not the Lima calendar day, regardless
	// of the Time's own Location — the same species of timezone bug fixed
	// three times already today in games/push. Parsing both sides as Lima
	// midnight via ParseInLocation and subtracting those instants directly
	// avoids the ambiguity entirely — no truncation involved.
	lastUpdated, err := time.ParseInLocation("2006-01-02", lastUpdatedOn, limaLoc)
	if err != nil {
		return nil, err
	}
	todayMidnight, err := time.ParseInLocation("2006-01-02", today(), limaLoc)
	if err != nil {
		return nil, err
	}
	daysPassed := int(todayMidnight.Sub(lastUpdated).Hours() / 24)

	// The streak's clock is deliberately NOT the same as decay's. last_updated_on
	// also moves forward whenever decay itself runs (see the UPDATE below), so on
	// a normal daily-care cadence it's always exactly 1 day stale by the time the
	// pet is next opened — using it for the streak would zero the streak every
	// single day even with perfect care. Instead, track the most recent day ANY
	// care action landed (now a SQL MAX over pet_care_log, replacing the old
	// 3-column latestDate scan) and only break the streak once a full day has
	// gone by with none — i.e. more than 1 day since that last action.
	lastCareOn, err := h.latestCareDate(teamID)
	if err != nil {
		return nil, err
	}
	newStreak := p.streak
	freezeConsumed := false
	if lastCareOn == "" {
		newStreak = 0
	} else if lastCare, parseErr := time.ParseInLocation("2006-01-02", lastCareOn, limaLoc); parseErr == nil {
		if daysSinceCare := int(todayMidnight.Sub(lastCare).Hours() / 24); daysSinceCare > 1 {
			// A real neglect gap — but a banked streak freeze absorbs it
			// once, same idea as Duolingo's streak freeze: spend a token
			// instead of losing the streak. Only spends one if there's an
			// actual nonzero streak worth protecting; a freeze never fires
			// against an already-zero streak.
			if p.streak > 0 && p.streakFreezes > 0 {
				freezeConsumed = true
			} else {
				newStreak = 0
			}
		}
	}

	needsUpdate := false
	if daysPassed > 0 {
		p.careScore -= decayPerDay * daysPassed
		if p.careScore < minCare {
			p.careScore = minCare
		}
		needsUpdate = true
	}
	if newStreak != p.streak {
		p.streak = newStreak
		needsUpdate = true
	}
	if freezeConsumed {
		p.streakFreezes--
		needsUpdate = true
	}
	if needsUpdate {
		if _, err := h.db.Exec(
			`UPDATE pet_state SET care_score = $1, streak = $2, streak_freezes = $3, last_updated_on = $4 WHERE id = $5`,
			p.careScore, p.streak, p.streakFreezes, today(), p.id,
		); err != nil {
			return nil, err
		}
	}
	return &p, nil
}

// latestCareDate returns the most recent bare "2006-01-02" care_date logged
// for this team across any of the 5 actions, or "" if none exist yet. Doing
// this comparison in SQL (MAX over the DATE column) rather than scanning it
// into Go for a string comparison sidesteps the DATE-comes-back-as-RFC3339
// gotcha entirely for anything that doesn't strictly need the Go-side value
// — here we do need it (for the streak's day-gap math), so it still goes
// through dateOnly once, just a single value instead of 3.
func (h *handler) latestCareDate(teamID int64) (string, error) {
	var d sql.NullString
	if err := h.db.QueryRow(
		`SELECT MAX(care_date) FROM pet_care_log WHERE team_id = $1`, teamID,
	).Scan(&d); err != nil {
		return "", err
	}
	if !d.Valid {
		return "", nil
	}
	return dateOnly(d.String), nil
}

// todayActionAttribution returns, for whichever of the 5 actions have
// already been logged today for this team, the display name of whoever did
// it — mirrors games/handlers.go's JOIN pattern for resolving a solver id
// to a name (there: riddle_attempts.solver_id -> users.name; here:
// pet_care_log.user_id -> users.name).
func (h *handler) todayActionAttribution(teamID int64) (map[string]string, error) {
	rows, err := h.db.Query(
		`SELECT pcl.action, u.name
		 FROM pet_care_log pcl
		 JOIN users u ON pcl.user_id = u.id
		 WHERE pcl.team_id = $1 AND pcl.care_date = $2`,
		teamID, today(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byAction := make(map[string]string, len(careActions))
	for rows.Next() {
		var action, name string
		if err := rows.Scan(&action, &name); err != nil {
			return nil, err
		}
		byAction[action] = name
	}
	return byAction, rows.Err()
}

// respondState builds and writes the full 5-action State for the given
// caught-up pet row: today's per-action done/attribution from pet_care_log,
// plus locked/unlocksAtHour computed from the current Lima hour so the
// frontend never does its own hour math.
func (h *handler) respondState(w http.ResponseWriter, p *petRow) {
	doneBy, err := h.todayActionAttribution(p.teamID)
	if err != nil {
		log.Printf("pet: load today's care log failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	hour := time.Now().In(limaLoc).Hour()
	statuses := make(map[string]ActionStatus, len(careActions))
	perfectDay := true
	for _, a := range careActions {
		name, done := doneBy[a.key]
		status := ActionStatus{
			Done:          done,
			Locked:        hour < a.unlockHour,
			UnlocksAtHour: a.unlockHour,
		}
		if done {
			n := name
			status.By = &n
		} else {
			perfectDay = false
		}
		statuses[a.key] = status
	}

	httpx.WriteJSON(w, http.StatusOK, stateResponse{Pet: State{
		CareScore:     p.careScore,
		Mood:          moodFor(p.careScore),
		Name:          p.name,
		Streak:        p.streak,
		Sparks:        p.sparks,
		StreakFreezes: p.streakFreezes,
		PerfectDay:    perfectDay,
		Bathe:         statuses["bathe"],
		Breakfast:     statuses["breakfast"],
		Lunch:         statuses["lunch"],
		Play:          statuses["play"],
		Dinner:        statuses["dinner"],
	}})
}

// GetState returns (creating if needed) the shared pet's current status,
// caught up on any decay since the last visit.
func (h *handler) GetState(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := middleware.UserFromContext(r.Context()); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("pet: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	p, err := h.loadOrCreate(teamID)
	if err != nil {
		log.Printf("pet: load state failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	h.respondState(w, p)
}

// care applies one of the 5 daily actions, gated by its configured unlock
// hour (looked up via unlockHourFor — careActions is the only place these
// numbers live). Idempotent once already done today — the pet_care_log
// UNIQUE(team_id, action, care_date) constraint IS the idempotency
// mechanism: a repeat call (double-tap, stale UI) just returns the current
// state rather than erroring, the same "don't punish a redundant request"
// stance the riddle game's guess-after-solved path uses.
func (h *handler) care(w http.ResponseWriter, r *http.Request, action string) {
	unlockHour := unlockHourFor(action)
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("pet: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	// Server-enforced unlock guard. The frontend already disables the
	// button and hides the icon before this hour, but that's a UI nicety,
	// not a boundary — a direct API call before unlockHour must not be able
	// to jump the queue, same "admin-only enforced here too, not just
	// hidden in the UI" stance Reset already takes below. Respond with the
	// caught-up current state rather than an error: this path can only be
	// hit by a stale client or a direct call (a correctly-rendered UI never
	// lets you tap a locked button), and erroring out would contradict the
	// pet's "gets sad, never permanently lost" design principle for no
	// reason — there's nothing punitive to communicate here.
	if time.Now().In(limaLoc).Hour() < unlockHour {
		p, err := h.loadOrCreate(teamID)
		if err != nil {
			log.Printf("pet: load state failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		h.respondState(w, p)
		return
	}

	p, err := h.loadOrCreate(teamID)
	if err != nil {
		log.Printf("pet: load state failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	// Streak bumps once per day, on whichever of the 5 actions happens
	// first — checked BEFORE inserting this one, so a 2nd..5th action the
	// same day doesn't bump it again.
	doneToday, err := h.todayActionAttribution(teamID)
	if err != nil {
		log.Printf("pet: load today's care log failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	firstActionToday := len(doneToday) == 0

	// INSERT ... ON CONFLICT DO NOTHING RETURNING id: a row comes back only
	// if this action genuinely wasn't logged yet today for this team. No
	// row back (sql.ErrNoRows from Scan) means it was already done — no-op,
	// fall straight through to respondState with the state as loaded.
	var insertedID int64
	insertErr := h.db.QueryRow(
		`INSERT INTO pet_care_log (team_id, action, care_date, user_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (team_id, action, care_date) DO NOTHING
		 RETURNING id`,
		teamID, action, today(), userID,
	).Scan(&insertedID)

	switch {
	case insertErr == nil:
		// New action today — apply the boost and maybe bump the streak.
		if firstActionToday {
			p.streak++
		}
		p.careScore += careBoost
		if p.careScore > maxCare {
			p.careScore = maxCare
		}
		// doneToday was read BEFORE this insert — if it already had all 4
		// other actions, this one just completed the 5th. Checking it this
		// way (rather than re-querying after the insert) guarantees the
		// payout fires exactly once: the UNIQUE(team_id, action, care_date)
		// constraint means this action can only ever be the "newly inserted"
		// one a single time per day, so this branch can only run once too.
		if len(doneToday) == len(careActions)-1 {
			p.sparks += perfectDaySparks
		}
		if _, err := h.db.Exec(
			`UPDATE pet_state SET care_score = $1, streak = $2, sparks = $3, last_updated_on = $4 WHERE id = $5`,
			p.careScore, p.streak, p.sparks, today(), p.id,
		); err != nil {
			log.Printf("pet: apply care action failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
	case insertErr == sql.ErrNoRows:
		// Already done today — idempotent no-op, nothing to apply.
	default:
		log.Printf("pet: log care action failed: %v", insertErr)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	h.respondState(w, p)
}

func (h *handler) Bathe(w http.ResponseWriter, r *http.Request)     { h.care(w, r, "bathe") }
func (h *handler) Breakfast(w http.ResponseWriter, r *http.Request) { h.care(w, r, "breakfast") }
func (h *handler) Lunch(w http.ResponseWriter, r *http.Request)     { h.care(w, r, "lunch") }
func (h *handler) Play(w http.ResponseWriter, r *http.Request)      { h.care(w, r, "play") }
func (h *handler) Dinner(w http.ResponseWriter, r *http.Request)    { h.care(w, r, "dinner") }

// BuyFreeze spends freezeCostSparks to add one streak freeze to the team's
// balance. Not admin-gated — unlike Reset this doesn't discard anything,
// it's a normal spend either partner can make, same as the 5 care actions.
func (h *handler) BuyFreeze(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := middleware.UserFromContext(r.Context()); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("pet: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	p, err := h.loadOrCreate(teamID)
	if err != nil {
		log.Printf("pet: load state failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if p.sparks < freezeCostSparks {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "not enough sparks")
		return
	}
	p.sparks -= freezeCostSparks
	p.streakFreezes++
	if _, err := h.db.Exec(
		`UPDATE pet_state SET sparks = $1, streak_freezes = $2 WHERE id = $3`,
		p.sparks, p.streakFreezes, p.id,
	); err != nil {
		log.Printf("pet: buy freeze failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	h.respondState(w, p)
}

type renameRequest struct {
	Name string `json:"name"`
}

// Rename sets the pet's display name. The very first name (p.name == "",
// i.e. it's never been named) is free — that's the couple's initial
// naming moment, not a shop purchase. Every rename after that spends
// renameCostSparks. Not admin-gated, same reasoning as BuyFreeze — a
// normal spend either partner can make.
func (h *handler) Rename(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := middleware.UserFromContext(r.Context()); !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	var req renameRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len([]rune(name)) > maxNameLen {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "name must be 1-20 characters")
		return
	}

	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("pet: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	p, err := h.loadOrCreate(teamID)
	if err != nil {
		log.Printf("pet: load state failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	cost := renameCostSparks
	if p.name == "" {
		cost = 0
	}
	if p.sparks < cost {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "not enough sparks")
		return
	}
	p.sparks -= cost
	p.name = name
	if _, err := h.db.Exec(
		`UPDATE pet_state SET sparks = $1, name = $2 WHERE id = $3`,
		p.sparks, p.name, p.id,
	); err != nil {
		log.Printf("pet: rename failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	h.respondState(w, p)
}

// Reset wipes the shared pet back to its default state (care_score 70, no
// action done today, streak 0, sparks/freezes 0, name cleared). Admin-only
// — this discards real shared progress, not just a view, so unlike the
// frontend-only canManage/canSeePrice gates elsewhere it's enforced here
// too, not just hidden in the UI.
func (h *handler) Reset(w http.ResponseWriter, r *http.Request) {
	_, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	if role != "admin" {
		httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden, "admin only")
		return
	}
	teamID, err := team.ResolveTeamID(h.db)
	if err != nil {
		log.Printf("pet: resolve team id failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if _, err := h.db.Exec(
		`UPDATE pet_state SET care_score = $1, streak = 0, sparks = 0, streak_freezes = 0, name = '', last_updated_on = $2 WHERE team_id = $3`,
		defaultCare, today(), teamID,
	); err != nil {
		log.Printf("pet: reset failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	// Also clear today's care log rows, otherwise the 5 actions would come
	// back as "already done" (with stale attribution) the moment
	// respondState re-queries pet_care_log right after this reset.
	if _, err := h.db.Exec(
		`DELETE FROM pet_care_log WHERE team_id = $1 AND care_date = $2`,
		teamID, today(),
	); err != nil {
		log.Printf("pet: reset care log failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	p, err := h.loadOrCreate(teamID)
	if err != nil {
		log.Printf("pet: load state after reset failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	h.respondState(w, p)
}
