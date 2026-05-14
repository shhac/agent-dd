package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-dd/internal/api"
)

func TestListEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/events" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("start") != "1000" {
			t.Errorf("expected start=1000, got %q", q.Get("start"))
		}
		if q.Get("end") != "2000" {
			t.Errorf("expected end=2000, got %q", q.Get("end"))
		}
		if q.Get("sources") != "nginx" {
			t.Errorf("expected sources=nginx, got %q", q.Get("sources"))
		}
		tags := q["tags"]
		if len(tags) != 2 || tags[0] != "env:prod" || tags[1] != "service:web" {
			t.Errorf("expected tags=[env:prod, service:web], got %v", tags)
		}
		if r.Header.Get("DD-API-KEY") != "test-key" {
			t.Error("missing or wrong DD-API-KEY")
		}
		if r.Header.Get("DD-APPLICATION-KEY") != "test-app" {
			t.Error("missing or wrong DD-APPLICATION-KEY")
		}

		json.NewEncoder(w).Encode(map[string]any{
			"events": []map[string]any{
				{"id": 1, "title": "Deploy started"},
				{"id": 2, "title": "Deploy finished"},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "test-key", "test-app")
	events, err := client.ListEvents(context.Background(), 1000, 2000, "nginx", []string{"env:prod", "service:web"})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Title != "Deploy started" {
		t.Errorf("expected first event title=Deploy started, got %s", events[0].Title)
	}
}

func TestGetEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/events/123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Real DD response uses `source_type_name` (not `source`); `id_str` is
		// the precision-safe string form of `id`. Large IDs (> 2^53) lose
		// precision when round-tripped through JS-flavoured JSON, so id_str
		// must round-trip cleanly even when ID itself does.
		json.NewEncoder(w).Encode(map[string]any{
			"event": map[string]any{
				"id":               9007199254740993, // > Number.MAX_SAFE_INTEGER
				"id_str":           "9007199254740993",
				"title":            "CPU spike",
				"text":             "CPU went above 90%",
				"source_type_name": "nagios",
				"url":              "https://app.datadoghq.com/event/event?id=9007199254740993",
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	event, err := client.GetEvent(context.Background(), 123)
	if err != nil {
		t.Fatalf("GetEvent failed: %v", err)
	}
	if event.IDStr != "9007199254740993" {
		t.Errorf("expected IDStr=9007199254740993, got %s", event.IDStr)
	}
	if event.Title != "CPU spike" {
		t.Errorf("expected title=CPU spike, got %s", event.Title)
	}
	if event.SourceTypeName != "nagios" {
		t.Errorf("expected SourceTypeName=nagios, got %s", event.SourceTypeName)
	}
	if event.URL == "" {
		t.Error("expected URL to be populated")
	}
}
