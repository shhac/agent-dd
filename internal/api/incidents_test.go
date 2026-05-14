package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
					"title":             "Database outage",
					"state":             "active",
					"severity":          "SEV-1",
					"customer_impacted": true,
					"public_id":         42,
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
	if incident.Attributes.State != "active" {
		t.Errorf("expected state=active, got %s", incident.Attributes.State)
	}
	if !incident.Attributes.CustomerImpacted {
		t.Error("expected customer_impacted=true")
	}
	if incident.Attributes.PublicID != 42 {
		t.Errorf("expected public_id=42, got %d", incident.Attributes.PublicID)
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

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		data, _ := body["data"].(map[string]any)
		if data["type"] != "incidents" {
			t.Errorf("expected data.type=incidents, got %v", data["type"])
		}

		attrs, _ := data["attributes"].(map[string]any)
		if attrs["title"] != "Service outage" {
			t.Errorf("expected title=Service outage, got %v", attrs["title"])
		}
		// v2 spec: severity is a top-level attribute string, not nested in fields.
		if attrs["severity"] != "SEV-1" {
			t.Errorf("expected attributes.severity=SEV-1, got %v", attrs["severity"])
		}
		// customer_impacted is required by the API.
		if attrs["customer_impacted"] != true {
			t.Errorf("expected customer_impacted=true, got %v", attrs["customer_impacted"])
		}
		if _, hasFields := attrs["fields"]; hasFields {
			t.Error("v2 create must not use legacy fields.severity envelope")
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":   "incident-abc",
				"type": "incidents",
				"attributes": map[string]any{
					"title":    "Service outage",
					"state":    "active",
					"severity": "SEV-1",
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "test-key", "test-app")
	incident, err := client.CreateIncident(context.Background(), "Service outage", "SEV-1", "", true)
	if err != nil {
		t.Fatalf("CreateIncident failed: %v", err)
	}
	if incident.ID != "incident-abc" {
		t.Errorf("expected ID=incident-abc, got %s", incident.ID)
	}
}

func TestCreateIncidentWithCommanderUUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		data := body["data"].(map[string]any)
		rels, ok := data["relationships"].(map[string]any)
		if !ok {
			t.Fatal("expected data.relationships to be present with commander")
		}
		cmdData := rels["commander_user"].(map[string]any)["data"].(map[string]any)
		if cmdData["type"] != "users" {
			t.Errorf("expected commander data.type=users, got %v", cmdData["type"])
		}
		if cmdData["id"] != "12345678-1234-1234-1234-123456789012" {
			t.Errorf("commander data.id mismatch, got %v", cmdData["id"])
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"id": "incident-xyz", "type": "incidents"},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	_, err := client.CreateIncident(context.Background(), "Outage", "SEV-2", "12345678-1234-1234-1234-123456789012", false)
	if err != nil {
		t.Fatalf("CreateIncident with valid UUID: %v", err)
	}
}

// Non-UUID commander values (handles, emails) must be rejected up-front.
// DD returns a 400 server-side; surfacing a clear error pre-flight saves a
// round trip and gives a better hint.
func TestCreateIncidentRejectsNonUUIDCommander(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be hit when commander is not a UUID")
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	for _, bad := range []string{"user@example.com", "alice", "abc123"} {
		_, err := client.CreateIncident(context.Background(), "Outage", "SEV-2", bad, false)
		if err == nil {
			t.Errorf("expected error for non-UUID commander %q, got nil", bad)
		}
		if err != nil && !strings.Contains(err.Error(), "UUID") {
			t.Errorf("error should mention UUID, got %q", err.Error())
		}
	}
}

func TestListIncidents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/incidents" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if s := r.URL.Query().Get("filter[state]"); s != "active" {
			t.Errorf("expected filter[state]=active, got %q", s)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":   "inc-1",
					"type": "incidents",
					"attributes": map[string]any{
						"title": "Outage",
						"state": "active",
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
	resp, err := client.ListIncidents(context.Background(), "active")
	if err != nil {
		t.Fatalf("ListIncidents failed: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(resp.Data))
	}
	if resp.Data[0].Attributes.State != "active" {
		t.Errorf("expected state=active, got %s", resp.Data[0].Attributes.State)
	}
	if !resp.HasMore() {
		t.Error("expected HasMore() to be true")
	}
}

func TestListIncidentsNoMore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{},
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

		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		data := body["data"].(map[string]any)
		attrs := data["attributes"].(map[string]any)

		// State must be written via fields.state.value, NOT attributes.status.
		if _, hasStatus := attrs["status"]; hasStatus {
			t.Error("update must not write attributes.status (legacy v1, silent no-op on v2)")
		}
		fields, ok := attrs["fields"].(map[string]any)
		if !ok {
			t.Fatal("expected attributes.fields for state update")
		}
		state := fields["state"].(map[string]any)
		if state["value"] != "resolved" {
			t.Errorf("expected fields.state.value=resolved, got %v", state["value"])
		}
		// Severity is a top-level attribute string.
		if attrs["severity"] != "SEV-3" {
			t.Errorf("expected attributes.severity=SEV-3, got %v", attrs["severity"])
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":   "inc-123",
				"type": "incidents",
				"attributes": map[string]any{
					"title":    "Updated incident",
					"state":    "resolved",
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
	if incident.Attributes.State != "resolved" {
		t.Errorf("expected state=resolved, got %s", incident.Attributes.State)
	}
}
