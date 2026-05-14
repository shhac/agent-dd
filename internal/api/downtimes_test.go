package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-dd/internal/api"
)

func TestCreateDowntime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/downtime" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		data, _ := body["data"].(map[string]any)
		if data["type"] != "downtime" {
			t.Errorf("expected type=downtime, got %v", data["type"])
		}
		attrs, _ := data["attributes"].(map[string]any)
		if attrs["message"] != "investigating" {
			t.Errorf("expected message=investigating, got %v", attrs["message"])
		}
		if attrs["scope"] != "monitor_id:123" {
			t.Errorf("expected scope=monitor_id:123, got %v", attrs["scope"])
		}

		// DD spec: schedule.end is an ISO-8601 datetime string (not epoch).
		schedule, ok := attrs["schedule"].(map[string]any)
		if !ok {
			t.Fatalf("expected schedule object, got %v", attrs["schedule"])
		}
		if _, hasStart := schedule["start"]; hasStart {
			t.Errorf("schedule.start should be omitted to mean 'now', got %v", schedule["start"])
		}
		gotEnd, _ := schedule["end"].(string)
		wantEnd := "2023-11-14T22:13:20Z" // time.Unix(1700000000, 0).UTC().Format(time.RFC3339)
		if gotEnd != wantEnd {
			t.Errorf("schedule.end = %q, want ISO-8601 %q", gotEnd, wantEnd)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":   "dt-abc-123",
				"type": "downtime",
				"attributes": map[string]any{
					"status":  "active",
					"message": "investigating",
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	dt, err := client.CreateDowntime(context.Background(), 123, 1700000000, "investigating")
	if err != nil {
		t.Fatalf("CreateDowntime failed: %v", err)
	}
	if dt.ID != "dt-abc-123" {
		t.Errorf("expected ID=dt-abc-123, got %s", dt.ID)
	}
}

// end=0 means indefinite — schedule should be omitted entirely, not sent
// with an empty end.
func TestCreateDowntimeIndefinite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		data, _ := body["data"].(map[string]any)
		attrs, _ := data["attributes"].(map[string]any)
		if _, hasSchedule := attrs["schedule"]; hasSchedule {
			t.Errorf("schedule should be omitted for indefinite downtime, got %v", attrs["schedule"])
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"id": "dt-1", "type": "downtime"},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	if _, err := client.CreateDowntime(context.Background(), 42, 0, "indefinite"); err != nil {
		t.Fatalf("CreateDowntime: %v", err)
	}
}

func TestListActiveDowntimes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/downtime" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if mid := r.URL.Query().Get("filter[monitor_id]"); mid != "123" {
			t.Errorf("expected filter[monitor_id]=123, got %q", mid)
		}
		if status := r.URL.Query().Get("filter[status]"); status != "active" {
			t.Errorf("expected filter[status]=active, got %q", status)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "dt-1", "type": "downtime", "attributes": map[string]any{"status": "active"}},
				{"id": "dt-2", "type": "downtime", "attributes": map[string]any{"status": "active"}},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	downtimes, err := client.ListActiveDowntimes(context.Background(), 123)
	if err != nil {
		t.Fatalf("ListActiveDowntimes failed: %v", err)
	}
	if len(downtimes) != 2 {
		t.Fatalf("expected 2 downtimes, got %d", len(downtimes))
	}
}

func TestCancelDowntime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/downtime/dt-abc-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	err := client.CancelDowntime(context.Background(), "dt-abc-123")
	if err != nil {
		t.Fatalf("CancelDowntime failed: %v", err)
	}
}
