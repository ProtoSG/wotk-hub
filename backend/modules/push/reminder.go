package push

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"workhub/shared/team"
)

// reminderTTL bounds how long a push service holds an undelivered
// notification before giving up — 1h is plenty for "the phone was offline
// for a bit", short enough that a stale "you haven't solved it" doesn't
// surface hours later after they already have.
const reminderTTL = 3600

// limaLoc is a deliberate duplicate of games' own Lima anchor, not an
// import of it — games now depends on this package (to send the
// solve-notification below), so importing games here would be a cycle.
// Peru's UTC-5 offset is a fixed fact (no DST since 1990), not something
// that drifts, so this tiny duplication is the pragmatic tradeoff versus
// fighting the cycle with a third shared package for two constants.
var limaLoc = time.FixedZone("America/Lima", -5*60*60)

// normalizeVAPIDSubject works around a webpush-go quirk: its VAPID JWT
// builder unconditionally prepends "mailto:" unless the subject already
// starts with "https:" — it never checks for an existing "mailto:" prefix.
// Our config's VAPID_SUBJECT is set (and documented, and every web-push
// tutorial's convention) as a full "mailto:foo@bar.com" string, so passing
// it straight through produced a malformed double-prefixed
// "mailto:mailto:foo@bar.com" JWT subject — confirmed via prod logs as a
// 403 BadJwtToken from Apple's push service (web.push.apple.com), the only
// platform tested against so far. Unconfirmed whether Chrome/Firefox's push
// services would have accepted or also rejected the same malformed value.
func normalizeVAPIDSubject(subject string) string {
	return strings.TrimPrefix(subject, "mailto:")
}

// NextLimaNoon returns the next occurrence of 12:00 Lima time — today's if
// it hasn't passed yet, otherwise tomorrow's.
func NextLimaNoon() time.Time {
	now := time.Now().In(limaLoc)
	noon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, limaLoc)
	if !now.Before(noon) {
		noon = noon.Add(24 * time.Hour)
	}
	return noon
}

// CheckUnsolvedAndNotify runs once — intended to be called right at Lima
// noon (see NextLimaNoon) — and sends a reminder push to every subscribed
// device belonging to a user who hasn't solved today's riddle yet. Fires
// once a day at a precise instant rather than polling a ticker, so there's
// no "did I already notify today" state to track.
func CheckUnsolvedAndNotify(db *sql.DB, vapidPublicKey, vapidPrivateKey, vapidSubject string) error {
	today := time.Now().In(limaLoc).Format("2006-01-02")

	rows, err := db.Query(`SELECT DISTINCT user_id FROM push_subscriptions`)
	if err != nil {
		return err
	}
	var userIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		userIDs = append(userIDs, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()

	for _, userID := range userIDs {
		var status string
		err := db.QueryRow(
			`SELECT rgs.status FROM riddle_game_sessions rgs
			 JOIN daily_riddles dr ON dr.id = rgs.current_riddle_id
			 WHERE rgs.team_id = $1 AND dr.published_on = $2`,
			userID, today,
		).Scan(&status)
		if err == nil && status == "solved" {
			continue // already done, nothing to remind them about
		}
		if err != nil && err != sql.ErrNoRows {
			log.Printf("push: check session status for user %d failed: %v", userID, err)
			continue
		}

		subRows, err := db.Query(
			`SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = $1`, userID,
		)
		if err != nil {
			log.Printf("push: load subscriptions for user %d failed: %v", userID, err)
			continue
		}
		var subs []webpush.Subscription
		for subRows.Next() {
			var s webpush.Subscription
			if err := subRows.Scan(&s.Endpoint, &s.Keys.P256dh, &s.Keys.Auth); err != nil {
				log.Printf("push: scan subscription for user %d failed: %v", userID, err)
				continue
			}
			subs = append(subs, s)
		}
		subRows.Close()

		payload := []byte(`{"title":"La Última Pregunta","body":"Todavía no resolviste la de hoy 🧩"}`)
		for _, sub := range subs {
			sendPush(db, sub, vapidPublicKey, vapidPrivateKey, vapidSubject, payload)
		}
	}
	return nil
}

// NotifyPartnerSolved pushes "your partner solved it" to every subscribed
// device belonging to anyone OTHER than solverUserID. Best-effort: the
// caller (games.SubmitRiddleGuess) fires this from a goroutine so a slow or
// failing push service can't add latency to the guess response, and every
// error here is logged, not propagated — a failed celebratory notification
// shouldn't be visible as an API error to the person who just solved it.
func NotifyPartnerSolved(db *sql.DB, vapidPublicKey, vapidPrivateKey, vapidSubject string, solverUserID int64, solverName string) {
	rows, err := db.Query(
		`SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id != $1`, solverUserID,
	)
	if err != nil {
		log.Printf("push: load partner subscriptions failed: %v", err)
		return
	}
	var subs []webpush.Subscription
	for rows.Next() {
		var s webpush.Subscription
		if err := rows.Scan(&s.Endpoint, &s.Keys.P256dh, &s.Keys.Auth); err != nil {
			log.Printf("push: scan partner subscription failed: %v", err)
			continue
		}
		subs = append(subs, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Printf("push: load partner subscriptions failed: %v", err)
		return
	}
	log.Printf("push: notifying %d partner subscription(s) that %s solved", len(subs), solverName)

	body, err := json.Marshal(map[string]string{
		"title": "La Última Pregunta",
		"body":  fmt.Sprintf("¡%s ya resolvió la de hoy! 🎉", solverName),
	})
	if err != nil {
		log.Printf("push: marshal solve notification failed: %v", err)
		return
	}
	for _, sub := range subs {
		sendPush(db, sub, vapidPublicKey, vapidPrivateKey, vapidSubject, body)
	}
}

// careActionBody has the "someone just did this" phrasing for each of the
// 5 care actions — a %s for the actor's name, a %s for the pet's name.
// Separate from petUnlockSchedule/petDeadlineSchedule's bodies, which are
// phrased as "it's time to do this" / "you missed doing this", not "your
// partner already did this".
var careActionBody = map[string]string{
	"bathe":     "%s bañó a %s 🛁",
	"breakfast": "%s le dio el desayuno a %s ☕",
	"lunch":     "%s le dio el almuerzo a %s 🍽️",
	"play":      "%s jugó con %s 🎾",
	"dinner":    "%s le dio la cena a %s 🌙",
}

// NotifyPartnerCareAction pushes "your partner just took care of the pet"
// to every subscribed device belonging to anyone OTHER than actorUserID.
// Same "fire from a goroutine, log-only on failure" contract as
// NotifyPartnerSolved — a failed celebratory notification shouldn't be
// visible as an API error to whoever just did the action. petName is passed
// in by the caller (pet.care already has it loaded) rather than queried
// here, unlike petDisplayName above — this fires from inside a request that
// already paid for that read, no reason to pay for it twice.
func NotifyPartnerCareAction(db *sql.DB, vapidPublicKey, vapidPrivateKey, vapidSubject string, actorUserID int64, actorName, action, petName string) {
	tmpl, ok := careActionBody[action]
	if !ok {
		log.Printf("push: unknown care action %q, skipping partner notification", action)
		return
	}
	name := petName
	if name == "" {
		name = "la mascota"
	}

	rows, err := db.Query(
		`SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id != $1`, actorUserID,
	)
	if err != nil {
		log.Printf("push: load partner subscriptions failed: %v", err)
		return
	}
	var subs []webpush.Subscription
	for rows.Next() {
		var s webpush.Subscription
		if err := rows.Scan(&s.Endpoint, &s.Keys.P256dh, &s.Keys.Auth); err != nil {
			log.Printf("push: scan partner subscription failed: %v", err)
			continue
		}
		subs = append(subs, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Printf("push: load partner subscriptions failed: %v", err)
		return
	}

	payload, err := json.Marshal(map[string]string{
		"title": name,
		"body":  fmt.Sprintf(tmpl, actorName, name),
	})
	if err != nil {
		log.Printf("push: marshal care action notification failed: %v", err)
		return
	}
	for _, sub := range subs {
		sendPush(db, sub, vapidPublicKey, vapidPrivateKey, vapidSubject, payload)
	}
}

// petReminderEvent is one scheduled instant this file might push a
// notification at — either a care action unlocking, or (below) its window's
// deadline passing.
type petReminderEvent struct {
	action string
	hour   int
	body   string
}

// petUnlockSchedule mirrors pet.careActions' unlockHour — duplicated here
// rather than importing the pet package. This file already avoids depending
// on games/pet Go packages, querying their tables directly via raw SQL
// instead (see CheckUnsolvedAndNotify above); this is the same tradeoff,
// just for a 5-row static table instead of a single Lima-offset constant.
var petUnlockSchedule = []petReminderEvent{
	{"bathe", 7, "Es hora del baño 🛁"},
	{"breakfast", 8, "Es hora del desayuno ☕"},
	{"lunch", 12, "Es hora del almuerzo 🍽️"},
	{"play", 16, "Es hora de jugar 🎾"},
	{"dinner", 19, "Es hora de la cena 🌙"},
}

// petDeadlineSchedule mirrors pet.careActions' deadlineHour — the "last
// call" half of pet.applyMissedWindowDecay's missed-window care_score
// penalty. Without this, that penalty was silent: care_score would drop the
// moment a window closed, but nobody knew until they happened to open the
// app. Fires at the same hour the penalty itself lands, so the copy can
// truthfully say it already cost something rather than just warning it's
// about to.
var petDeadlineSchedule = []petReminderEvent{
	{"bathe", 8, "Se pasó la hora del baño — le costó ánimo a la mascota 🛁⏰"},
	{"breakfast", 12, "Se pasó la hora del desayuno — le costó ánimo a la mascota ☕⏰"},
	{"lunch", 16, "Se pasó la hora del almuerzo — le costó ánimo a la mascota 🍽️⏰"},
	{"play", 19, "Se pasó la hora de jugar — le costó ánimo a la mascota 🎾⏰"},
	{"dinner", 22, "Se pasó la hora de la cena — le costó ánimo a la mascota 🌙⏰"},
}

// NextPetReminderTime returns whichever of the 10 scheduled pet-care events
// (5 unlocks + 5 deadlines) fires soonest — today's occurrence if still
// upcoming, otherwise tomorrow's. Same shape as NextLimaNoon, generalized
// from 1 fixed instant to whichever of 10 is closest.
func NextPetReminderTime() (event petReminderEvent, at time.Time) {
	now := time.Now().In(limaLoc)
	consider := func(e petReminderEvent) {
		candidate := time.Date(now.Year(), now.Month(), now.Day(), e.hour, 0, 0, 0, limaLoc)
		if !candidate.After(now) {
			candidate = candidate.Add(24 * time.Hour)
		}
		if at.IsZero() || candidate.Before(at) {
			event, at = e, candidate
		}
	}
	for _, e := range petUnlockSchedule {
		consider(e)
	}
	for _, e := range petDeadlineSchedule {
		consider(e)
	}
	return event, at
}

// petDisplayName reads pet_state.name for a team, raw SQL rather than
// importing the pet package (same tradeoff every other pet.* query in this
// file already takes — see petUnlockSchedule's comment). Falls back to
// "Nuestra mascota" for an unnamed pet or a team whose pet_state row
// doesn't exist yet — same fallback MascotaTab's own CardTitle uses
// (`pet?.name || 'Nuestra mascota'`), so a push title and the in-app title
// never disagree.
func petDisplayName(db *sql.DB, teamID int64) string {
	var name string
	if err := db.QueryRow(`SELECT name FROM pet_state WHERE team_id = $1`, teamID).Scan(&name); err != nil {
		return "Nuestra mascota"
	}
	if name == "" {
		return "Nuestra mascota"
	}
	return name
}

// CheckPetReminderAndNotify runs once — intended to be called right at
// event.hour (see NextPetReminderTime) — and pushes event.body to every
// subscribed device unless event.action is already done today. Unlike
// CheckUnsolvedAndNotify (per-user riddle progress), the pet is one shared
// state for the whole team, so there's a single done/not-done check and
// every subscriber gets notified, not a per-user loop. The same done-check
// covers both unlock and deadline events — an already-done action is
// equally pointless to remind about either way.
func CheckPetReminderAndNotify(db *sql.DB, event petReminderEvent, vapidPublicKey, vapidPrivateKey, vapidSubject string) error {
	teamID, err := team.ResolveTeamID(db)
	if err != nil {
		return err
	}
	today := time.Now().In(limaLoc).Format("2006-01-02")
	var done bool
	if err := db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM pet_care_log WHERE team_id = $1 AND action = $2 AND care_date = $3)`,
		teamID, event.action, today,
	).Scan(&done); err != nil {
		return err
	}
	if done {
		return nil // already taken care of, nothing to remind about
	}

	rows, err := db.Query(`SELECT endpoint, p256dh, auth FROM push_subscriptions`)
	if err != nil {
		return err
	}
	var subs []webpush.Subscription
	for rows.Next() {
		var s webpush.Subscription
		if err := rows.Scan(&s.Endpoint, &s.Keys.P256dh, &s.Keys.Auth); err != nil {
			rows.Close()
			return err
		}
		subs = append(subs, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// Was a hardcoded "Nuestra mascota" literal — ignored a rename entirely,
	// so a couple who'd renamed their pet still got reminders titled with
	// the generic fallback name forever. petDisplayName reads the real one.
	payload, err := json.Marshal(map[string]string{"title": petDisplayName(db, teamID), "body": event.body})
	if err != nil {
		return err
	}
	for _, sub := range subs {
		sendPush(db, sub, vapidPublicKey, vapidPrivateKey, vapidSubject, payload)
	}
	return nil
}

func sendPush(db *sql.DB, sub webpush.Subscription, vapidPublicKey, vapidPrivateKey, vapidSubject string, payload []byte) {
	resp, err := webpush.SendNotification(payload, &sub, &webpush.Options{
		Subscriber:      normalizeVAPIDSubject(vapidSubject),
		VAPIDPublicKey:  vapidPublicKey,
		VAPIDPrivateKey: vapidPrivateKey,
		TTL:             reminderTTL,
	})
	if err != nil {
		log.Printf("push: send failed for endpoint %s: %v", sub.Endpoint, err)
		return
	}
	defer resp.Body.Close()

	// Always log the outcome — a silent success looked identical to "never
	// attempted" in the logs, which made a real delivery failure
	// indistinguishable from this function never having run at all.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("push: sent to %s, status %d", sub.Endpoint, resp.StatusCode)
	} else {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("push: send to %s rejected, status %d: %s", sub.Endpoint, resp.StatusCode, string(body))
	}

	// 404/410 means the push service considers this subscription gone for
	// good (uninstalled, permissions revoked, browser data cleared) —
	// clean it up so future runs stop trying it.
	if resp.StatusCode == 404 || resp.StatusCode == 410 {
		if _, err := db.Exec(`DELETE FROM push_subscriptions WHERE endpoint = $1`, sub.Endpoint); err != nil {
			log.Printf("push: cleanup stale subscription failed: %v", err)
		}
	}
}
