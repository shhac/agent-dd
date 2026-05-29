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

type TraceSearchRequest struct {
	Data TraceSearchData `json:"data"`
}

type TraceSearchData struct {
	Type       string                `json:"type"`
	Attributes TraceSearchAttributes `json:"attributes"`
}

type TraceSearchAttributes struct {
	Filter TraceFilter `json:"filter"`
	Sort   string      `json:"sort,omitempty"`
	Page   *TracePage  `json:"page,omitempty"`
}

type TraceFilter struct {
	Query string `json:"query"`
	From  string `json:"from"`
	To    string `json:"to"`
}

type TracePage struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
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

	req := TraceSearchRequest{
		Data: TraceSearchData{
			Type: "search_request",
			Attributes: TraceSearchAttributes{
				Filter: TraceFilter{Query: filterQuery, From: from, To: to},
				Sort:   "-timestamp",
			},
		},
	}
	if limit > 0 {
		req.Data.Attributes.Page = &TracePage{Limit: limit}
	}
	if cursor != "" {
		if req.Data.Attributes.Page == nil {
			req.Data.Attributes.Page = &TracePage{}
		}
		req.Data.Attributes.Page.Cursor = cursor
	}

	return doAndDecode[TraceSearchResponse](c, ctx, http.MethodPost, "/v2/spans/events/search", req)
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
