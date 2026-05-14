package logs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/shhac/agent-dd/internal/cli/shared"
)

func TestLogsSearch(t *testing.T) {
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/logs/events/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		filter, ok := body["filter"].(map[string]any)
		if !ok {
			t.Fatal("missing filter in request body")
		}
		if query, _ := filter["query"].(string); query != "service:web" {
			t.Errorf("expected query 'service:web', got %q", query)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":   "log1",
					"type": "log",
					"attributes": map[string]any{
						"timestamp": "2024-01-15T10:00:00Z",
						"service":   "web-api",
						"status":    "error",
						"message":   "connection timeout",
					},
				},
			},
		})
	})

	client, err := shared.ClientFactory()
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.SearchLogs(context.Background(), "service:web", "2024-01-15T09:00:00Z", "2024-01-15T10:00:00Z", "", 50, "", "")
	if err != nil {
		t.Fatalf("SearchLogs failed: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(resp.Data))
	}
	if resp.Data[0].Attributes.Service != "web-api" {
		t.Errorf("expected service 'web-api', got %q", resp.Data[0].Attributes.Service)
	}
}

func TestLogsSearchWithCursor(t *testing.T) {
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":   "log1",
					"type": "log",
					"attributes": map[string]any{
						"timestamp": "2024-01-15T10:00:00Z",
						"service":   "web-api",
						"status":    "error",
						"message":   "timeout",
					},
				},
			},
			"meta": map[string]any{
				"page": map[string]any{
					"after": "cursor123",
				},
			},
		})
	})

	client, err := shared.ClientFactory()
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.SearchLogs(context.Background(), "service:web", "2024-01-15T09:00:00Z", "2024-01-15T10:00:00Z", "", 50, "", "")
	if err != nil {
		t.Fatalf("SearchLogs failed: %v", err)
	}
	if resp.Cursor() != "cursor123" {
		t.Errorf("expected cursor 'cursor123', got %q", resp.Cursor())
	}
}

func TestLogsSearchPassesCursor(t *testing.T) {
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		page, ok := body["page"].(map[string]any)
		if !ok {
			t.Fatal("missing page in request body")
		}
		cursor, _ := page["cursor"].(string)
		if cursor != "mycursor" {
			t.Errorf("expected page.cursor='mycursor', got %q", cursor)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{},
		})
	})

	client, err := shared.ClientFactory()
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.SearchLogs(context.Background(), "service:web", "2024-01-15T09:00:00Z", "2024-01-15T10:00:00Z", "", 50, "mycursor", "")
	if err != nil {
		t.Fatalf("SearchLogs with cursor failed: %v", err)
	}
}

func TestLogsAggregate(t *testing.T) {
	shared.SetupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/logs/analytics/aggregate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"buckets": []map[string]any{
					{"by": map[string]any{"service": "web-api"}, "computes": map[string]any{"c0": 42}},
				},
			},
		})
	})

	client, err := shared.ClientFactory()
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.AggregateLogs(context.Background(), "status:error", "2024-01-15T09:00:00Z", "2024-01-15T10:00:00Z", []string{"service"})
	if err != nil {
		t.Fatalf("AggregateLogs failed: %v", err)
	}
	if len(resp.Data.Buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(resp.Data.Buckets))
	}
}
