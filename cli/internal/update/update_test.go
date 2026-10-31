package update

import "testing"

func TestAvailable(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"0.1.44", "0.1.45", true},
		{"0.1.44", "0.2.0", true},
		{"0.1.9", "0.1.10", true},
		{"0.9.0", "1.0.0", true},
		{"0.1.45", "0.1.45", false},
		{"0.1.46", "0.1.45", false},
		{"dev", "0.1.45", false},
		{"0.1.44", "", false},
		{"0.1.44", "0.2.0-beta.1", false},
		{"0.1.44", "<html>", false},
		{"v0.1.44", "0.1.45", false},
	}
	for _, c := range cases {
		if got := Available(c.current, c.latest); got != c.want {
			t.Errorf("Available(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}
