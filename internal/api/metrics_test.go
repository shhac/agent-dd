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

		// Mirror the real /api/v1/query response shape: pointlist + tag_set + scope.
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"series": []map[string]any{
				{
					"metric":    "system.cpu.user",
					"scope":     "host:test-host",
					"tag_set":   []string{"host:test-host"},
					"pointlist": [][]float64{{1000, 42.5}, {1060, 43.1}},
					"interval":  60,
					"length":    2,
					"aggr":      "avg",
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
	s := resp.Series[0]
	if s.Metric != "system.cpu.user" {
		t.Errorf("expected metric=system.cpu.user, got %s", s.Metric)
	}
	if s.Scope != "host:test-host" {
		t.Errorf("expected scope=host:test-host, got %q", s.Scope)
	}
	if len(s.TagSet) != 1 || s.TagSet[0] != "host:test-host" {
		t.Errorf("expected tag_set=[host:test-host], got %v", s.TagSet)
	}
	if len(s.Pointlist) != 2 {
		t.Fatalf("expected 2 points, got %d", len(s.Pointlist))
	}
	if s.Pointlist[0][0] != 1000 || s.Pointlist[0][1] != 42.5 {
		t.Errorf("expected first point [1000,42.5], got %v", s.Pointlist[0])
	}
	if s.Aggr != "avg" {
		t.Errorf("expected aggr=avg, got %s", s.Aggr)
	}
}

// Empty series: a valid query with no matches should decode cleanly to an
// empty slice — nil or panic here would break "no data" handling in the CLI.
func TestQueryMetricsEmptySeries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"series": []any{},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "k", "a")
	resp, err := client.QueryMetrics(context.Background(), "avg:nope{*}", 0, 1)
	if err != nil {
		t.Fatalf("QueryMetrics empty: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status=ok, got %s", resp.Status)
	}
	if len(resp.Series) != 0 {
		t.Errorf("expected empty series, got %d", len(resp.Series))
	}
}

// status:"error" + an error message — the surface is currently best-effort
// (we surface Status but don't fail), so this test pins the contract: the
// caller can read Status and decide what to do.
func TestQueryMetricsErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "error",
			"error":  "query parse error",
			"series": []any{},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "k", "a")
	resp, err := client.QueryMetrics(context.Background(), "garbage", 0, 1)
	if err != nil {
		t.Fatalf("QueryMetrics should not error on status:error response (HTTP 200): %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("expected status=error, got %s", resp.Status)
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
