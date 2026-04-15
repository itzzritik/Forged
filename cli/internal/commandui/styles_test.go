package commandui

import "testing"

func TestClampBodyWidth(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{name: "zero defaults to max", input: 0, want: 72},
		{name: "small widths clamp up", input: 20, want: 40},
		{name: "mid widths stay unchanged", input: 60, want: 60},
		{name: "large widths clamp down", input: 120, want: 72},
	}

	for _, tt := range tests {
		if got := ClampBodyWidth(tt.input); got != tt.want {
			t.Fatalf("%s: ClampBodyWidth(%d) = %d, want %d", tt.name, tt.input, got, tt.want)
		}
	}
}
