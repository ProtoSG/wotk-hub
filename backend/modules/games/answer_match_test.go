package games

import "testing"

func TestAnswerMatches(t *testing.T) {
	cases := []struct {
		name   string
		guess  string
		stored string
		want   bool
	}{
		{"exact match", "El agujero", "El agujero", true},
		{"bare noun without article", "agujero", "El agujero", true},
		{"case insensitive", "AGUJERO", "El agujero", true},
		{"accent insensitive", "camaleon", "El camaleón", true},
		{"wrong word", "el pozo", "El agujero", false},
		{"multi-answer, first alternative", "Radar", "Radar, nivel, kayak, oteo", true},
		{"multi-answer, middle alternative", "kayak", "Radar, nivel, kayak, oteo", true},
		{"multi-answer, no match", "canoa", "Radar, nivel, kayak, oteo", false},
		{"leading/trailing whitespace", "  El agujero  ", "El agujero", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := answerMatches(tc.guess, tc.stored); got != tc.want {
				t.Errorf("answerMatches(%q, %q) = %v, want %v", tc.guess, tc.stored, got, tc.want)
			}
		})
	}
}
