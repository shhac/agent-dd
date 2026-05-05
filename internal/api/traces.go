package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// Trace span from APM.
type TraceSpan struct {
	TraceID  string  `json:"trace_id"`
	SpanID   string  `json:"span_id"`
	Service  string  `json:"service,omitempty"`
	Name     string  `json:"name,omitempty"`
	Resource string  `json:"resource,omitempty"`
	Type     string  `json:"type,omitempty"`
	Start    int64   `json:"start,omitempty"`
	Duration float64 `json:"duration,omitempty"`
	Error    int     `json:"error,omitempty"`
	Status   string  `json:"status,omitempty"`
}

// APMService represents an APM service.
type APMService struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

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
	TraceID  string  `json:"trace_id,omitempty"`
	SpanID   string  `json:"span_id,omitempty"`
	Service  string  `json:"service,omitempty"`
	Name     string  `json:"name,omitempty"`
	Resource string  `json:"resource,omitempty"`
	Type     string  `json:"type,omitempty"`
	Start    int64   `json:"start,omitempty"`
	Duration float64 `json:"duration,omitempty"`
	Error    int     `json:"error,omitempty"`
	Status   string  `json:"status,omitempty"`
}

func (c *Client) SearchTraces(ctx context.Context, query, service, from, to string, limit int) (*TraceSearchResponse, error) {
	filterQuery := query
	if service != "" {
		filterQuery = strings.TrimSpace("service:" + service + " " + query)
	}

	// /api/v2/spans/events/search requires the JSON:API envelope:
	// {"data": {"type": "search_request", "attributes": {...}}}.
	// A flat {filter:...} body is rejected with HTTP 400 and the
	// message "document is missing required top-level members".
	attrs := map[string]any{
		"filter": map[string]any{
			"query": filterQuery,
			"from":  from,
			"to":    to,
		},
		"sort": "-timestamp",
	}
	if limit > 0 {
		attrs["page"] = map[string]any{"limit": limit}
	}
	body := map[string]any{
		"data": map[string]any{
			"type":       "search_request",
			"attributes": attrs,
		},
	}

	return doAndDecode[TraceSearchResponse](c, ctx, http.MethodPost, "/v2/spans/events/search", body)
}

type serviceListResponse struct {
	Data struct {
		Attributes struct {
			Services []string `json:"services"`
		} `json:"attributes"`
	} `json:"data"`
}

func (c *Client) ListServices(ctx context.Context, env, search string) ([]APMService, error) {
	params := url.Values{}
	if env != "" {
		params.Set("filter[env]", env)
	} else {
		params.Set("filter[env]", "*")
	}

	resp, err := doAndDecode[serviceListResponse](c, ctx, http.MethodGet, buildPath("/v2/apm/services", params), nil)
	if err != nil {
		return nil, err
	}

	services := make([]APMService, 0, len(resp.Data.Attributes.Services))
	for _, name := range resp.Data.Attributes.Services {
		if search == "" || strings.Contains(name, search) {
			services = append(services, APMService{Name: name})
		}
	}
	return services, nil
}
