package push

import "testing"

func TestNormalizeVAPIDSubject(t *testing.T) {
	cases := []struct {
		name    string
		subject string
		want    string
	}{
		{"strips mailto prefix", "mailto:foo@bar.com", "foo@bar.com"},
		{"leaves https url untouched", "https://example.com", "https://example.com"},
		{"leaves bare email untouched", "foo@bar.com", "foo@bar.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeVAPIDSubject(tc.subject); got != tc.want {
				t.Errorf("normalizeVAPIDSubject(%q) = %q, want %q", tc.subject, got, tc.want)
			}
		})
	}
}
