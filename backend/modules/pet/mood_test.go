package pet

import "testing"

func TestMoodFor(t *testing.T) {
	cases := []struct {
		careScore int
		want      string
	}{
		{100, "happy"},
		{75, "happy"},
		{74, "neutral"},
		{50, "neutral"},
		{49, "sad"},
		{25, "sad"},
		{24, "hungry"},
		{0, "hungry"},
	}
	for _, tc := range cases {
		if got := moodFor(tc.careScore); got != tc.want {
			t.Errorf("moodFor(%d) = %q, want %q", tc.careScore, got, tc.want)
		}
	}
}
