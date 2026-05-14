package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// Event represents a Datadog event. The documented response uses
// `source_type_name` (not `source`) for the source identifier; `id_str` is
// the string form of `id` (Datadog event IDs exceed JS's 53-bit safe int
// range, so consumers passing the response through JS or to other JSON
// tooling should prefer `id_str` to avoid precision loss).
type Event struct {
	ID             int64    `json:"id"`
	IDStr          string   `json:"id_str,omitempty"`
	Title          string   `json:"title"`
	Text           string   `json:"text,omitempty"`
	DateHappened   int64    `json:"date_happened,omitempty"`
	SourceTypeName string   `json:"source_type_name,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	Priority       string   `json:"priority,omitempty"`
	AlertType      string   `json:"alert_type,omitempty"`
	Host           string   `json:"host,omitempty"`
	URL            string   `json:"url,omitempty"`
	DeviceName     string   `json:"device_name,omitempty"`
}

type EventListResponse struct {
	Events []Event `json:"events"`
}

func (c *Client) ListEvents(ctx context.Context, from, to int64, source string, tags []string) ([]Event, error) {
	params := url.Values{
		"start": {strconv.FormatInt(from, 10)},
		"end":   {strconv.FormatInt(to, 10)},
	}
	if source != "" {
		params.Set("sources", source)
	}
	for _, tag := range tags {
		params.Add("tags", tag)
	}

	path := "/v1/events?" + params.Encode()
	resp, err := doAndDecode[EventListResponse](c, ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return resp.Events, nil
}

func (c *Client) GetEvent(ctx context.Context, id int64) (*Event, error) {
	type eventResp struct {
		Event Event `json:"event"`
	}
	path := "/v1/events/" + strconv.FormatInt(id, 10)
	return doAndDecodeField[eventResp, Event](c, ctx, http.MethodGet, path, nil, func(r *eventResp) *Event { return &r.Event })
}
