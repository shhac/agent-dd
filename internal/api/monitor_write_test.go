package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-dd/internal/api"
	agenterrors "github.com/shhac/agent-dd/internal/errors"
)

// These tests assert request shape — path, method, body — against bespoke
// handlers. Round-trip behaviour against the canonical fixture shapes is
// covered by the mockdd integration tests instead; the split is the convention
// documented in mockddtest's package comment.

func TestCreateMonitorPostsTheWholeDefinition(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/monitor" {
			t.Errorf("path = %s, want /api/v1/monitor", r.URL.Path)
		}
		if got := r.Header.Get("DD-API-KEY"); got != "key" {
			t.Errorf("DD-API-KEY = %q, want key", got)
		}
		if got := r.Header.Get("DD-APPLICATION-KEY"); got != "app" {
			t.Errorf("DD-APPLICATION-KEY = %q, want app", got)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 4242, "name": "created"})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	definition := map[string]any{
		"type":    "metric alert",
		"query":   "avg(last_5m):avg:system.cpu.user{*} > 90",
		"name":    "CPU high",
		"options": map[string]any{"thresholds": map[string]any{"critical": 90}},
	}

	created, err := client.CreateMonitor(context.Background(), definition)
	if err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	if gotBody["type"] != "metric alert" {
		t.Errorf("body type = %v, want metric alert", gotBody["type"])
	}
	if gotBody["query"] != "avg(last_5m):avg:system.cpu.user{*} > 90" {
		t.Errorf("body query = %v", gotBody["query"])
	}
	opts, ok := gotBody["options"].(map[string]any)
	if !ok {
		t.Fatalf("body options = %T, want nested object", gotBody["options"])
	}
	if _, hasThresholds := opts["thresholds"]; !hasThresholds {
		t.Error("nested options.thresholds did not survive marshalling")
	}
	if created["name"] != "created" {
		t.Errorf("returned monitor = %v, want the decoded response", created)
	}
}

func TestGetMonitorRawPreservesUnmodelledFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/monitor/1001" {
			t.Errorf("path = %s, want /api/v1/monitor/1001", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"id": 1001,
			"name": "mon",
			"restricted_roles": ["role-abc"],
			"options": {"thresholds": {"critical": 90}, "some_future_option": "keep me"}
		}`))
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	monitor, err := client.GetMonitorRaw(context.Background(), 1001)
	if err != nil {
		t.Fatalf("GetMonitorRaw: %v", err)
	}

	// The whole point of the raw accessor: fields the CLI has no struct field
	// for must still be readable, because update writes them back.
	if _, ok := monitor["restricted_roles"]; !ok {
		t.Error("restricted_roles was dropped")
	}
	opts, ok := monitor["options"].(map[string]any)
	if !ok {
		t.Fatalf("options = %T, want map", monitor["options"])
	}
	if opts["some_future_option"] != "keep me" {
		t.Error("an option this CLI does not model was dropped")
	}
}

func TestUpdateMonitorPutsToTheIDPath(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/api/v1/monitor/1001" {
			t.Errorf("path = %s, want /api/v1/monitor/1001", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1001, "name": "renamed"})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	updated, err := client.UpdateMonitor(context.Background(), 1001, map[string]any{
		"name":             "renamed",
		"restricted_roles": []any{"role-abc"},
	})
	if err != nil {
		t.Fatalf("UpdateMonitor: %v", err)
	}

	if gotBody["name"] != "renamed" {
		t.Errorf("body name = %v, want renamed", gotBody["name"])
	}
	if _, ok := gotBody["restricted_roles"]; !ok {
		t.Error("unmodelled field was not written back — this is the clobbering bug the design exists to prevent")
	}
	if updated["name"] != "renamed" {
		t.Errorf("returned monitor = %v", updated)
	}
}

func TestDeleteMonitorWithoutForce(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/api/v1/monitor/1001" {
			t.Errorf("path = %s, want /api/v1/monitor/1001", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{"deleted_monitor_id": 1001})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	if err := client.DeleteMonitor(context.Background(), 1001, false); err != nil {
		t.Fatalf("DeleteMonitor: %v", err)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want empty — force must not be sent unless asked for", gotQuery)
	}
}

func TestDeleteMonitorWithForce(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("force")
		_ = json.NewEncoder(w).Encode(map[string]any{"deleted_monitor_id": 1001})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	if err := client.DeleteMonitor(context.Background(), 1001, true); err != nil {
		t.Fatalf("DeleteMonitor: %v", err)
	}
	if gotQuery != "true" {
		t.Errorf("force = %q, want true", gotQuery)
	}
}

func TestValidateMonitorPostsToValidateEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	if err := client.ValidateMonitor(context.Background(), map[string]any{"type": "metric alert"}); err != nil {
		t.Fatalf("ValidateMonitor: %v", err)
	}
	if gotPath != "/api/v1/monitor/validate" {
		t.Errorf("path = %s, want /api/v1/monitor/validate", gotPath)
	}
}

func TestValidateExistingMonitorPostsToIDValidateEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	if err := client.ValidateExistingMonitor(context.Background(), 1001, map[string]any{"name": "x"}); err != nil {
		t.Fatalf("ValidateExistingMonitor: %v", err)
	}
	if gotPath != "/api/v1/monitor/1001/validate" {
		t.Errorf("path = %s, want /api/v1/monitor/1001/validate", gotPath)
	}
}

// A malformed query is the single most likely failure an agent will hit when
// composing a monitor, so it must classify as agent-fixable rather than
// bubbling up as an opaque failure.
func TestValidateMonitorClassifiesBadQueryAsAgentFixable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": []string{"The value provided for parameter 'query' is invalid"}})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	err := client.ValidateMonitor(context.Background(), map[string]any{"query": "nonsense"})
	if err == nil {
		t.Fatal("expected an error for an invalid query")
	}
	var aerr *agenterrors.APIError
	if !errors.As(err, &aerr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if aerr.FixableBy != agenterrors.FixableByAgent {
		t.Errorf("fixable_by = %s, want agent", aerr.FixableBy)
	}
}

func TestCreateMonitorClassifiesMissingWritePermissionAsHumanFixable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": []string{"Forbidden: monitors_write required"}})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	_, err := client.CreateMonitor(context.Background(), map[string]any{"type": "metric alert"})
	if err == nil {
		t.Fatal("expected an error for a 403")
	}
	var aerr *agenterrors.APIError
	if !errors.As(err, &aerr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if aerr.FixableBy != agenterrors.FixableByHuman {
		t.Errorf("fixable_by = %s, want human", aerr.FixableBy)
	}
}
