package shared_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shhac/agent-dd/internal/cli/shared"
	"github.com/shhac/agent-dd/internal/config"
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

func TestResolveOrgPrecedence(t *testing.T) {
	setupConfigDir(t)
	writeConfigFile(t, `{
  "default_org": "default-org",
  "organizations": {
    "default-org": {"site": "datadoghq.com"},
    "other-org": {"site": "datadoghq.eu"}
  },
  "settings": {}
}`)
	t.Setenv("DD_ORG", "env-org")

	if got, err := shared.ResolveOrg("explicit-org"); err != nil || got != "explicit-org" {
		t.Fatalf("explicit org = %q, %v; want explicit-org, nil", got, err)
	}
	if got, err := shared.ResolveOrg(""); err != nil || got != "env-org" {
		t.Fatalf("env org = %q, %v; want env-org, nil", got, err)
	}

	t.Setenv("DD_ORG", "")
	if got, err := shared.ResolveOrg(""); err != nil || got != "default-org" {
		t.Fatalf("default org = %q, %v; want default-org, nil", got, err)
	}
}

func TestResolveOrgMissingIncludesAvailableOrgs(t *testing.T) {
	setupConfigDir(t)
	writeConfigFile(t, `{
  "organizations": {"first": {}, "second": {}},
  "settings": {}
}`)

	_, err := shared.ResolveOrg("")
	if err == nil {
		t.Fatal("expected missing-org error")
	}
	if !strings.Contains(err.Error(), "no organization specified") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewClientFromOrgCredentialPrecedence(t *testing.T) {
	t.Run("DD_API_URL override uses env credentials", func(t *testing.T) {
		setupConfigDir(t)
		t.Setenv("DD_API_URL", "http://mock.example/api")
		t.Setenv("DD_API_KEY", "env-api")
		t.Setenv("DD_APP_KEY", "env-app")

		client, err := shared.NewClientFromOrg("ignored-org")
		if err != nil {
			t.Fatalf("NewClientFromOrg: %v", err)
		}
		assertClientFields(t, client, "http://mock.example/api", "env-api", "env-app")
	})

	t.Run("direct env credentials when no org", func(t *testing.T) {
		setupConfigDir(t)
		t.Setenv("DD_API_KEY", "env-api")
		t.Setenv("DD_APP_KEY", "env-app")
		t.Setenv("DD_SITE", "datadoghq.eu")

		client, err := shared.NewClientFromOrg("")
		if err != nil {
			t.Fatalf("NewClientFromOrg: %v", err)
		}
		assertClientFields(t, client, "https://api.datadoghq.eu/api", "env-api", "env-app")
	})

	t.Run("direct env credentials default site", func(t *testing.T) {
		setupConfigDir(t)
		t.Setenv("DD_API_KEY", "env-api")
		t.Setenv("DD_APP_KEY", "env-app")

		client, err := shared.NewClientFromOrg("")
		if err != nil {
			t.Fatalf("NewClientFromOrg: %v", err)
		}
		assertClientFields(t, client, "https://api.datadoghq.com/api", "env-api", "env-app")
	})

	t.Run("explicit org overrides env credentials", func(t *testing.T) {
		setupConfigDir(t)
		t.Setenv("DD_API_KEY", "env-api")
		t.Setenv("DD_APP_KEY", "env-app")
		writeConfigFile(t, `{
  "organizations": {"prod": {"site": "datadoghq.eu"}},
  "settings": {}
}`)
		writeCredentialsFile(t, `{"prod": {"api_key": "org-api", "app_key": "org-app"}}`)

		client, err := shared.NewClientFromOrg("prod")
		if err != nil {
			t.Fatalf("NewClientFromOrg: %v", err)
		}
		assertClientFields(t, client, "https://api.datadoghq.eu/api", "org-api", "org-app")
	})

	t.Run("DD_ORG resolves configured org", func(t *testing.T) {
		setupConfigDir(t)
		t.Setenv("DD_ORG", "prod")
		writeConfigFile(t, `{
  "organizations": {"prod": {"site": "us5.datadoghq.com"}},
  "settings": {}
}`)
		writeCredentialsFile(t, `{"prod": {"api_key": "org-api", "app_key": "org-app"}}`)

		client, err := shared.NewClientFromOrg("")
		if err != nil {
			t.Fatalf("NewClientFromOrg: %v", err)
		}
		assertClientFields(t, client, "https://api.us5.datadoghq.com/api", "org-api", "org-app")
	})

	t.Run("missing credentials", func(t *testing.T) {
		setupConfigDir(t)
		writeConfigFile(t, `{"organizations": {"prod": {}}, "settings": {}}`)

		_, err := shared.NewClientFromOrg("prod")
		if err == nil || !strings.Contains(err.Error(), `credentials for organization "prod" not found`) {
			t.Fatalf("expected missing credentials error, got %v", err)
		}
	})

	t.Run("empty API key", func(t *testing.T) {
		setupConfigDir(t)
		writeConfigFile(t, `{"organizations": {"prod": {}}, "settings": {}}`)
		writeCredentialsFile(t, `{"prod": {"api_key": "", "app_key": "org-app"}}`)

		_, err := shared.NewClientFromOrg("prod")
		if err == nil || !strings.Contains(err.Error(), `organization "prod" has no API key`) {
			t.Fatalf("expected empty API key error, got %v", err)
		}
	})
}

func setupConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	config.SetConfigDir(dir)
	t.Cleanup(func() { config.SetConfigDir("") })
	for _, env := range []string{"DD_API_URL", "DD_API_KEY", "DD_APP_KEY", "DD_SITE", "DD_ORG"} {
		t.Setenv(env, "")
	}
	return dir
}

func writeConfigFile(t *testing.T, body string) {
	t.Helper()
	if err := os.MkdirAll(config.ConfigDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config.ConfigDir(), "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	config.ClearCache()
}

func writeCredentialsFile(t *testing.T, body string) {
	t.Helper()
	if err := os.MkdirAll(config.ConfigDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config.ConfigDir(), "credentials.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertClientFields(t *testing.T, client any, wantBaseURL, wantAPIKey, wantAppKey string) {
	t.Helper()
	v := reflect.ValueOf(client).Elem()
	if got := v.FieldByName("baseURL").String(); got != wantBaseURL {
		t.Errorf("baseURL = %q, want %q", got, wantBaseURL)
	}
	if got := v.FieldByName("apiKey").String(); got != wantAPIKey {
		t.Errorf("apiKey = %q, want %q", got, wantAPIKey)
	}
	if got := v.FieldByName("appKey").String(); got != wantAppKey {
		t.Errorf("appKey = %q, want %q", got, wantAppKey)
	}
}
