package config

import "testing"

func TestNormalizePPOMode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "active", in: "active", want: "active"},
		{name: "shadow uppercase", in: "SHADOW", want: "shadow"},
		{name: "fallback spaced", in: " fallback ", want: "fallback"},
		{name: "empty defaults active", in: "", want: "active"},
		{name: "invalid defaults active", in: "unknown", want: "active"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizePPOMode(tt.in); got != tt.want {
				t.Fatalf("normalizePPOMode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
