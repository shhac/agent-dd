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

func (c *Client) ListMonitors(ctx context.Context, search string, tags []string, status string) ([]Monitor, error) {
	params := url.Values{}
	if search != "" {
		params.Set("name", search)
	}
	for _, tag := range tags {
		params.Add("monitor_tags", tag)
	}
	if status != "" {
		// Datadog v1 uses group_states to filter
	}

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

func (c *Client) SearchMonitors(ctx context.Context, query string, status string) ([]Monitor, error) {
	params := url.Values{}
	if query != "" {
		params.Set("query", query)
	}

	type searchResp struct {
		Monitors []Monitor `json:"monitors"`
	}
	resp, err := doAndDecode[searchResp](c, ctx, http.MethodGet, buildPath("/v1/monitor/search", params), nil)
	if err != nil {
		return nil, err
	}

	return filterMonitorsByStatus(resp.Monitors, status), nil
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

