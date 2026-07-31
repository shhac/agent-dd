package mockdd_test

import (
	"context"
	"errors"
	"testing"

	agenterrors "github.com/shhac/agent-dd/internal/errors"
	"github.com/shhac/agent-dd/internal/mockdd/mockddtest"
)

// Round-trips against the canonical mock. These assert that a write is
// actually observable afterwards — the property a hand-stubbed handler can
// fake and a real store cannot.

func TestMockddCreateMonitorIsReadableAfterwards(t *testing.T) {
	client := mockddtest.NewTestClient(t)
	ctx := context.Background()

	created, err := client.CreateMonitor(ctx, map[string]any{
		"type":    "metric alert",
		"query":   "avg(last_5m):avg:system.cpu.user{service:api} > 95",
		"name":    "CPU very high",
		"message": "page the on-call",
		"tags":    []string{"service:api"},
		"options": map[string]any{"thresholds": map[string]any{"critical": 95}},
	})
	if err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	id, ok := created["id"].(float64)
	if !ok {
		t.Fatalf("created monitor has no numeric id: %v", created["id"])
	}

	fetched, err := client.GetMonitorRaw(ctx, int(id))
	if err != nil {
		t.Fatalf("GetMonitorRaw after create: %v", err)
	}
	if fetched["name"] != "CPU very high" {
		t.Errorf("name = %v, want CPU very high", fetched["name"])
	}
	if fetched["query"] != "avg(last_5m):avg:system.cpu.user{service:api} > 95" {
		t.Errorf("query = %v", fetched["query"])
	}
	opts, ok := fetched["options"].(map[string]any)
	if !ok {
		t.Fatalf("options = %T, want map", fetched["options"])
	}
	if _, hasThresholds := opts["thresholds"]; !hasThresholds {
		t.Error("nested options did not survive the round trip")
	}
}

func TestMockddCreatedMonitorAppearsInList(t *testing.T) {
	client := mockddtest.NewTestClient(t)
	ctx := context.Background()

	before, err := client.ListMonitors(ctx, "", nil, "")
	if err != nil {
		t.Fatalf("ListMonitors: %v", err)
	}

	if _, err := client.CreateMonitor(ctx, map[string]any{
		"type":  "metric alert",
		"query": "avg(last_5m):avg:system.mem.used{*} > 90",
		"name":  "Memory high",
	}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	after, err := client.ListMonitors(ctx, "", nil, "")
	if err != nil {
		t.Fatalf("ListMonitors: %v", err)
	}
	if len(after) != len(before)+1 {
		t.Errorf("list went from %d to %d monitors, want one more", len(before), len(after))
	}
}

// The property the whole read-modify-write design exists to protect: changing
// one field must leave everything else — including fields the CLI never models
// — exactly as it was.
func TestMockddUpdateMonitorPreservesUnmodifiedFields(t *testing.T) {
	client := mockddtest.NewTestClient(t)
	ctx := context.Background()

	created, err := client.CreateMonitor(ctx, map[string]any{
		"type":             "metric alert",
		"query":            "avg(last_5m):avg:system.cpu.user{*} > 90",
		"name":             "original name",
		"message":          "original message",
		"tags":             []string{"team:platform", "env:prod"},
		"restricted_roles": []string{"role-abc"},
		"options": map[string]any{
			"thresholds":        map[string]any{"critical": 90, "warning": 80},
			"notify_no_data":    true,
			"renotify_interval": 60,
		},
	})
	if err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
	id := int(created["id"].(float64))

	existing, err := client.GetMonitorRaw(ctx, id)
	if err != nil {
		t.Fatalf("GetMonitorRaw: %v", err)
	}
	existing["name"] = "renamed"

	if _, err := client.UpdateMonitor(ctx, id, existing); err != nil {
		t.Fatalf("UpdateMonitor: %v", err)
	}

	after, err := client.GetMonitorRaw(ctx, id)
	if err != nil {
		t.Fatalf("GetMonitorRaw after update: %v", err)
	}

	if after["name"] != "renamed" {
		t.Errorf("name = %v, want renamed", after["name"])
	}
	if after["message"] != "original message" {
		t.Errorf("message = %v — an unrelated field was lost", after["message"])
	}
	if _, ok := after["restricted_roles"]; !ok {
		t.Error("restricted_roles was lost — this is precisely the clobbering bug the design prevents")
	}
	opts, ok := after["options"].(map[string]any)
	if !ok {
		t.Fatalf("options = %T, want map", after["options"])
	}
	if opts["notify_no_data"] != true {
		t.Error("options.notify_no_data was lost")
	}
	thresholds, ok := opts["thresholds"].(map[string]any)
	if !ok {
		t.Fatalf("thresholds = %T, want map", opts["thresholds"])
	}
	if thresholds["warning"] != 80.0 {
		t.Errorf("options.thresholds.warning = %v, want 80", thresholds["warning"])
	}
}

func TestMockddDeleteMonitorRemovesIt(t *testing.T) {
	client := mockddtest.NewTestClient(t)
	ctx := context.Background()

	created, err := client.CreateMonitor(ctx, map[string]any{
		"type":  "metric alert",
		"query": "avg(last_5m):avg:system.cpu.user{*} > 99",
		"name":  "throwaway",
	})
	if err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
	id := int(created["id"].(float64))

	if err := client.DeleteMonitor(ctx, id, false); err != nil {
		t.Fatalf("DeleteMonitor: %v", err)
	}

	_, err = client.GetMonitorRaw(ctx, id)
	if err == nil {
		t.Fatal("monitor is still readable after delete")
	}
	var aerr *agenterrors.APIError
	if !errors.As(err, &aerr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if aerr.FixableBy != agenterrors.FixableByAgent {
		t.Errorf("fixable_by = %s, want agent for a 404", aerr.FixableBy)
	}
}

// A monitor referenced by another resource must refuse to delete without
// force. That refusal is real signal, so the mock models it rather than
// letting every delete succeed.
func TestMockddDeleteReferencedMonitorRequiresForce(t *testing.T) {
	client := mockddtest.NewTestClient(t)
	ctx := context.Background()

	// Fixture monitor 1003 is referenced by an SLO in the mock's fixtures.
	err := client.DeleteMonitor(ctx, 1003, false)
	if err == nil {
		t.Fatal("expected a refusal deleting a referenced monitor without force")
	}
	var aerr *agenterrors.APIError
	if !errors.As(err, &aerr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if aerr.FixableBy != agenterrors.FixableByAgent {
		t.Errorf("fixable_by = %s, want agent", aerr.FixableBy)
	}

	if err := client.DeleteMonitor(ctx, 1003, true); err != nil {
		t.Fatalf("DeleteMonitor with force: %v", err)
	}
}

func TestMockddValidateMonitorAcceptsAndRejects(t *testing.T) {
	client := mockddtest.NewTestClient(t)
	ctx := context.Background()

	if err := client.ValidateMonitor(ctx, map[string]any{
		"type":  "metric alert",
		"query": "avg(last_5m):avg:system.cpu.user{*} > 90",
		"name":  "fine",
	}); err != nil {
		t.Fatalf("ValidateMonitor on a well-formed definition: %v", err)
	}

	err := client.ValidateMonitor(ctx, map[string]any{
		"type":  "metric alert",
		"query": "this is not a valid query",
		"name":  "broken",
	})
	if err == nil {
		t.Fatal("expected a validation failure for a malformed query")
	}
	var aerr *agenterrors.APIError
	if !errors.As(err, &aerr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if aerr.FixableBy != agenterrors.FixableByAgent {
		t.Errorf("fixable_by = %s, want agent — a bad query is the agent's to fix", aerr.FixableBy)
	}
}

func TestMockddValidateExistingMonitor(t *testing.T) {
	client := mockddtest.NewTestClient(t)

	if err := client.ValidateExistingMonitor(context.Background(), 1001, map[string]any{
		"type":  "metric alert",
		"query": "avg(last_5m):avg:system.cpu.user{*} > 50",
		"name":  "adjusted",
	}); err != nil {
		t.Fatalf("ValidateExistingMonitor: %v", err)
	}
}
