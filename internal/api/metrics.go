package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// MetricSeries represents a metric query result series.
type MetricSeries struct {
	Metric string      `json:"metric,omitempty"`
	Tags   []string    `json:"tags,omitempty"`
	Points [][]float64 `json:"points"`
}

// MetricMetadata represents metadata about a metric.
type MetricMetadata struct {
	Name        string `json:"metric,omitempty"`
	Type        string `json:"type,omitempty"`
	Unit        string `json:"unit,omitempty"`
	Description string `json:"description,omitempty"`
	Integration string `json:"integration,omitempty"`
	PerUnit     string `json:"per_unit,omitempty"`
	ShortName   string `json:"short_name,omitempty"`
}

type MetricQueryResponse struct {
	Status string         `json:"status"`
	Series []MetricSeries `json:"series"`
}

func (c *Client) QueryMetrics(ctx context.Context, query string, from, to int64) (*MetricQueryResponse, error) {
	params := url.Values{
		"query": {query},
		"from":  {fmt.Sprintf("%d", from)},
		"to":    {fmt.Sprintf("%d", to)},
	}
	path := "/v1/query?" + params.Encode()
	return doAndDecode[MetricQueryResponse](c, ctx, http.MethodGet, path, nil)
}

type MetricListResponse struct {
	Data []MetricListEntry `json:"data"`
}

type MetricListEntry struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func (c *Client) ListMetrics(ctx context.Context, search string, tag string) (*MetricListResponse, error) {
	params := url.Values{}
	if search != "" {
		params.Set("filter[metric]", search)
	}
	if tag != "" {
		params.Set("filter[tags]", tag)
	}

	return doAndDecode[MetricListResponse](c, ctx, http.MethodGet, buildPath("/v2/metrics", params), nil)
}

func (c *Client) GetMetricMetadata(ctx context.Context, metricName string) (*MetricMetadata, error) {
	path := fmt.Sprintf("/v1/metrics/%s", url.PathEscape(metricName))
	return doAndDecode[MetricMetadata](c, ctx, http.MethodGet, path, nil)
}
