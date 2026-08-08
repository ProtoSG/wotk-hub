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

// petActionSchedule mirrors pet.careActions' unlock hours — duplicated here
// rather than importing the pet package. This file already avoids depending
// on games/pet Go packages, querying their tables directly via raw SQL
// instead (see CheckUnsolvedAndNotify above); this is the same tradeoff,
// just for a 5-row static table instead of a single Lima-offset constant.
var petActionSchedule = []struct {
	action string
	hour   int
	body   string
}{
	{"bathe", 7, "Es hora del baño 🛁"},
	{"breakfast", 8, "Es hora del desayuno ☕"},
	{"lunch", 12, "Es hora del almuerzo 🍽️"},
	{"play", 16, "Es hora de jugar 🎾"},
	{"dinner", 19, "Es hora de la cena 🌙"},
}

// NextPetActionTime returns whichever of the 5 pet care actions unlocks
// soonest — today's occurrence if still upcoming, otherwise tomorrow's.
// Same shape as NextLimaNoon, generalized from 1 fixed instant to whichever
// of 5 is closest.
func NextPetActionTime() (action string, at time.Time) {
	now := time.Now().In(limaLoc)
	for _, a := range petActionSchedule {
		candidate := time.Date(now.Year(), now.Month(), now.Day(), a.hour, 0, 0, 0, limaLoc)
		if !candidate.After(now) {
			candidate = candidate.Add(24 * time.Hour)
		}
		if action == "" || candidate.Before(at) {
			action, at = a.action, candidate
		}
	}
	return action, at
}

// CheckPetActionAndNotify runs once — intended to be called right at the
// unlock hour for `action` (see NextPetActionTime) — and pushes a reminder
// to every subscribed device unless that action is already done today.
// Unlike CheckUnsolvedAndNotify (per-user riddle progress), the pet is one
// shared state for the whole team, so there's a single done/not-done check
// and every subscriber gets notified, not a per-user loop.
func CheckPetActionAndNotify(db *sql.DB, action, vapidPublicKey, vapidPrivateKey, vapidSubject string) error {
	var body string
	for _, a := range petActionSchedule {
		if a.action == action {
			body = a.body
			break
		}
	}
	if body == "" {
		return fmt.Errorf("push: unknown pet action %q", action)
	}

	teamID, err := team.ResolveTeamID(db)
	if err != nil {
		return err
	}
	today := time.Now().In(limaLoc).Format("2006-01-02")
	var done bool
	if err := db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM pet_care_log WHERE team_id = $1 AND action = $2 AND care_date = $3)`,
		teamID, action, today,
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

	payload, err := json.Marshal(map[string]string{"title": "Nuestra mascota", "body": body})
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
