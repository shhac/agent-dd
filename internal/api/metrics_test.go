package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-dd/internal/api"
)

func TestQueryMetrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/query" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("query") != "avg:system.cpu.user{*}" {
			t.Errorf("expected query=avg:system.cpu.user{*}, got %q", q.Get("query"))
		}
		if q.Get("from") != "1000" {
			t.Errorf("expected from=1000, got %q", q.Get("from"))
		}
		if q.Get("to") != "2000" {
			t.Errorf("expected to=2000, got %q", q.Get("to"))
		}
		if r.Header.Get("DD-API-KEY") != "test-key" {
			t.Error("missing or wrong DD-API-KEY")
		}

		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"series": []map[string]any{
				{
					"metric": "system.cpu.user",
					"tags":   []string{"host:web01"},
					"points": [][]float64{{1000, 42.5}, {1060, 43.1}},
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "test-key", "test-app")
	resp, err := client.QueryMetrics(context.Background(), "avg:system.cpu.user{*}", 1000, 2000)
	if err != nil {
		t.Fatalf("QueryMetrics failed: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status=ok, got %s", resp.Status)
	}
	if len(resp.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(resp.Series))
	}
	if resp.Series[0].Metric != "system.cpu.user" {
		t.Errorf("expected metric=system.cpu.user, got %s", resp.Series[0].Metric)
	}
	if len(resp.Series[0].Points) != 2 {
		t.Errorf("expected 2 points, got %d", len(resp.Series[0].Points))
	}
}

func TestListMetricsV2(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/metrics" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if q := r.URL.Query().Get("filter[tags]"); q != "env:prod" {
			t.Errorf("expected filter[tags]=env:prod, got %q", q)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "system.cpu.user", "type": "metrics"},
				{"id": "system.mem.used", "type": "metrics"},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	resp, err := client.ListMetrics(context.Background(), "", "env:prod")
	if err != nil {
		t.Fatalf("ListMetrics (v2) failed: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(resp.Data))
	}
	if resp.Data[0].ID != "system.cpu.user" {
		t.Errorf("expected first metric ID=system.cpu.user, got %s", resp.Data[0].ID)
	}
}

func TestListMetricsSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/metrics" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if q := r.URL.Query().Get("filter[metric]"); q != "system.cpu" {
			t.Errorf("expected filter[metric]=system.cpu, got %q", q)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "system.cpu.user", "type": "metrics"},
				{"id": "system.cpu.system", "type": "metrics"},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	resp, err := client.ListMetrics(context.Background(), "system.cpu", "")
	if err != nil {
		t.Fatalf("ListMetrics (search) failed: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(resp.Data))
	}
}

func TestGetMetricMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/metrics/system.cpu.user" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"metric":      "system.cpu.user",
			"type":        "gauge",
			"unit":        "percent",
			"description": "User CPU usage",
			"integration": "system",
			"per_unit":    "second",
			"short_name":  "cpu user",
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	meta, err := client.GetMetricMetadata(context.Background(), "system.cpu.user")
	if err != nil {
		t.Fatalf("GetMetricMetadata failed: %v", err)
	}
	if meta.Name != "system.cpu.user" {
		t.Errorf("expected Name=system.cpu.user, got %s", meta.Name)
	}
	if meta.Type != "gauge" {
		t.Errorf("expected Type=gauge, got %s", meta.Type)
	}
	if meta.Unit != "percent" {
		t.Errorf("expected Unit=percent, got %s", meta.Unit)
	}
	if meta.Description != "User CPU usage" {
		t.Errorf("expected Description=User CPU usage, got %s", meta.Description)
	}
}
