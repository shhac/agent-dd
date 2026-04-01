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

func TestMuteHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/host/web-01.prod/mute" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("DD-API-KEY") != "test-key" {
			t.Error("missing or wrong DD-API-KEY")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body["hostname"] != "web-01.prod" {
			t.Errorf("expected hostname=web-01.prod, got %v", body["hostname"])
		}
		// JSON numbers decode as float64
		if end, ok := body["end"].(float64); !ok || int64(end) != 1700000000 {
			t.Errorf("expected end=1700000000, got %v", body["end"])
		}
		if body["message"] != "scheduled maintenance" {
			t.Errorf("expected message=scheduled maintenance, got %v", body["message"])
		}

		json.NewEncoder(w).Encode(map[string]any{"action": "muted", "hostname": "web-01.prod"})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "test-key", "test-app")
	err := client.MuteHost(context.Background(), "web-01.prod", 1700000000, "scheduled maintenance")
	if err != nil {
		t.Fatalf("MuteHost failed: %v", err)
	}
}

func TestListHosts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/hosts" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		filters := q["filter"]
		if len(filters) < 1 || filters[0] != "web" {
			t.Errorf("expected filter to contain web, got %v", filters)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"host_list": []map[string]any{
				{"name": "web-01.prod", "up": true, "is_muted": false},
				{"name": "web-02.prod", "up": true, "is_muted": true},
			},
			"total_returned": 2,
			"total_matching": 5,
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	resp, err := client.ListHosts(context.Background(), "web", nil)
	if err != nil {
		t.Fatalf("ListHosts failed: %v", err)
	}
	if resp.TotalReturned != 2 {
		t.Errorf("expected TotalReturned=2, got %d", resp.TotalReturned)
	}
	if resp.TotalMatching != 5 {
		t.Errorf("expected TotalMatching=5, got %d", resp.TotalMatching)
	}
	if len(resp.HostList) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(resp.HostList))
	}
	if resp.HostList[0].Name != "web-01.prod" {
		t.Errorf("expected first host name=web-01.prod, got %s", resp.HostList[0].Name)
	}
}

func TestGetHostNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"host_list":      []any{},
			"total_returned": 0,
			"total_matching": 0,
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	_, err := client.GetHost(context.Background(), "nonexistent-host")
	if err == nil {
		t.Fatal("expected error for missing host, got nil")
	}

	var aerr *agenterrors.APIError
	if !errors.As(err, &aerr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if aerr.FixableBy != agenterrors.FixableByAgent {
		t.Errorf("expected FixableByAgent, got %s", aerr.FixableBy)
	}
}
