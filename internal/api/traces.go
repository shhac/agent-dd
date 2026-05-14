package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// SpanError holds error details returned by the v2 spans events API in the
// `error` attribute. v1's int flag form does not appear on this endpoint.
type SpanError struct {
	Message string `json:"message,omitempty"`
	Type    string `json:"type,omitempty"`
	Stack   string `json:"stack,omitempty"`
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

// TraceAttributes mirrors `data[].attributes` from /v2/spans/events/search.
// The documented v2 schema includes most fields here plus a free-form
// `attributes` blob carrying custom per-span data (HTTP method, db.statement,
// error stack tags, etc) — `Attributes` and `Custom` are the rich-detail
// fields the search response already returns. There is no separate
// "get span by ID" endpoint; search responses are self-contained.
type TraceAttributes struct {
	TraceID        string         `json:"trace_id,omitempty"`
	SpanID         string         `json:"span_id,omitempty"`
	ParentID       string         `json:"parent_id,omitempty"`
	Service        string         `json:"service,omitempty"`
	OperationName  string         `json:"operation_name,omitempty"`
	ResourceName   string         `json:"resource_name,omitempty"`
	ResourceHash   string         `json:"resource_hash,omitempty"`
	Type           string         `json:"type,omitempty"`
	StartTimestamp string         `json:"start_timestamp,omitempty"`
	EndTimestamp   string         `json:"end_timestamp,omitempty"`
	Env            string         `json:"env,omitempty"`
	Host           string         `json:"host,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
	Error          *SpanError     `json:"error,omitempty"`
	Status         string         `json:"status,omitempty"`
	SingleSpan     bool           `json:"single_span,omitempty"`
	Attributes     map[string]any `json:"attributes,omitempty"`
	Custom         map[string]any `json:"custom,omitempty"`
}

func (c *Client) SearchTraces(ctx context.Context, query, service, from, to string, limit int, cursor string) (*TraceSearchResponse, error) {
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
	page := map[string]any{}
	if limit > 0 {
		page["limit"] = limit
	}
	if cursor != "" {
		page["cursor"] = cursor
	}
	if len(page) > 0 {
		attrs["page"] = page
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
