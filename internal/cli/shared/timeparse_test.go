package shared_test

import (
	"testing"
	"time"

	"github.com/shhac/agent-dd/internal/cli/shared"
)

func TestParseTimeRange(t *testing.T) {
	fromTime, toTime, err := shared.ParseTimeRange("now-1h", "now")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if time.Since(fromTime) < 59*time.Minute || time.Since(fromTime) > 61*time.Minute {
		t.Errorf("fromTime not ~1h ago: %v", fromTime)
	}
	if time.Since(toTime) > 2*time.Second {
		t.Errorf("toTime not ~now: %v", toTime)
	}
}

func TestParseTimeRangeDefaults(t *testing.T) {
	fromTime, toTime, err := shared.ParseTimeRange("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if time.Since(fromTime) < 59*time.Minute || time.Since(fromTime) > 61*time.Minute {
		t.Errorf("default fromTime not ~1h ago: %v", fromTime)
	}
	if time.Since(toTime) > 2*time.Second {
		t.Errorf("default toTime not ~now: %v", toTime)
	}
}

func TestParseTimeRangeFromError(t *testing.T) {
	_, _, err := shared.ParseTimeRange("garbage", "now")
	if err == nil {
		t.Error("expected error for invalid from")
	}
}

func TestParseTimeRangeToError(t *testing.T) {
	_, _, err := shared.ParseTimeRange("now-1h", "garbage")
	if err == nil {
		t.Error("expected error for invalid to")
	}
}

func TestParseTimeRelativeWeek(t *testing.T) {
	result, err := shared.ParseTime("now-1w")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().Add(-7 * 24 * time.Hour)
	if result.Sub(expected).Abs() > 2*time.Second {
		t.Errorf("ParseTime(now-1w): got %v, want ~%v", result, expected)
	}
}

func TestParseTimeRelativeSeconds(t *testing.T) {
	result, err := shared.ParseTime("now-30s")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().Add(-30 * time.Second)
	if result.Sub(expected).Abs() > 2*time.Second {
		t.Errorf("ParseTime(now-30s): got %v, want ~%v", result, expected)
	}
}

func TestParseTimeRelativeErrors(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"now!1h", "invalid operator"},
		{"now-s", "missing number"},
		{"now-xm", "non-numeric value"},
		{"now-1z", "invalid unit"},
	}
	for _, tt := range tests {
		_, err := shared.ParseTime(tt.input)
		if err == nil {
			t.Errorf("ParseTime(%q): expected error for %s", tt.input, tt.desc)
		}
	}
}
