package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-dd/internal/api"
)

func TestSearchTraces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/spans/events/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		filter, ok := body["filter"].(map[string]any)
		if !ok {
			t.Fatal("missing filter in request body")
		}
		query, _ := filter["query"].(string)
		if query != "service:web-api status:error" {
			t.Errorf("expected query 'service:web-api status:error', got %q", query)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"type": "spans",
					"attributes": map[string]any{
						"trace_id": "abc123",
						"service":  "web-api",
						"name":     "http.request",
						"duration": 1.5,
						"status":   "error",
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	resp, err := client.SearchTraces(context.Background(), "status:error", "web-api", "2024-01-15T09:00:00Z", "2024-01-15T10:00:00Z", 10)
	if err != nil {
		t.Fatalf("SearchTraces failed: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 span, got %d", len(resp.Data))
	}
	if resp.Data[0].Attributes.TraceID != "abc123" {
		t.Errorf("expected trace_id=abc123, got %s", resp.Data[0].Attributes.TraceID)
	}
	if resp.Data[0].Attributes.Service != "web-api" {
		t.Errorf("expected service=web-api, got %s", resp.Data[0].Attributes.Service)
	}
}

func TestSearchTracesCursor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"type": "spans",
					"attributes": map[string]any{
						"trace_id": "def456",
						"service":  "web-api",
					},
				},
			},
			"meta": map[string]any{
				"page": map[string]any{
					"after": "next-page-cursor",
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	resp, err := client.SearchTraces(context.Background(), "*", "", "2024-01-15T09:00:00Z", "2024-01-15T10:00:00Z", 10)
	if err != nil {
		t.Fatalf("SearchTraces failed: %v", err)
	}
	if resp.Cursor() != "next-page-cursor" {
		t.Errorf("expected cursor 'next-page-cursor', got %q", resp.Cursor())
	}
}

func TestListServices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/apm/services" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if env := r.URL.Query().Get("filter[env]"); env != "production" {
			t.Errorf("expected filter[env]=production, got %q", env)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"attributes": map[string]any{
					"services": []string{"web-api", "auth-svc", "worker-job"},
				},
				"id":   "1",
				"type": "services_list",
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	services, err := client.ListServices(context.Background(), "production", "")
	if err != nil {
		t.Fatalf("ListServices failed: %v", err)
	}
	if len(services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(services))
	}

	names := make(map[string]bool)
	for _, s := range services {
		names[s.Name] = true
	}
	for _, expected := range []string{"web-api", "auth-svc", "worker-job"} {
		if !names[expected] {
			t.Errorf("expected service %q not found", expected)
		}
	}
}

func TestListServicesSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"attributes": map[string]any{
					"services": []string{"web-api", "web-frontend", "auth-svc"},
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	services, err := client.ListServices(context.Background(), "*", "web")
	if err != nil {
		t.Fatalf("ListServices failed: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services matching 'web', got %d", len(services))
	}
}
