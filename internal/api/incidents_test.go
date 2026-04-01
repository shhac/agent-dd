package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-dd/internal/api"
)

func TestGetIncident(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/incidents/inc-999" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("DD-API-KEY") != "test-key" {
			t.Error("missing or wrong DD-API-KEY")
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":   "inc-999",
				"type": "incidents",
				"attributes": map[string]any{
					"title":    "Database outage",
					"status":   "active",
					"severity": "SEV-1",
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "test-key", "test-app")
	incident, err := client.GetIncident(context.Background(), "inc-999")
	if err != nil {
		t.Fatalf("GetIncident failed: %v", err)
	}
	if incident.ID != "inc-999" {
		t.Errorf("expected ID=inc-999, got %s", incident.ID)
	}
	if incident.Attributes.Title != "Database outage" {
		t.Errorf("expected title=Database outage, got %s", incident.Attributes.Title)
	}
	if incident.Attributes.Status != "active" {
		t.Errorf("expected status=active, got %s", incident.Attributes.Status)
	}
}

func TestCreateIncident(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/incidents" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("DD-API-KEY") != "test-key" {
			t.Error("missing or wrong DD-API-KEY")
		}
		if r.Header.Get("DD-APPLICATION-KEY") != "test-app" {
			t.Error("missing or wrong DD-APPLICATION-KEY")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		data, ok := body["data"].(map[string]any)
		if !ok {
			t.Fatal("expected body.data to be an object")
		}
		if data["type"] != "incidents" {
			t.Errorf("expected data.type=incidents, got %v", data["type"])
		}

		attrs, ok := data["attributes"].(map[string]any)
		if !ok {
			t.Fatal("expected data.attributes to be an object")
		}
		if attrs["title"] != "Service outage" {
			t.Errorf("expected title=Service outage, got %v", attrs["title"])
		}

		fields, ok := attrs["fields"].(map[string]any)
		if !ok {
			t.Fatal("expected data.attributes.fields to be an object")
		}
		sev, ok := fields["severity"].(map[string]any)
		if !ok {
			t.Fatal("expected fields.severity to be an object")
		}
		if sev["value"] != "SEV-1" {
			t.Errorf("expected severity.value=SEV-1, got %v", sev["value"])
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":   "incident-abc",
				"type": "incidents",
				"attributes": map[string]any{
					"title":    "Service outage",
					"status":   "active",
					"severity": "SEV-1",
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "test-key", "test-app")
	incident, err := client.CreateIncident(context.Background(), "Service outage", "SEV-1", "")
	if err != nil {
		t.Fatalf("CreateIncident failed: %v", err)
	}
	if incident.ID != "incident-abc" {
		t.Errorf("expected ID=incident-abc, got %s", incident.ID)
	}
	if incident.Attributes.Title != "Service outage" {
		t.Errorf("expected title=Service outage, got %s", incident.Attributes.Title)
	}
}

func TestCreateIncidentWithCommander(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		data := body["data"].(map[string]any)

		rels, ok := data["relationships"].(map[string]any)
		if !ok {
			t.Fatal("expected data.relationships to be present with commander")
		}
		commander, ok := rels["commander_user"].(map[string]any)
		if !ok {
			t.Fatal("expected relationships.commander_user to be an object")
		}
		cmdData, ok := commander["data"].(map[string]any)
		if !ok {
			t.Fatal("expected commander_user.data to be an object")
		}
		if cmdData["type"] != "users" {
			t.Errorf("expected commander data.type=users, got %v", cmdData["type"])
		}
		if cmdData["id"] != "user@example.com" {
			t.Errorf("expected commander data.id=user@example.com, got %v", cmdData["id"])
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":   "incident-xyz",
				"type": "incidents",
				"attributes": map[string]any{
					"title":  "Outage with commander",
					"status": "active",
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	incident, err := client.CreateIncident(context.Background(), "Outage with commander", "SEV-2", "user@example.com")
	if err != nil {
		t.Fatalf("CreateIncident with commander failed: %v", err)
	}
	if incident.ID != "incident-xyz" {
		t.Errorf("expected ID=incident-xyz, got %s", incident.ID)
	}
}

func TestListIncidents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/incidents" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":   "inc-1",
					"type": "incidents",
					"attributes": map[string]any{
						"title":  "Outage",
						"status": "active",
					},
				},
			},
			"meta": map[string]any{
				"pagination": map[string]any{
					"offset":      0,
					"next_offset": 25,
					"size":        25,
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	resp, err := client.ListIncidents(context.Background(), "")
	if err != nil {
		t.Fatalf("ListIncidents failed: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(resp.Data))
	}
	if resp.Data[0].ID != "inc-1" {
		t.Errorf("expected ID=inc-1, got %s", resp.Data[0].ID)
	}
	if !resp.HasMore() {
		t.Error("expected HasMore() to be true")
	}
}

func TestListIncidentsNoMore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":   "inc-1",
					"type": "incidents",
					"attributes": map[string]any{
						"title":  "Minor issue",
						"status": "resolved",
					},
				},
			},
			"meta": map[string]any{
				"pagination": map[string]any{
					"offset":      0,
					"next_offset": 0,
					"size":        1,
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	resp, err := client.ListIncidents(context.Background(), "")
	if err != nil {
		t.Fatalf("ListIncidents failed: %v", err)
	}
	if resp.HasMore() {
		t.Error("expected HasMore() to be false")
	}
}

func TestIncidentListResponseHasMore(t *testing.T) {
	tests := []struct {
		name string
		resp *api.IncidentListResponse
		want bool
	}{
		{"nil Meta", &api.IncidentListResponse{Meta: nil}, false},
		{"nil Pagination", &api.IncidentListResponse{Meta: &api.IncidentListMeta{Pagination: nil}}, false},
		{"NextOffset > Offset", &api.IncidentListResponse{Meta: &api.IncidentListMeta{Pagination: &api.IncidentPagination{Offset: 0, NextOffset: 25}}}, true},
		{"NextOffset == Offset", &api.IncidentListResponse{Meta: &api.IncidentListMeta{Pagination: &api.IncidentPagination{Offset: 25, NextOffset: 25}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resp.HasMore(); got != tt.want {
				t.Errorf("HasMore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateIncident(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/incidents/inc-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		data, ok := body["data"].(map[string]any)
		if !ok {
			t.Fatal("expected body.data to be an object")
		}
		if data["type"] != "incidents" {
			t.Errorf("expected data.type=incidents, got %v", data["type"])
		}
		if data["id"] != "inc-123" {
			t.Errorf("expected data.id=inc-123, got %v", data["id"])
		}

		attrs, ok := data["attributes"].(map[string]any)
		if !ok {
			t.Fatal("expected data.attributes to be an object")
		}
		if attrs["status"] != "resolved" {
			t.Errorf("expected status=resolved, got %v", attrs["status"])
		}

		fields, ok := attrs["fields"].(map[string]any)
		if !ok {
			t.Fatal("expected attributes.fields for severity update")
		}
		sev, ok := fields["severity"].(map[string]any)
		if !ok {
			t.Fatal("expected fields.severity to be an object")
		}
		if sev["value"] != "SEV-3" {
			t.Errorf("expected severity.value=SEV-3, got %v", sev["value"])
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":   "inc-123",
				"type": "incidents",
				"attributes": map[string]any{
					"title":    "Updated incident",
					"status":   "resolved",
					"severity": "SEV-3",
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	incident, err := client.UpdateIncident(context.Background(), "inc-123", "resolved", "SEV-3")
	if err != nil {
		t.Fatalf("UpdateIncident failed: %v", err)
	}
	if incident.ID != "inc-123" {
		t.Errorf("expected ID=inc-123, got %s", incident.ID)
	}
	if incident.Attributes.Status != "resolved" {
		t.Errorf("expected status=resolved, got %s", incident.Attributes.Status)
	}
}
