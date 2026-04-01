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
