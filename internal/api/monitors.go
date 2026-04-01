package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

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

func (c *Client) MuteMonitor(ctx context.Context, id int, end string, reason string) error {
	body := map[string]any{}
	if end != "" {
		body["end"] = end
	}
	if reason != "" {
		body["scope"] = "*"
	}
	path := fmt.Sprintf("/v1/monitor/%d/mute", id)
	_, err := c.do(ctx, http.MethodPost, path, body)
	return err
}

func (c *Client) UnmuteMonitor(ctx context.Context, id int) error {
	path := fmt.Sprintf("/v1/monitor/%d/unmute", id)
	_, err := c.do(ctx, http.MethodPost, path, map[string]any{"scope": "*", "all_scopes": true})
	return err
}
