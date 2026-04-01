package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-dd/internal/api"
)

func TestListMonitorsStatusFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "name": "A", "overall_state": "alert", "type": "metric alert"},
			{"id": 2, "name": "B", "overall_state": "ok", "type": "metric alert"},
			{"id": 3, "name": "C", "overall_state": "alert", "type": "metric alert"},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")

	monitors, err := client.ListMonitors(context.Background(), "", nil, "alert")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(monitors) != 2 {
		t.Fatalf("expected 2 monitors, got %d", len(monitors))
	}
	for _, m := range monitors {
		if m.Status != "alert" {
			t.Errorf("expected status alert, got %s", m.Status)
		}
	}
}

func TestListMonitorsNoFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "name": "A", "overall_state": "alert", "type": "metric alert"},
			{"id": 2, "name": "B", "overall_state": "ok", "type": "metric alert"},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")

	monitors, err := client.ListMonitors(context.Background(), "", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(monitors) != 2 {
		t.Fatalf("expected 2 monitors, got %d", len(monitors))
	}
}

func TestMuteMonitor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/monitor/123/mute" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("DD-API-KEY") != "test-key" {
			t.Error("missing or wrong DD-API-KEY")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body["end"] != "1700000000" {
			t.Errorf("expected end=1700000000, got %v", body["end"])
		}
		if body["scope"] != "*" {
			t.Errorf("expected scope=*, got %v", body["scope"])
		}

		json.NewEncoder(w).Encode(map[string]any{"id": 123})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "test-key", "test-app")
	err := client.MuteMonitor(context.Background(), 123, "1700000000", "maintenance")
	if err != nil {
		t.Fatalf("MuteMonitor failed: %v", err)
	}
}

func TestUnmuteMonitor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/monitor/123/unmute" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body["scope"] != "*" {
			t.Errorf("expected scope=*, got %v", body["scope"])
		}
		if body["all_scopes"] != true {
			t.Errorf("expected all_scopes=true, got %v", body["all_scopes"])
		}

		json.NewEncoder(w).Encode(map[string]any{"id": 123})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	err := client.UnmuteMonitor(context.Background(), 123)
	if err != nil {
		t.Fatalf("UnmuteMonitor failed: %v", err)
	}
}

func TestSearchMonitors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/monitor/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if q := r.URL.Query().Get("query"); q != "host:web01" {
			t.Errorf("expected query=host:web01, got %q", q)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"monitors": []map[string]any{
				{"id": 10, "name": "CPU High", "overall_state": "alert", "type": "metric alert"},
				{"id": 11, "name": "Disk OK", "overall_state": "ok", "type": "metric alert"},
				{"id": 12, "name": "Mem High", "overall_state": "alert", "type": "metric alert"},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	monitors, err := client.SearchMonitors(context.Background(), "host:web01", "alert")
	if err != nil {
		t.Fatalf("SearchMonitors failed: %v", err)
	}
	if len(monitors) != 2 {
		t.Fatalf("expected 2 monitors after status filter, got %d", len(monitors))
	}
	for _, m := range monitors {
		if m.Status != "alert" {
			t.Errorf("expected status alert, got %s", m.Status)
		}
	}
}
