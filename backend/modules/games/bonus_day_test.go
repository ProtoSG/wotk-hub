package games

import "testing"

func TestIsBonusDay(t *testing.T) {
	cases := []struct {
		date string // Lima calendar date
		want bool
	}{
		{"2026-08-09", true},  // Sunday
		{"2026-08-02", true},  // Sunday
		{"2026-08-08", false}, // Saturday
		{"2026-08-07", false}, // Friday
		{"2026-08-03", false}, // Monday
		{"not-a-date", false}, // malformed input — fails closed, not a bonus day
	}
	for _, tc := range cases {
		t.Run(tc.date, func(t *testing.T) {
			if got := isBonusDay(tc.date); got != tc.want {
				t.Errorf("isBonusDay(%q) = %v, want %v", tc.date, got, tc.want)
			}
		})
	}
}
