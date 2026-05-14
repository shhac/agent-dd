package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-dd/internal/api"
)

func TestListSLOs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/slo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("query") != "api-latency" {
			t.Errorf("expected query=api-latency, got %q", q.Get("query"))
		}
		tags := q["tags_query"]
		if len(tags) != 1 || tags[0] != "team:platform" {
			t.Errorf("expected tags_query=[team:platform], got %v", tags)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":   "slo-abc",
					"name": "API Latency SLO",
					"type": "metric",
					"thresholds": []map[string]any{
						{"timeframe": "30d", "target": 99.9},
					},
				},
				{
					"id":   "slo-def",
					"name": "Uptime SLO",
					"type": "monitor",
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	slos, err := client.ListSLOs(context.Background(), "api-latency", []string{"team:platform"})
	if err != nil {
		t.Fatalf("ListSLOs failed: %v", err)
	}
	if len(slos) != 2 {
		t.Fatalf("expected 2 SLOs, got %d", len(slos))
	}
	if slos[0].ID != "slo-abc" {
		t.Errorf("expected first SLO ID=slo-abc, got %s", slos[0].ID)
	}
	if slos[0].Name != "API Latency SLO" {
		t.Errorf("expected first SLO name=API Latency SLO, got %s", slos[0].Name)
	}
}

func TestGetSLO(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/slo/slo-abc" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":          "slo-abc",
				"name":        "API Latency SLO",
				"type":        "metric",
				"description": "Tracks p99 latency",
				"thresholds": []map[string]any{
					{"timeframe": "30d", "target": 99.9, "warning": 99.95},
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	slo, err := client.GetSLO(context.Background(), "slo-abc")
	if err != nil {
		t.Fatalf("GetSLO failed: %v", err)
	}
	if slo.ID != "slo-abc" {
		t.Errorf("expected ID=slo-abc, got %s", slo.ID)
	}
	if slo.Name != "API Latency SLO" {
		t.Errorf("expected name=API Latency SLO, got %s", slo.Name)
	}
	if slo.Description != "Tracks p99 latency" {
		t.Errorf("expected description=Tracks p99 latency, got %s", slo.Description)
	}
}

// /v1/slo/{id}/history returns:
//   data.overall      → SLO-history-SLI data (sli_value/uptime/precision/...)
//   data.thresholds   → map keyed by timeframe, values are SLOThreshold
//                       (timeframe/target/warning) — NOT per-window SLI metrics.
// Previously Thresholds was typed as map[string]SLOHistoryMetrics, so the
// threshold map decoded into structs with all-zero fields. This test pins
// the real wire shape.
func TestGetSLOHistory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/slo/slo-abc/history" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("from_ts") != "1000" {
			t.Errorf("expected from_ts=1000, got %q", q.Get("from_ts"))
		}
		if q.Get("to_ts") != "2000" {
			t.Errorf("expected to_ts=2000, got %q", q.Get("to_ts"))
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"from_ts": 1000,
				"to_ts":   2000,
				"type":    "metric",
				"overall": map[string]any{
					"sli_value":              99.95,
					"uptime":                 99.99,
					"span_precision":         2.0,
					"error_budget_remaining": 0.05,
				},
				"thresholds": map[string]any{
					"30d": map[string]any{"timeframe": "30d", "target": 99.9, "warning": 99.95},
					"7d":  map[string]any{"timeframe": "7d", "target": 99.0},
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	history, err := client.GetSLOHistory(context.Background(), "slo-abc", 1000, 2000)
	if err != nil {
		t.Fatalf("GetSLOHistory failed: %v", err)
	}
	if history.Overall == nil {
		t.Fatal("expected non-nil Overall")
	}
	if history.Overall.SLIValue != 99.95 {
		t.Errorf("expected SLIValue=99.95, got %f", history.Overall.SLIValue)
	}
	if history.Overall.Uptime != 99.99 {
		t.Errorf("expected Uptime=99.99, got %f", history.Overall.Uptime)
	}
	if history.Overall.SpanPrecision != 2.0 {
		t.Errorf("expected SpanPrecision=2.0, got %f", history.Overall.SpanPrecision)
	}
	if history.FromTs != 1000 || history.ToTs != 2000 {
		t.Errorf("expected FromTs=1000 ToTs=2000, got %d/%d", history.FromTs, history.ToTs)
	}
	if history.Type != "metric" {
		t.Errorf("expected Type=metric, got %q", history.Type)
	}
	if len(history.Thresholds) != 2 {
		t.Fatalf("expected 2 thresholds, got %d", len(history.Thresholds))
	}
	th30 := history.Thresholds["30d"]
	if th30.Timeframe != "30d" || th30.Target != 99.9 || th30.Warning != 99.95 {
		t.Errorf("30d threshold = %+v, want {30d 99.9 99.95}", th30)
	}
}
