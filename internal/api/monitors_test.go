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
			"counts": map[string]any{
				"status": []map[string]any{
					{"name": "alert", "count": 2},
					{"name": "ok", "count": 1},
				},
				"muted": []map[string]any{
					{"name": "false", "count": 3},
				},
			},
			"metadata": map[string]any{"total": 3, "page": 0, "per_page": 30, "page_count": 1, "total_results": 3},
			"monitors": []map[string]any{
				{"id": 10, "name": "CPU High", "overall_state": "alert", "type": "metric alert"},
				{"id": 11, "name": "Disk OK", "overall_state": "ok", "type": "metric alert"},
				{"id": 12, "name": "Mem High", "overall_state": "alert", "type": "metric alert"},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	resp, err := client.SearchMonitors(context.Background(), "host:web01", "alert")
	if err != nil {
		t.Fatalf("SearchMonitors failed: %v", err)
	}
	if len(resp.Monitors) != 2 {
		t.Fatalf("expected 2 monitors after status filter, got %d", len(resp.Monitors))
	}
	for _, m := range resp.Monitors {
		if m.Status != "alert" {
			t.Errorf("expected status alert, got %s", m.Status)
		}
	}
	// The status filter applies client-side to Monitors only; Counts reflects
	// the full pre-filter result set so the rollup remains meaningful.
	if resp.Counts == nil {
		t.Fatal("expected non-nil Counts")
	}
	if len(resp.Counts.Status) != 2 {
		t.Errorf("expected 2 status buckets, got %d", len(resp.Counts.Status))
	}
	if resp.Metadata == nil || resp.Metadata.Total != 3 {
		t.Errorf("expected metadata.total=3, got %+v", resp.Metadata)
	}
}
