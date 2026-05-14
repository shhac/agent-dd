package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	agenterrors "github.com/shhac/agent-dd/internal/errors"
)

// Host represents a Datadog host.
type Host struct {
	Name             string              `json:"name"`
	Aliases          []string            `json:"aliases,omitempty"`
	Apps             []string            `json:"apps,omitempty"`
	IsMuted          bool                `json:"is_muted"`
	MuteTimeout      int64               `json:"mute_timeout,omitempty"`
	Sources          []string            `json:"sources,omitempty"`
	Up               bool                `json:"up"`
	TagsBySource     map[string][]string `json:"tags_by_source,omitempty"`
	LastReportedTime int64               `json:"last_reported_time,omitempty"`
}

type HostListResponse struct {
	HostList      []Host `json:"host_list"`
	TotalReturned int    `json:"total_returned"`
	TotalMatching int    `json:"total_matching"`
}

// ListHosts queries /v1/hosts. `filter` is a single string accepting Datadog
// query syntax (not a repeatable param) — search text and tag selectors are
// combined with `AND` into one expression.
func (c *Client) ListHosts(ctx context.Context, search string, tags []string) (*HostListResponse, error) {
	params := url.Values{}
	parts := make([]string, 0, 1+len(tags))
	if search != "" {
		parts = append(parts, search)
	}
	parts = append(parts, tags...)
	if len(parts) > 0 {
		params.Set("filter", strings.Join(parts, " AND "))
	}

	return doAndDecode[HostListResponse](c, ctx, http.MethodGet, buildPath("/v1/hosts", params), nil)
}

func (c *Client) GetHost(ctx context.Context, hostname string) (*Host, error) {
	params := url.Values{"filter": {hostname}}
	path := "/v1/hosts?" + params.Encode()

	resp, err := doAndDecode[HostListResponse](c, ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if len(resp.HostList) == 0 {
		return nil, agenterrors.Newf(agenterrors.FixableByAgent, "host %q not found", hostname).
			WithHint("Check the hostname — use 'agent-dd hosts list' to see available hosts")
	}
	return &resp.HostList[0], nil
}

func (c *Client) MuteHost(ctx context.Context, hostname string, end int64, reason string) error {
	body := map[string]any{
		"hostname": hostname,
	}
	if end > 0 {
		body["end"] = end
	}
	if reason != "" {
		body["message"] = reason
	}

	path := fmt.Sprintf("/v1/host/%s/mute", url.PathEscape(hostname))
	_, err := c.do(ctx, http.MethodPost, path, body)
	return err
}
