package push

import (
	"strconv"
	"testing"
	"time"
)

func TestNextLimaNoon(t *testing.T) {
	before := time.Now()
	next := NextLimaNoon()

	if got := next.Hour(); got != 12 {
		t.Errorf("NextLimaNoon() hour = %d, want 12", got)
	}
	if got := next.Minute(); got != 0 {
		t.Errorf("NextLimaNoon() minute = %d, want 0", got)
	}
	if got := next.Location().String(); got != "America/Lima" {
		t.Errorf("NextLimaNoon() location = %s, want America/Lima", got)
	}
	if !next.After(before) {
		t.Errorf("NextLimaNoon() = %v, want a time after %v", next, before)
	}
	if until := next.Sub(before); until > 24*time.Hour {
		t.Errorf("NextLimaNoon() is %v away, want at most 24h", until)
	}
}

func TestNextPetReminderTime(t *testing.T) {
	before := time.Now()
	event, next := NextPetReminderTime()

	// Valid (action, hour) pairs across BOTH schedules — unlock and
	// deadline. Unlike the single-schedule version this replaced, the same
	// action now has two legitimate hours (e.g. "bathe" at unlock hour 7 or
	// deadline hour 8), so a simple map[string]int no longer captures every
	// valid combination.
	valid := map[string]bool{}
	for _, e := range petUnlockSchedule {
		valid[e.action+":"+strconv.Itoa(e.hour)] = true
	}
	for _, e := range petDeadlineSchedule {
		valid[e.action+":"+strconv.Itoa(e.hour)] = true
	}
	gotHour := next.Hour()
	if !valid[event.action+":"+strconv.Itoa(gotHour)] {
		t.Fatalf("NextPetReminderTime() = (action %q, hour %d), want a valid unlock/deadline pair", event.action, gotHour)
	}
	if got := next.Minute(); got != 0 {
		t.Errorf("NextPetReminderTime() minute = %d, want 0", got)
	}
	if got := next.Location().String(); got != "America/Lima" {
		t.Errorf("NextPetReminderTime() location = %s, want America/Lima", got)
	}
	if !next.After(before) {
		t.Errorf("NextPetReminderTime() = %v, want a time after %v", next, before)
	}
	if until := next.Sub(before); until > 24*time.Hour {
		t.Errorf("NextPetReminderTime() is %v away, want at most 24h", until)
	}

	// The combined unlock+deadline hours span the whole day (7..22), so
	// whichever one is "next" should never be more than the largest gap
	// between consecutive hours away — a loose upper bound that'd catch
	// NextPetReminderTime picking the wrong one entirely (e.g. always
	// returning tomorrow's bathe regardless of the current hour).
	if until := next.Sub(before); until > 15*time.Hour {
		t.Errorf("NextPetReminderTime() is %v away, want at most 15h given hourly spread 7..22", until)
	}
}
