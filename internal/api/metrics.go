package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

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
	if search != "" {
		return c.searchMetricsV1(ctx, search)
	}

	params := url.Values{}
	if tag != "" {
		params.Set("filter[tags]", tag)
	}

	return doAndDecode[MetricListResponse](c, ctx, http.MethodGet, buildPath("/v2/metrics", params), nil)
}

func (c *Client) searchMetricsV1(ctx context.Context, query string) (*MetricListResponse, error) {
	type searchResp struct {
		Results struct {
			Metrics []string `json:"metrics"`
		} `json:"results"`
	}

	params := url.Values{"q": {"metrics:" + query}}
	path := "/v1/search?" + params.Encode()

	resp, err := doAndDecode[searchResp](c, ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	entries := make([]MetricListEntry, len(resp.Results.Metrics))
	for i, m := range resp.Results.Metrics {
		entries[i] = MetricListEntry{ID: m, Type: "metric"}
	}
	return &MetricListResponse{Data: entries}, nil
}

func (c *Client) GetMetricMetadata(ctx context.Context, metricName string) (*MetricMetadata, error) {
	path := fmt.Sprintf("/v1/metrics/%s", url.PathEscape(metricName))
	return doAndDecode[MetricMetadata](c, ctx, http.MethodGet, path, nil)
}
