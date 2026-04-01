package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	agenterrors "github.com/shhac/agent-dd/internal/errors"
)

type TraceSearchResponse struct {
	Data []TraceData `json:"data"`
	Meta *SearchMeta `json:"meta,omitempty"`
}

// Cursor returns the pagination cursor from the response, or empty if none.
func (r *TraceSearchResponse) Cursor() string {
	return CursorFrom(r.Meta)
}

type TraceData struct {
	Type       string          `json:"type"`
	Attributes TraceAttributes `json:"attributes"`
}

type TraceAttributes struct {
	TraceID    string  `json:"trace_id,omitempty"`
	SpanID     string  `json:"span_id,omitempty"`
	Service    string  `json:"service,omitempty"`
	Name       string  `json:"name,omitempty"`
	Resource   string  `json:"resource,omitempty"`
	Type       string  `json:"type,omitempty"`
	Start      int64   `json:"start,omitempty"`
	Duration   float64 `json:"duration,omitempty"`
	Error      int     `json:"error,omitempty"`
	Status     string  `json:"status,omitempty"`
}

func (c *Client) SearchTraces(ctx context.Context, query, service, from, to string, limit int) (*TraceSearchResponse, error) {
	filterQuery := query
	if service != "" {
		filterQuery = strings.TrimSpace("service:" + service + " " + query)
	}

	body := map[string]any{
		"filter": map[string]any{
			"query": filterQuery,
			"from":  from,
			"to":    to,
		},
	}
	if limit > 0 {
		body["page"] = map[string]any{"limit": limit}
	}

	return doAndDecode[TraceSearchResponse](c, ctx, http.MethodPost, "/v2/spans/events/search", body)
}

type ServiceListResponse struct {
	Data []ServiceData `json:"data"`
}

type ServiceData struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Attributes ServiceAttributes `json:"attributes"`
}

type ServiceAttributes struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

func (c *Client) ListServices(ctx context.Context, search string) ([]APMService, error) {
	params := url.Values{}
	if search != "" {
		params.Set("filter", search)
	}

	raw, err := c.do(ctx, http.MethodGet, buildPath("/v1/services", params), nil)
	if err != nil {
		return nil, err
	}

	// v1 services endpoint returns a map
	var serviceMap map[string]any
	if err := json.Unmarshal(raw, &serviceMap); err != nil {
		return nil, agenterrors.Wrap(err, agenterrors.FixableByAgent)
	}

	services := make([]APMService, 0)
	for name := range serviceMap {
		if search == "" || strings.Contains(name, search) {
			services = append(services, APMService{Name: name})
		}
	}
	return services, nil
}
