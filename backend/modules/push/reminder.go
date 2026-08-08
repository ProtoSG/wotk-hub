package push

import (
	"database/sql"
	"log"
	"time"
	"workhub/modules/games"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// reminderTTL bounds how long a push service holds an undelivered
// notification before giving up — 1h is plenty for "the phone was offline
// for a bit", short enough that a stale "you haven't solved it" doesn't
// surface hours later after they already have.
const reminderTTL = 3600

// NextLimaNoon returns the next occurrence of 12:00 Lima time — today's if
// it hasn't passed yet, otherwise tomorrow's. Shares games.LimaLocation so
// this and the riddle's own day-boundary logic can never quietly disagree
// on what "noon" or "today" means (see games/handlers.go's comments on the
// timezone bugs that cost a full afternoon to track down).
func NextLimaNoon() time.Time {
	loc := games.LimaLocation()
	now := time.Now().In(loc)
	noon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc)
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
	today := games.TodayLima()

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

		for _, sub := range subs {
			sendReminder(db, sub, vapidPublicKey, vapidPrivateKey, vapidSubject)
		}
	}
	return nil
}

func sendReminder(db *sql.DB, sub webpush.Subscription, vapidPublicKey, vapidPrivateKey, vapidSubject string) {
	payload := []byte(`{"title":"La Última Pregunta","body":"Todavía no resolviste la de hoy 🧩"}`)
	resp, err := webpush.SendNotification(payload, &sub, &webpush.Options{
		Subscriber:      vapidSubject,
		VAPIDPublicKey:  vapidPublicKey,
		VAPIDPrivateKey: vapidPrivateKey,
		TTL:             reminderTTL,
	})
	if err != nil {
		log.Printf("push: send failed for endpoint %s: %v", sub.Endpoint, err)
		return
	}
	defer resp.Body.Close()

	// 404/410 means the push service considers this subscription gone for
	// good (uninstalled, permissions revoked, browser data cleared) —
	// clean it up so future runs stop trying it.
	if resp.StatusCode == 404 || resp.StatusCode == 410 {
		if _, err := db.Exec(`DELETE FROM push_subscriptions WHERE endpoint = $1`, sub.Endpoint); err != nil {
			log.Printf("push: cleanup stale subscription failed: %v", err)
		}
	}
}
