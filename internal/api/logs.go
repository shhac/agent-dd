package api

import (
	"context"
	"net/http"
)

// LogEntry represents a Datadog log entry.
type LogEntry struct {
	ID         string            `json:"id"`
	Timestamp  string            `json:"timestamp,omitempty"`
	Service    string            `json:"service,omitempty"`
	Status     string            `json:"status,omitempty"`
	Message    string            `json:"message,omitempty"`
	Host       string            `json:"host,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Attributes map[string]any    `json:"attributes,omitempty"`
}

// LogEntryCompact is the token-efficient view of a log entry.
type LogEntryCompact struct {
	Timestamp string `json:"timestamp"`
	Service   string `json:"service,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

type LogSearchRequest struct {
	Filter *LogFilter `json:"filter"`
	Sort   string     `json:"sort,omitempty"`
	Page   *LogPage   `json:"page,omitempty"`
}

type LogFilter struct {
	Query string `json:"query"`
	From  string `json:"from"`
	To    string `json:"to"`
}

type LogPage struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type LogSearchResponse struct {
	Data []LogData   `json:"data"`
	Meta *SearchMeta `json:"meta,omitempty"`
}

type LogData struct {
	ID         string        `json:"id"`
	Type       string        `json:"type"`
	Attributes LogAttributes `json:"attributes"`
}

type LogAttributes struct {
	Timestamp  string         `json:"timestamp,omitempty"`
	Service    string         `json:"service,omitempty"`
	Status     string         `json:"status,omitempty"`
	Message    string         `json:"message,omitempty"`
	Host       string         `json:"host,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

func (c *Client) SearchLogs(ctx context.Context, query, from, to, sort string, limit int, cursor string) (*LogSearchResponse, error) {
	req := LogSearchRequest{
		Filter: &LogFilter{
			Query: query,
			From:  from,
			To:    to,
		},
	}
	if sort != "" {
		req.Sort = sort
	}
	if limit > 0 || cursor != "" {
		req.Page = &LogPage{Limit: limit, Cursor: cursor}
	}

	return doAndDecode[LogSearchResponse](c, ctx, http.MethodPost, "/v2/logs/events/search", req)
}

// Cursor returns the pagination cursor from the response, or empty if none.
func (r *LogSearchResponse) Cursor() string {
	return CursorFrom(r.Meta)
}

type LogAggregateBucket struct {
	Computes map[string]any `json:"computes,omitempty"`
	By       map[string]any `json:"by,omitempty"`
}

type LogAggregateResponse struct {
	Data struct {
		Buckets []LogAggregateBucket `json:"buckets"`
	} `json:"data"`
}

func (c *Client) AggregateLogs(ctx context.Context, query, from, to string, groupBy []string) (*LogAggregateResponse, error) {
	computes := []map[string]any{
		{"aggregation": "count", "type": "total"},
	}

	groups := make([]map[string]any, 0, len(groupBy))
	for _, g := range groupBy {
		groups = append(groups, map[string]any{
			"facet": g,
			"limit": 10,
			"total": map[string]string{"aggregation": "count", "order": "desc"},
		})
	}

	body := map[string]any{
		"filter": map[string]any{
			"query": query,
			"from":  from,
			"to":    to,
		},
		"compute": computes,
	}
	if len(groups) > 0 {
		body["group_by"] = groups
	}

	return doAndDecode[LogAggregateResponse](c, ctx, http.MethodPost, "/v2/logs/analytics/aggregate", body)
}
