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

func TestNewClientSiteURL(t *testing.T) {
	tests := []struct {
		site     string
		wantBase string
	}{
		{"datadoghq.com", "https://api.datadoghq.com/api"},
		{"datadoghq.eu", "https://api.datadoghq.eu/api"},
		{"us3.datadoghq.com", "https://api.us3.datadoghq.com/api"},
		{"us5.datadoghq.com", "https://api.us5.datadoghq.com/api"},
		{"ap1.datadoghq.com", "https://api.ap1.datadoghq.com/api"},
		{"", "https://api.datadoghq.com/api"},
	}
	for _, tt := range tests {
		c := api.NewClient("key", "app", tt.site)
		if c == nil {
			t.Errorf("NewClient(%q) returned nil", tt.site)
		}
	}
}

func TestValidate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/validate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("DD-API-KEY") != "test-key" {
			t.Error("missing or wrong DD-API-KEY")
		}
		if r.Header.Get("DD-APPLICATION-KEY") != "test-app" {
			t.Error("missing or wrong DD-APPLICATION-KEY")
		}
		json.NewEncoder(w).Encode(map[string]any{"valid": true})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "test-key", "test-app")
	if err := client.Validate(context.Background()); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
}

func TestClassifyHTTPErrorDefault(t *testing.T) {
	for _, status := range []int{400, 409} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]any{"errors": []string{"bad request"}})
		}))

		client := api.NewTestClient(srv.URL+"/api", "key", "app")
		err := client.Validate(context.Background())
		srv.Close()

		if err == nil {
			t.Errorf("status %d: expected error, got nil", status)
			continue
		}
		var aerr *agenterrors.APIError
		if !errors.As(err, &aerr) {
			t.Errorf("status %d: expected *APIError, got %T", status, err)
			continue
		}
		if aerr.FixableBy != agenterrors.FixableByAgent {
			t.Errorf("status %d: fixable_by = %q, want %q", status, aerr.FixableBy, agenterrors.FixableByAgent)
		}
		if aerr.Hint != "" {
			t.Errorf("status %d: expected empty hint for default branch, got %q", status, aerr.Hint)
		}
	}
}

func TestDoAndDecodeMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	// Validate calls do() which returns raw JSON, then we need a method that
	// calls doAndDecode to trigger the unmarshal error. ListSLOs uses doAndDecode.
	_, err := client.ListSLOs(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	var aerr *agenterrors.APIError
	if !errors.As(err, &aerr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if aerr.FixableBy != agenterrors.FixableByAgent {
		t.Errorf("expected FixableByAgent, got %s", aerr.FixableBy)
	}
}

func TestClassifyHTTPErrors(t *testing.T) {
	tests := []struct {
		status   int
		wantType string
	}{
		{401, "human"},
		{403, "human"},
		{404, "agent"},
		{429, "retry"},
		{500, "retry"},
		{503, "retry"},
	}

	for _, tt := range tests {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tt.status)
			json.NewEncoder(w).Encode(map[string]any{"errors": []string{"test error"}})
		}))

		client := api.NewTestClient(srv.URL+"/api", "key", "app")
		err := client.Validate(context.Background())
		srv.Close()

		if err == nil {
			t.Errorf("status %d: expected error, got nil", tt.status)
			continue
		}
		var aerr *agenterrors.APIError
		if !errors.As(err, &aerr) {
			t.Errorf("status %d: expected *APIError, got %T", tt.status, err)
			continue
		}
		if string(aerr.FixableBy) != tt.wantType {
			t.Errorf("status %d: fixable_by = %q, want %q", tt.status, aerr.FixableBy, tt.wantType)
		}
		if aerr.Hint == "" && (tt.status == 401 || tt.status == 403 || tt.status == 404 || tt.status == 429 || tt.status >= 500) {
			t.Errorf("status %d: expected non-empty hint", tt.status)
		}
	}
}
