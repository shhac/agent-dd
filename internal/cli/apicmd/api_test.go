package apicmd_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/cli/apicmd"
	"github.com/shhac/agent-dd/internal/cli/shared"
	"github.com/shhac/agent-dd/internal/mockdd/mockddtest"
)

func newCmd() *cobra.Command {
	root := &cobra.Command{Use: "agent-dd"}
	g := &shared.GlobalFlags{Format: "json"}
	apicmd.Register(root, func() *shared.GlobalFlags { return g })
	return root
}

// run executes the api command with args, returning captured stdout and stderr.
func run(t *testing.T, args ...string) (stdout, stderr string) {
	t.Helper()
	root := newCmd()
	root.SetArgs(append([]string{"api"}, args...))
	stderr = mockddtest.CaptureStderr(t, func() {
		stdout = mockddtest.CaptureStdout(t, func() {
			if err := root.Execute(); err != nil {
				// command errors are surfaced via stderr JSON, not returned;
				// a returned error still shouldn't fail the test harness here.
				_ = err
			}
		})
	})
	return stdout, stderr
}

// failIfCalled installs a client factory that fails the test if the command
// ever resolves a client — used to prove a guard short-circuits before any
// network setup.
func failIfCalled(t *testing.T) {
	t.Helper()
	shared.ClientFactory = func() (*api.Client, error) {
		t.Fatal("client resolved — expected the command to short-circuit first")
		return nil, nil
	}
	t.Cleanup(func() { shared.ClientFactory = nil })
}

func TestAPIGetPassthrough(t *testing.T) {
	srv := shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/apm/services" {
			t.Errorf("expected path /api/v2/apm/services, got %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"attributes":{"services":["web"]}}}`))
	})
	_ = srv

	stdout, stderr := run(t, "/v2/apm/services")
	if stderr != "" {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout not JSON: %q (%v)", stdout, err)
	}
	if _, ok := got["data"]; !ok {
		t.Errorf("expected response passthrough with data key, got %v", got)
	}
}

// A leading /api is stripped so callers can paste either form of the path.
func TestAPIPathNormalization(t *testing.T) {
	for _, in := range []string{"/v2/foo", "/api/v2/foo", "v2/foo"} {
		shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v2/foo" {
				t.Errorf("input %q: expected /api/v2/foo, got %s", in, r.URL.Path)
			}
			_, _ = w.Write([]byte(`{}`))
		})
		_, stderr := run(t, in)
		if stderr != "" {
			t.Errorf("input %q: unexpected stderr %s", in, stderr)
		}
	}
}

func TestAPIQueryParamsAppended(t *testing.T) {
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("filter[env]"); got != "production" {
			t.Errorf("expected filter[env]=production, got %q", got)
		}
		if got := r.URL.Query().Get("window"); got != "3600" {
			t.Errorf("expected window=3600, got %q", got)
		}
		_, _ = w.Write([]byte(`{}`))
	})
	_, stderr := run(t, "/v2/metrics", "--query", "filter[env]=production", "--query", "window=3600")
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
}

// POST to a search/aggregate path is treated as a read — no --allow-write — and
// the JSON body is forwarded with the JSON content type.
func TestAPIPostSearchBodyForwarded(t *testing.T) {
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("body not JSON: %q", body)
		}
		if parsed["x"] != float64(1) {
			t.Errorf("expected body x=1, got %v", parsed["x"])
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	_, stderr := run(t, "POST", "/v2/spans/analytics/aggregate", "--body", `{"x":1}`)
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
}

func TestAPIWriteGuardBlocksMutationWithoutFlag(t *testing.T) {
	failIfCalled(t)
	for _, tc := range [][]string{
		{"DELETE", "/v1/monitor/1"},
		{"PATCH", "/v1/monitor/1", "--body", `{"x":1}`},
		{"POST", "/v2/incidents", "--body", `{"x":1}`},
	} {
		_, stderr := run(t, tc...)
		var row map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &row); err != nil {
			t.Fatalf("%v: expected JSON error, got %q", tc, stderr)
		}
		if row["fixable_by"] != "agent" {
			t.Errorf("%v: expected fixable_by=agent, got %v", tc, row["fixable_by"])
		}
		if hint, _ := row["hint"].(string); !strings.Contains(hint, "allow-write") {
			t.Errorf("%v: expected hint to mention --allow-write, got %q", tc, hint)
		}
	}
}

func TestAPIWriteGuardAllowsWithFlag(t *testing.T) {
	hit := false
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		hit = true
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		_, _ = w.Write([]byte(`{}`))
	})
	_, stderr := run(t, "DELETE", "/v1/monitor/1", "--allow-write")
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !hit {
		t.Error("expected the DELETE to reach the server with --allow-write")
	}
}

// --print-request previews without sending and redacts credentials — even for
// a mutating request, which it may preview without --allow-write.
func TestAPIPrintRequestRedactsAndDoesNotSend(t *testing.T) {
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("--print-request must not send a request")
	})
	stdout, stderr := run(t, "POST", "/v2/incidents", "--body", `{"x":1}`, "--print-request")
	if stderr != "" {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
	var preview struct {
		Method  string            `json:"method"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Body    json.RawMessage   `json:"body"`
	}
	if err := json.Unmarshal([]byte(stdout), &preview); err != nil {
		t.Fatalf("preview not JSON: %q (%v)", stdout, err)
	}
	if preview.Method != "POST" {
		t.Errorf("expected method POST, got %q", preview.Method)
	}
	if !strings.HasSuffix(preview.URL, "/api/v2/incidents") {
		t.Errorf("expected URL ending /api/v2/incidents, got %q", preview.URL)
	}
	for _, h := range []string{"DD-API-KEY", "DD-APPLICATION-KEY"} {
		v := preview.Headers[h]
		if strings.Contains(v, "test-") || !strings.Contains(v, "set") {
			t.Errorf("header %s should be redacted, got %q", h, v)
		}
	}
}

func TestAPIInvalidBodyErrorsBeforeNetwork(t *testing.T) {
	failIfCalled(t)
	_, stderr := run(t, "POST", "/v2/spans/analytics/aggregate", "--body", "not json")
	var row map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &row); err != nil {
		t.Fatalf("expected JSON error, got %q", stderr)
	}
	if msg, _ := row["error"].(string); !strings.Contains(msg, "valid JSON") {
		t.Errorf("expected invalid-JSON error, got %q", msg)
	}
}

func TestAPIBodyFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "body.json")
	if err := os.WriteFile(path, []byte(`{"from":"file"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(body, &parsed)
		if parsed["from"] != "file" {
			t.Errorf("expected body loaded from file, got %v", parsed)
		}
		_, _ = w.Write([]byte(`{}`))
	})
	_, stderr := run(t, "POST", "/v2/spans/analytics/aggregate", "--body", "@"+path)
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
}
