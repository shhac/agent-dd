package monitors_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/shhac/agent-dd/internal/cli/monitors"
	"github.com/shhac/agent-dd/internal/cli/shared"
	"github.com/shhac/agent-dd/internal/mockdd/mockddtest"
	"github.com/shhac/agent-dd/internal/output"
	"github.com/spf13/cobra"
)

// runMonitors drives the real cobra command tree so flag wiring, validation and
// output are all exercised the way an agent would hit them. The commands print
// via output.Print/WriteError to the process's own stdout and stderr, so both
// are redirected through a pipe for the duration of the call and returned
// together — an agent sees one stream of the two anyway.
func runMonitors(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()

	root := &cobra.Command{Use: "agent-dd"}
	monitors.Register(root, func() *shared.GlobalFlags {
		g := &shared.GlobalFlags{}
		g.Format = "json"
		return g
	})

	var cobraOut bytes.Buffer
	root.SetOut(&cobraOut)
	root.SetErr(&cobraOut)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(append([]string{"monitors"}, args...))

	var err error
	captured := mockddtest.CaptureCombined(t, func() {
		err = root.Execute()
		// libcli.Run is the single error sink in the real binary: a RunE that
		// returns gets rendered to stderr as a structured {error,fixable_by}
		// row. Mirror that here so tests see what an agent sees.
		if err != nil {
			output.WriteError(os.Stderr, err)
		}
	})

	return captured + cobraOut.String(), err
}

func TestCreateMonitorSendsTypedFlagsAsADefinition(t *testing.T) {
	var gotBody map[string]any
	var gotPath string
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 9001, "name": "CPU high"})
	})

	if _, err := runMonitors(t, "", "create",
		"--type", "metric alert",
		"--query", "avg(last_5m):avg:system.cpu.user{*} > 90",
		"--name", "CPU high",
		"--message", "page on-call",
		"--tag", "service:api",
		"--tag", "env:prod",
		"--priority", "2",
		"--threshold-critical", "90",
		"--threshold-warning", "80",
		"--renotify-interval", "30",
	); err != nil {
		t.Fatalf("create: %v", err)
	}

	if gotPath != "/api/v1/monitor" {
		t.Errorf("path = %s, want /api/v1/monitor", gotPath)
	}
	if gotBody["type"] != "metric alert" {
		t.Errorf("type = %v", gotBody["type"])
	}
	if gotBody["name"] != "CPU high" {
		t.Errorf("name = %v", gotBody["name"])
	}
	if gotBody["priority"] != float64(2) {
		t.Errorf("priority = %v, want 2", gotBody["priority"])
	}

	tags, ok := gotBody["tags"].([]any)
	if !ok || len(tags) != 2 {
		t.Fatalf("tags = %v, want two entries", gotBody["tags"])
	}

	opts, ok := gotBody["options"].(map[string]any)
	if !ok {
		t.Fatalf("options = %T, want map", gotBody["options"])
	}
	if opts["renotify_interval"] != float64(30) {
		t.Errorf("options.renotify_interval = %v, want 30", opts["renotify_interval"])
	}
	thresholds, ok := opts["thresholds"].(map[string]any)
	if !ok {
		t.Fatalf("thresholds = %T, want map", opts["thresholds"])
	}
	if thresholds["critical"] != float64(90) || thresholds["warning"] != float64(80) {
		t.Errorf("thresholds = %v, want critical 90 / warning 80", thresholds)
	}
}

func TestCreateMonitorRequiresTypeAndQuery(t *testing.T) {
	var called bool
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	out, err := runMonitors(t, "", "create", "--name", "no type or query")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if called {
		t.Error("a request was sent despite the definition being incomplete")
	}
	if !strings.Contains(out, "type") {
		t.Errorf("error should name the missing flag, got: %s", out)
	}
}

func TestCreateMonitorAcceptsBodyFromStdin(t *testing.T) {
	var gotBody map[string]any
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 9001})
	})

	stdin := `{"type":"log alert","query":"logs(\"status:error\").index(\"main\").rollup(\"count\").last(\"5m\") > 10","name":"from stdin","options":{"enable_logs_sample":true}}`
	if _, err := runMonitors(t, stdin, "create", "--body", "@-"); err != nil {
		t.Fatalf("create: %v", err)
	}

	if gotBody["name"] != "from stdin" {
		t.Errorf("name = %v, want from stdin", gotBody["name"])
	}
	opts, ok := gotBody["options"].(map[string]any)
	if !ok {
		t.Fatalf("options = %T, want map", gotBody["options"])
	}
	if opts["enable_logs_sample"] != true {
		t.Error("an option only expressible via --body did not survive")
	}
}

// Flags win over --body so an agent can take a template and adjust one field.
func TestCreateMonitorFlagsOverrideBody(t *testing.T) {
	var gotBody map[string]any
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 9001})
	})

	stdin := `{"type":"metric alert","query":"avg(last_5m):avg:system.cpu.user{*} > 90","name":"from body"}`
	if _, err := runMonitors(t, stdin, "create", "--body", "@-", "--name", "from flag"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if gotBody["name"] != "from flag" {
		t.Errorf("name = %v, want from flag", gotBody["name"])
	}
}

func TestCreateMonitorDryRunHitsValidateAndNeverWrites(t *testing.T) {
	var paths []string
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})

	out, err := runMonitors(t, "", "create",
		"--type", "metric alert",
		"--query", "avg(last_5m):avg:system.cpu.user{*} > 90",
		"--name", "CPU high",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("create --dry-run: %v", err)
	}

	for _, p := range paths {
		if p == "POST /api/v1/monitor" {
			t.Error("--dry-run sent the create request")
		}
	}
	if len(paths) != 1 || paths[0] != "POST /api/v1/monitor/validate" {
		t.Errorf("requests = %v, want exactly one POST to /api/v1/monitor/validate", paths)
	}
	if !strings.Contains(out, "valid") {
		t.Errorf("dry-run output should report validity, got: %s", out)
	}
}

func TestUpdateMonitorReadsModifiesWrites(t *testing.T) {
	var putBody map[string]any
	var methods []string
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{
				"id": 1001,
				"name": "original",
				"type": "metric alert",
				"query": "avg(last_5m):avg:system.cpu.user{*} > 90",
				"message": "untouched message",
				"restricted_roles": ["role-abc"],
				"overall_state": "alert",
				"created": "2025-11-01T08:00:00Z",
				"options": {"thresholds": {"critical": 90, "warning": 80}, "notify_no_data": true}
			}`))
		case http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&putBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 1001, "name": "renamed"})
		}
	})

	if _, err := runMonitors(t, "", "update", "1001", "--name", "renamed"); err != nil {
		t.Fatalf("update: %v", err)
	}

	if len(methods) != 2 || methods[0] != http.MethodGet || methods[1] != http.MethodPut {
		t.Fatalf("methods = %v, want GET then PUT", methods)
	}

	if putBody["name"] != "renamed" {
		t.Errorf("name = %v, want renamed", putBody["name"])
	}
	if putBody["message"] != "untouched message" {
		t.Errorf("message = %v — an untouched field was lost", putBody["message"])
	}
	if _, ok := putBody["restricted_roles"]; !ok {
		t.Error("restricted_roles was dropped — the clobbering bug this design prevents")
	}

	// Server-owned fields must not be echoed back — under either spelling.
	// Reads normalise `overall_state` to `status`, so stripping only the raw
	// name would let the state survive into the body.
	for _, readOnly := range []string{"overall_state", "status", "created"} {
		if _, present := putBody[readOnly]; present {
			t.Errorf("read-only field %q was written back", readOnly)
		}
	}

	opts, ok := putBody["options"].(map[string]any)
	if !ok {
		t.Fatalf("options = %T, want map", putBody["options"])
	}
	if opts["notify_no_data"] != true {
		t.Error("options.notify_no_data was lost")
	}
}

func TestUpdateMonitorMergesOneOptionWithoutClearingSiblings(t *testing.T) {
	var putBody map[string]any
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{
				"id": 1001, "name": "mon", "type": "metric alert", "query": "q",
				"options": {"thresholds": {"critical": 90, "warning": 80}, "notify_no_data": true, "evaluation_delay": 300}
			}`))
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&putBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1001})
	})

	if _, err := runMonitors(t, "", "update", "1001", "--renotify-interval", "30"); err != nil {
		t.Fatalf("update: %v", err)
	}

	opts := putBody["options"].(map[string]any)
	if opts["renotify_interval"] != float64(30) {
		t.Errorf("renotify_interval = %v, want 30", opts["renotify_interval"])
	}
	if opts["evaluation_delay"] != float64(300) {
		t.Error("evaluation_delay was cleared by an unrelated option change")
	}
	thresholds := opts["thresholds"].(map[string]any)
	if thresholds["critical"] != float64(90) || thresholds["warning"] != float64(80) {
		t.Errorf("thresholds = %v — cleared by an unrelated option change", thresholds)
	}
}

func TestUpdateMonitorReportsADiff(t *testing.T) {
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":1001,"name":"before","type":"metric alert","query":"q","options":{"thresholds":{"critical":90}}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1001})
	})

	out, err := runMonitors(t, "", "update", "1001", "--name", "after", "--threshold-critical", "95")
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	changes, ok := result["changes"].(map[string]any)
	if !ok {
		t.Fatalf("output has no changes map: %s", out)
	}
	name, ok := changes["name"].(map[string]any)
	if !ok {
		t.Fatalf("changes has no name entry: %s", out)
	}
	if name["from"] != "before" || name["to"] != "after" {
		t.Errorf("name change = %v, want before -> after", name)
	}
	if _, ok := changes["options.thresholds.critical"]; !ok {
		t.Errorf("expected a dotted path for the nested threshold change, got %v", changes)
	}
	if _, ok := changes["query"]; ok {
		t.Error("query did not change and must not appear in the diff")
	}
}

func TestUpdateMonitorRequiresAtLeastOneChange(t *testing.T) {
	var called bool
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	out, err := runMonitors(t, "", "update", "1001")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if called {
		t.Error("a request was sent with nothing to change")
	}
	if !strings.Contains(out, "error") {
		t.Errorf("expected an error, got: %s", out)
	}
}

func TestUpdateMonitorDryRunValidatesAgainstTheExistingID(t *testing.T) {
	var paths []string
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":1001,"name":"mon","type":"metric alert","query":"q"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})

	if _, err := runMonitors(t, "", "update", "1001", "--name", "renamed", "--dry-run"); err != nil {
		t.Fatalf("update --dry-run: %v", err)
	}

	for _, p := range paths {
		if strings.HasPrefix(p, "PUT ") {
			t.Errorf("--dry-run issued a write: %s", p)
		}
	}
	if !containsPath(paths, "POST /api/v1/monitor/1001/validate") {
		t.Errorf("requests = %v, want a POST to the per-ID validate endpoint", paths)
	}
}

func TestDeleteMonitorRefusesWithoutYes(t *testing.T) {
	var called bool
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	out, err := runMonitors(t, "", "delete", "1001")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if called {
		t.Error("delete issued a request without --yes")
	}
	if !strings.Contains(out, "--yes") {
		t.Errorf("error should name --yes, got: %s", out)
	}
}

func TestDeleteMonitorWithYes(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{"deleted_monitor_id": 1001})
	})

	if _, err := runMonitors(t, "", "delete", "1001", "--yes"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/v1/monitor/1001" {
		t.Errorf("%s %s, want DELETE /api/v1/monitor/1001", gotMethod, gotPath)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want empty — force must not be implied by --yes", gotQuery)
	}
}

// A referenced monitor's refusal must arrive as an agent-fixable error naming
// --force, not as an opaque 400.
func TestDeleteMonitorReferencedSuggestsForce(t *testing.T) {
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []string{"Monitor is referenced in an SLO and cannot be deleted"},
		})
	})

	out, err := runMonitors(t, "", "delete", "1001", "--yes")
	if err == nil {
		t.Fatal("expected the refusal to surface as an error")
	}
	if !strings.Contains(out, "--force") {
		t.Errorf("error should hint at --force, got: %s", out)
	}
	if !strings.Contains(out, `"fixable_by":"agent"`) {
		t.Errorf("refusal should classify as agent-fixable, got: %s", out)
	}
}

func TestDeleteMonitorWithForce(t *testing.T) {
	var gotForce string
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotForce = r.URL.Query().Get("force")
		_ = json.NewEncoder(w).Encode(map[string]any{"deleted_monitor_id": 1001})
	})

	if _, err := runMonitors(t, "", "delete", "1001", "--yes", "--force"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if gotForce != "true" {
		t.Errorf("force = %q, want true", gotForce)
	}
}

func containsPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}
