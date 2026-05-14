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

type TraceAttributes struct {
	TraceID        string     `json:"trace_id,omitempty"`
	SpanID         string     `json:"span_id,omitempty"`
	Service        string     `json:"service,omitempty"`
	OperationName  string     `json:"operation_name,omitempty"`
	ResourceName   string     `json:"resource_name,omitempty"`
	Type           string     `json:"type,omitempty"`
	StartTimestamp string     `json:"start_timestamp,omitempty"`
	EndTimestamp   string     `json:"end_timestamp,omitempty"`
	Env            string     `json:"env,omitempty"`
	Tags           []string   `json:"tags,omitempty"`
	Error          *SpanError `json:"error,omitempty"`
	Status         string     `json:"status,omitempty"`
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

// SpanAggregateBucket is a single result bucket from the aggregate endpoint.
type SpanAggregateBucket struct {
	By      map[string]string  `json:"by"`
	Compute map[string]float64 `json:"compute"`
}

type spanAggregateData struct {
	Attributes struct {
		By      map[string]string  `json:"by"`
		Compute map[string]float64 `json:"compute"`
	} `json:"attributes"`
}

type spanAggregateResponse struct {
	Data []spanAggregateData `json:"data"`
}

// AggregateSpans calls POST /v2/spans/analytics/aggregate. aggregation must be
// one of: count, sum, min, max, pc75, pc90, pc95, pc98, pc99.
func (c *Client) AggregateSpans(ctx context.Context, query, from, to, aggregation, metric string, groupBy []string) ([]SpanAggregateBucket, error) {
	compute := map[string]any{
		"aggregation": aggregation,
		"type":        "total",
	}
	if metric != "" {
		compute["metric"] = metric
	}

	groups := make([]map[string]any, len(groupBy))
	for i, facet := range groupBy {
		groups[i] = map[string]any{
			"facet": facet,
			"limit": 100,
			"sort": map[string]any{
				"type":        "measure",
				"order":       "desc",
				"aggregation": aggregation,
				"metric":      metric,
			},
		}
	}

	body := map[string]any{
		"data": map[string]any{
			"type": "aggregate_request",
			"attributes": map[string]any{
				"filter":   map[string]any{"query": query, "from": from, "to": to},
				"compute":  []any{compute},
				"group_by": groups,
			},
		},
	}

	resp, err := doAndDecode[spanAggregateResponse](c, ctx, http.MethodPost, "/v2/spans/analytics/aggregate", body)
	if err != nil {
		return nil, err
	}

	buckets := make([]SpanAggregateBucket, len(resp.Data))
	for i, d := range resp.Data {
		buckets[i] = SpanAggregateBucket{
			By:      d.Attributes.By,
			Compute: d.Attributes.Compute,
		}
	}
	return buckets, nil
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
