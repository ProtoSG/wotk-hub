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
