package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// SLO represents a Service Level Objective.
type SLO struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Type        string         `json:"type,omitempty"`
	Description string         `json:"description,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Thresholds  []SLOThreshold `json:"thresholds,omitempty"`
	Status      *SLOStatus     `json:"overall_status,omitempty"`
}

type SLOThreshold struct {
	Timeframe string  `json:"timeframe"`
	Target    float64 `json:"target"`
	Warning   float64 `json:"warning,omitempty"`
}

type SLOStatus struct {
	Status               float64 `json:"status,omitempty"`
	ErrorBudgetRemaining float64 `json:"error_budget_remaining,omitempty"`
}

// SLOHistory represents SLO history data.
type SLOHistory struct {
	Overall    *SLOHistoryMetrics           `json:"overall,omitempty"`
	Thresholds map[string]SLOHistoryMetrics `json:"thresholds,omitempty"`
}

type SLOHistoryMetrics struct {
	SLIValue             float64 `json:"sli_value,omitempty"`
	SpanPrecision        float64 `json:"span_precision,omitempty"`
	Uptime               float64 `json:"uptime,omitempty"`
	ErrorBudgetRemaining float64 `json:"error_budget_remaining,omitempty"`
}

type SLOListResponse struct {
	Data []SLO `json:"data"`
}

func (c *Client) ListSLOs(ctx context.Context, search string, tags []string) ([]SLO, error) {
	params := url.Values{}
	if search != "" {
		params.Set("query", search)
	}
	for _, tag := range tags {
		params.Add("tags_query", tag)
	}

	resp, err := doAndDecode[SLOListResponse](c, ctx, http.MethodGet, buildPath("/v1/slo", params), nil)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetSLO(ctx context.Context, id string) (*SLO, error) {
	path := "/v1/slo/" + url.PathEscape(id)
	return doAndDecodeData[SLO](c, ctx, http.MethodGet, path, nil)
}

func (c *Client) GetSLOHistory(ctx context.Context, id string, from, to int64) (*SLOHistory, error) {
	params := url.Values{
		"from_ts": {strconv.FormatInt(from, 10)},
		"to_ts":   {strconv.FormatInt(to, 10)},
	}
	path := "/v1/slo/" + url.PathEscape(id) + "/history?" + params.Encode()
	return doAndDecodeData[SLOHistory](c, ctx, http.MethodGet, path, nil)
}
