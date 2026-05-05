package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// MetricSeries represents a metric query result series as returned by
// the v1 /query endpoint. The Datadog response uses `pointlist`, `scope`
// and `tag_set` (not `points`/`tags`); previously mapping these to the
// wrong JSON tags caused values to deserialize as nil.
type MetricSeries struct {
	Metric      string      `json:"metric,omitempty"`
	DisplayName string      `json:"display_name,omitempty"`
	Scope       string      `json:"scope,omitempty"`
	TagSet      []string    `json:"tag_set,omitempty"`
	Pointlist   [][]float64 `json:"pointlist,omitempty"`
	Interval    int64       `json:"interval,omitempty"`
	Length      int         `json:"length,omitempty"`
	Aggr        string      `json:"aggr,omitempty"`
	Start       float64     `json:"start,omitempty"`
	End         float64     `json:"end,omitempty"`
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
		"from":  {strconv.FormatInt(from, 10)},
		"to":    {strconv.FormatInt(to, 10)},
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
	path := "/v1/metrics/" + url.PathEscape(metricName)
	return doAndDecode[MetricMetadata](c, ctx, http.MethodGet, path, nil)
}
