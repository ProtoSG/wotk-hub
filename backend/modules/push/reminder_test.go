package push

import (
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

func TestNextPetActionTime(t *testing.T) {
	before := time.Now()
	action, next := NextPetActionTime()

	wantHours := map[string]int{"bathe": 7, "breakfast": 8, "lunch": 12, "play": 16, "dinner": 19}
	wantHour, ok := wantHours[action]
	if !ok {
		t.Fatalf("NextPetActionTime() action = %q, want one of bathe/breakfast/lunch/play/dinner", action)
	}
	if got := next.Hour(); got != wantHour {
		t.Errorf("NextPetActionTime() hour = %d, want %d for action %q", got, wantHour, action)
	}
	if got := next.Minute(); got != 0 {
		t.Errorf("NextPetActionTime() minute = %d, want 0", got)
	}
	if got := next.Location().String(); got != "America/Lima" {
		t.Errorf("NextPetActionTime() location = %s, want America/Lima", got)
	}
	if !next.After(before) {
		t.Errorf("NextPetActionTime() = %v, want a time after %v", next, before)
	}
	if until := next.Sub(before); until > 24*time.Hour {
		t.Errorf("NextPetActionTime() is %v away, want at most 24h", until)
	}

	// The 5 unlock hours span the whole day (7..19), so whichever one is
	// "next" should never be more than the largest gap between consecutive
	// hours away — a loose upper bound that'd catch NextPetActionTime
	// picking the wrong one entirely (e.g. always returning tomorrow's
	// bathe regardless of the current hour).
	if until := next.Sub(before); until > 12*time.Hour {
		t.Errorf("NextPetActionTime() is %v away, want at most 12h given hourly spread 7..19", until)
	}
}
