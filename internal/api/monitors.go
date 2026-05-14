package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Monitor represents a Datadog monitor.
type Monitor struct {
	ID       int              `json:"id"`
	Name     string           `json:"name"`
	Type     string           `json:"type"`
	Query    string           `json:"query,omitempty"`
	Message  string           `json:"message,omitempty"`
	Tags     []string         `json:"tags,omitempty"`
	Status   string           `json:"overall_state,omitempty"`
	Created  string           `json:"created,omitempty"`
	Modified string           `json:"modified,omitempty"`
	Options  *json.RawMessage `json:"options,omitempty"`
}

// MonitorCompact is the token-efficient view of a monitor.
type MonitorCompact struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

// MonitorSearchResponse is the /v1/monitor/search envelope. `counts` summarises
// the result set by state/muted/tag/type, useful for triage rollups; `metadata`
// carries pagination info.
type MonitorSearchResponse struct {
	Monitors []Monitor             `json:"monitors"`
	Counts   *MonitorSearchCounts  `json:"counts,omitempty"`
	Metadata *MonitorSearchMetaSet `json:"metadata,omitempty"`
}

// MonitorSearchCounts mirrors the buckets Datadog returns in the search envelope.
// Each entry is { name, count }; we surface them as flat maps so the JSON output
// stays readable without modelling every possible state value.
type MonitorSearchCounts struct {
	Status []MonitorSearchBucket `json:"status,omitempty"`
	Muted  []MonitorSearchBucket `json:"muted,omitempty"`
	Tag    []MonitorSearchBucket `json:"tag,omitempty"`
	Type   []MonitorSearchBucket `json:"type,omitempty"`
}

type MonitorSearchBucket struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type MonitorSearchMetaSet struct {
	Total        int `json:"total"`
	Page         int `json:"page"`
	PerPage      int `json:"per_page"`
	PageCount    int `json:"page_count"`
	TotalResults int `json:"total_results"`
}

func (c *Client) ListMonitors(ctx context.Context, search string, tags []string, status string) ([]Monitor, error) {
	params := url.Values{}
	if search != "" {
		params.Set("name", search)
	}
	for _, tag := range tags {
		params.Add("monitor_tags", tag)
	}
	// Status filtering: Datadog v1 monitor list takes group_states, but the
	// CLI doesn't expose that yet — clients filter client-side instead.

	monitors, err := doAndDecode[[]Monitor](c, ctx, http.MethodGet, buildPath("/v1/monitor", params), nil)
	if err != nil {
		return nil, err
	}

	return filterMonitorsByStatus(*monitors, status), nil
}

func (c *Client) GetMonitor(ctx context.Context, id int) (*Monitor, error) {
	path := fmt.Sprintf("/v1/monitor/%d", id)
	return doAndDecode[Monitor](c, ctx, http.MethodGet, path, nil)
}

// SearchMonitors hits /v1/monitor/search and returns the full envelope so
// callers can surface the result-set rollups (counts by state/muted/tag/type).
// Status filtering is still applied client-side to the returned monitor list.
func (c *Client) SearchMonitors(ctx context.Context, query string, status string) (*MonitorSearchResponse, error) {
	params := url.Values{}
	if query != "" {
		params.Set("query", query)
	}

	resp, err := doAndDecode[MonitorSearchResponse](c, ctx, http.MethodGet, buildPath("/v1/monitor/search", params), nil)
	if err != nil {
		return nil, err
	}
	resp.Monitors = filterMonitorsByStatus(resp.Monitors, status)
	return resp, nil
}

func filterMonitorsByStatus(monitors []Monitor, status string) []Monitor {
	if status == "" {
		return monitors
	}
	filtered := make([]Monitor, 0, len(monitors))
	for _, m := range monitors {
		if m.Status == status {
			filtered = append(filtered, m)
		}
	}
	return filtered
}
