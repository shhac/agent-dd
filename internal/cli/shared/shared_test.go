package shared_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shhac/agent-dd/internal/cli/shared"
	"github.com/shhac/agent-dd/internal/mockdd/mockddtest"
	"github.com/shhac/agent-dd/internal/output"
)

func TestParseTimeRelative(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
		checkFn func(time.Time) bool
	}{
		{"now", false, func(t time.Time) bool { return time.Since(t) < 2*time.Second }},
		{"now-15m", false, func(t time.Time) bool {
			expected := time.Now().Add(-15 * time.Minute)
			return t.Sub(expected).Abs() < 2*time.Second
		}},
		{"now-1h", false, func(t time.Time) bool {
			expected := time.Now().Add(-1 * time.Hour)
			return t.Sub(expected).Abs() < 2*time.Second
		}},
		{"now-7d", false, func(t time.Time) bool {
			expected := time.Now().Add(-7 * 24 * time.Hour)
			return t.Sub(expected).Abs() < 2*time.Second
		}},
		{"now+1h", false, func(t time.Time) bool {
			expected := time.Now().Add(1 * time.Hour)
			return t.Sub(expected).Abs() < 2*time.Second
		}},
	}

	for _, tt := range tests {
		result, err := shared.ParseTime(tt.input)
		if tt.wantErr && err == nil {
			t.Errorf("ParseTime(%q): expected error", tt.input)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("ParseTime(%q): unexpected error: %v", tt.input, err)
		}
		if err == nil && tt.checkFn != nil && !tt.checkFn(result) {
			t.Errorf("ParseTime(%q): time %v didn't pass check", tt.input, result)
		}
	}
}

func TestParseTimeRFC3339(t *testing.T) {
	result, err := shared.ParseTime("2024-01-15T10:00:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("got %v, want %v", result, expected)
	}
}

func TestParseTimeUnixEpoch(t *testing.T) {
	result, err := shared.ParseTime("1705312800")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Unix(1705312800, 0)
	if !result.Equal(expected) {
		t.Errorf("got %v, want %v", result, expected)
	}
}

func TestParseTimeInvalid(t *testing.T) {
	_, err := shared.ParseTime("yesterday")
	if err == nil {
		t.Error("expected error for invalid time string")
	}
}

func TestParseTimeEmpty(t *testing.T) {
	result, err := shared.ParseTime("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsZero() {
		t.Errorf("expected zero time for empty string, got %v", result)
	}
}

func TestCursorPagination(t *testing.T) {
	t.Run("empty cursor returns nil", func(t *testing.T) {
		p := shared.CursorPagination("")
		if p != nil {
			t.Errorf("expected nil for empty cursor, got %+v", p)
		}
	})

	t.Run("non-empty cursor returns pagination", func(t *testing.T) {
		p := shared.CursorPagination("abc123")
		if p == nil {
			t.Fatal("expected non-nil pagination")
		}
		if !p.HasMore {
			t.Error("expected HasMore=true")
		}
		if p.NextCursor != "abc123" {
			t.Errorf("expected NextCursor='abc123', got %q", p.NextCursor)
		}
	})
}

func TestRequireFlag(t *testing.T) {
	t.Run("non-empty returns true", func(t *testing.T) {
		if !shared.RequireFlag("query", "some-value", "") {
			t.Error("expected true for non-empty value")
		}
	})

	t.Run("empty returns false", func(t *testing.T) {
		if shared.RequireFlag("query", "", "provide a query") {
			t.Error("expected false for empty value")
		}
	})
}

func TestSingleTag(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		result := shared.SingleTag("")
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("non-empty returns slice", func(t *testing.T) {
		result := shared.SingleTag("env:prod")
		if len(result) != 1 || result[0] != "env:prod" {
			t.Errorf("expected [env:prod], got %v", result)
		}
	})
}

func TestParseIntArg(t *testing.T) {
	t.Run("valid integer", func(t *testing.T) {
		val, ok := shared.ParseIntArg("monitor ID", "123")
		if !ok {
			t.Error("expected ok=true for valid integer")
		}
		if val != 123 {
			t.Errorf("expected 123, got %d", val)
		}
	})

	t.Run("invalid integer", func(t *testing.T) {
		val, ok := shared.ParseIntArg("monitor ID", "abc")
		if ok {
			t.Error("expected ok=false for invalid integer")
		}
		if val != 0 {
			t.Errorf("expected 0, got %d", val)
		}
	})
}

// Ensure output.Pagination is used correctly by CursorPagination.
var _ *output.Pagination = shared.CursorPagination("test")

func TestValidateLimitOrWriteErr(t *testing.T) {
	t.Run("at and below max pass without writing", func(t *testing.T) {
		for _, limit := range []int{0, 1, 50, shared.MaxSearchPageLimit} {
			stderr := mockddtest.CaptureStderr(t, func() {
				if !shared.ValidateLimitOrWriteErr(limit) {
					t.Errorf("limit %d should be allowed", limit)
				}
			})
			if stderr != "" {
				t.Errorf("limit %d should not write to stderr, got %q", limit, stderr)
			}
		}
	})

	t.Run("over max fails with agent-fixable hinted error", func(t *testing.T) {
		var ok bool
		stderr := mockddtest.CaptureStderr(t, func() {
			ok = shared.ValidateLimitOrWriteErr(shared.MaxSearchPageLimit + 1)
		})
		if ok {
			t.Fatal("limit over max should be rejected")
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &row); err != nil {
			t.Fatalf("expected JSON error on stderr, got %q (%v)", stderr, err)
		}
		if row["fixable_by"] != "agent" {
			t.Errorf("expected fixable_by=agent, got %v", row["fixable_by"])
		}
		if msg, _ := row["error"].(string); !strings.Contains(msg, "1000") {
			t.Errorf("expected error citing the 1000 max, got %q", msg)
		}
		if hint, _ := row["hint"].(string); !strings.Contains(hint, "cursor") {
			t.Errorf("expected hint pointing at cursor pagination, got %q", hint)
		}
	})
}
