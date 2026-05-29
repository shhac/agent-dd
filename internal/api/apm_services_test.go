package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-dd/internal/api"
)

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

// Empty services slice should yield an empty result, not nil-deref.
func TestListServicesEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"attributes": map[string]any{
					"services": []string{},
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	services, err := client.ListServices(context.Background(), "production", "")
	if err != nil {
		t.Fatalf("ListServices empty: %v", err)
	}
	if len(services) != 0 {
		t.Errorf("expected 0 services, got %d", len(services))
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
