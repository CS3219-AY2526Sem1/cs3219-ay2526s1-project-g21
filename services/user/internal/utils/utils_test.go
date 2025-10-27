package utils

import "testing"

func TestIsPasswordValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{name: "too short", password: "abc!", want: false},
		{name: "missing special", password: "abcdefgh", want: false},
		{name: "valid", password: "Abcdefg!", want: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsPasswordValid(tc.password); got != tc.want {
				t.Fatalf("IsPasswordValid(%q) = %v, want %v", tc.password, got, tc.want)
			}
		})
	}
}
