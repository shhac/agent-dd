package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	agenterrors "github.com/shhac/agent-dd/internal/errors"
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
	Length      int64       `json:"length,omitempty"`
	Aggr        string      `json:"aggr,omitempty"`
	Start       int64       `json:"start,omitempty"`
	End         int64       `json:"end,omitempty"`
	Expression  string      `json:"expression,omitempty"`
	QueryIndex  int         `json:"query_index,omitempty"`
}

// MetricMetadata represents metadata about a metric returned by /v1/metrics/{name}.
// Datadog does NOT echo the metric name in the response body, so [GetMetricMetadata]
// sets Name from the request argument instead of relying on a JSON tag.
type MetricMetadata struct {
	Name           string `json:"-"`
	Type           string `json:"type,omitempty"`
	Unit           string `json:"unit,omitempty"`
	Description    string `json:"description,omitempty"`
	Integration    string `json:"integration,omitempty"`
	PerUnit        string `json:"per_unit,omitempty"`
	ShortName      string `json:"short_name,omitempty"`
	StatsdInterval int64  `json:"statsd_interval,omitempty"`
}

type MetricQueryResponse struct {
	Status   string         `json:"status"`
	Series   []MetricSeries `json:"series"`
	Error    string         `json:"error,omitempty"`
	Message  string         `json:"message,omitempty"`
	FromDate int64          `json:"from_date,omitempty"`
	ToDate   int64          `json:"to_date,omitempty"`
	GroupBy  []string       `json:"group_by,omitempty"`
	ResType  string         `json:"res_type,omitempty"`
	Query    string         `json:"query,omitempty"`
}

func (c *Client) QueryMetrics(ctx context.Context, query string, from, to int64) (*MetricQueryResponse, error) {
	params := url.Values{
		"query": {query},
		"from":  {strconv.FormatInt(from, 10)},
		"to":    {strconv.FormatInt(to, 10)},
	}
	path := "/v1/query?" + params.Encode()
	resp, err := doAndDecode[MetricQueryResponse](c, ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	// Datadog returns HTTP 200 for query failures (parse errors, unknown
	// metrics, etc) with status="error" and the reason in `error`/`message`.
	// Without surfacing this, callers see an empty series and assume "no data".
	if resp.Status == "error" {
		msg := resp.Error
		if msg == "" {
			msg = resp.Message
		}
		if msg == "" {
			msg = "metric query returned status=error with no detail"
		}
		return resp, agenterrors.New("Metric query failed: "+msg, agenterrors.FixableByAgent).
			WithHint("Check the query syntax with 'agent-dd metrics metadata <name>'")
	}
	return resp, nil
}

type MetricListResponse struct {
	Data []MetricListEntry `json:"data"`
}

type MetricListEntry struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// ListMetrics queries /v2/metrics. The `search` argument filters the returned
// metric IDs client-side using a substring match — the v2 endpoint does not
// expose a server-side name filter (`filter[metric]` is not documented and
// silently ignored).
func (c *Client) ListMetrics(ctx context.Context, search string, tag string) (*MetricListResponse, error) {
	params := url.Values{}
	if tag != "" {
		params.Set("filter[tags]", tag)
	}

	resp, err := doAndDecode[MetricListResponse](c, ctx, http.MethodGet, buildPath("/v2/metrics", params), nil)
	if err != nil {
		return nil, err
	}
	if search != "" {
		filtered := resp.Data[:0]
		for _, m := range resp.Data {
			if strings.Contains(m.ID, search) {
				filtered = append(filtered, m)
			}
		}
		resp.Data = filtered
	}
	return resp, nil
}

func (c *Client) GetMetricMetadata(ctx context.Context, metricName string) (*MetricMetadata, error) {
	path := "/v1/metrics/" + url.PathEscape(metricName)
	meta, err := doAndDecode[MetricMetadata](c, ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	meta.Name = metricName
	return meta, nil
}
