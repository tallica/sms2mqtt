package modem

import "testing"

func TestSignalLevel(t *testing.T) {
	tests := []struct {
		dbm  int
		want string
	}{
		{-50, "excellent"},
		{-73, "excellent"},
		{-74, "good"},
		{-83, "good"},
		{-84, "fair"},
		{-93, "fair"},
		{-94, "poor"},
		{-120, "poor"},
	}
	for _, tt := range tests {
		if got := SignalLevel(tt.dbm); got != tt.want {
			t.Errorf("SignalLevel(%d) = %q, want %q", tt.dbm, got, tt.want)
		}
	}
}
