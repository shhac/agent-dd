package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Downtime struct {
	ID         string              `json:"id"`
	Type       string              `json:"type,omitempty"`
	Attributes *DowntimeAttributes `json:"attributes,omitempty"`
}

type DowntimeAttributes struct {
	Message   string            `json:"message,omitempty"`
	Scope     string            `json:"scope,omitempty"`
	Status    string            `json:"status,omitempty"`
	Schedule  *DowntimeSchedule `json:"schedule,omitempty"`
	MonitorID int               `json:"monitor_identifier,omitempty"`
}

type DowntimeSchedule struct {
	Start *string `json:"start"`
	End   *string `json:"end,omitempty"`
}

type downtimeListResponse struct {
	Data []Downtime `json:"data"`
}

// CreateDowntime creates a v2 downtime to mute a monitor. `end` is a unix
// timestamp in seconds; 0 means indefinite. The v2 downtime API expects
// schedule.{start,end} as ISO-8601 datetime strings (not epoch numbers),
// and `start` is omitted entirely to mean "starts immediately" — sending an
// explicit null was non-spec and risked rejection.
func (c *Client) CreateDowntime(ctx context.Context, monitorID int, end int64, reason string) (*Downtime, error) {
	attrs := map[string]any{
		"message":            reason,
		"scope":              "monitor_id:" + strconv.Itoa(monitorID),
		"monitor_identifier": map[string]any{"monitor_id": monitorID},
	}
	if end > 0 {
		attrs["schedule"] = map[string]any{
			"end": time.Unix(end, 0).UTC().Format(time.RFC3339),
		}
	}

	body := map[string]any{
		"data": map[string]any{
			"type":       "downtime",
			"attributes": attrs,
		},
	}

	return doAndDecodeData[Downtime](c, ctx, http.MethodPost, "/v2/downtime", body)
}

// ListActiveDowntimes returns active downtimes for a specific monitor.
func (c *Client) ListActiveDowntimes(ctx context.Context, monitorID int) ([]Downtime, error) {
	params := url.Values{
		"filter[monitor_id]": {strconv.Itoa(monitorID)},
		"filter[status]":     {"active"},
	}

	resp, err := doAndDecode[downtimeListResponse](c, ctx, http.MethodGet, buildPath("/v2/downtime", params), nil)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// CancelDowntime cancels a downtime by ID.
func (c *Client) CancelDowntime(ctx context.Context, downtimeID string) error {
	_, err := c.do(ctx, http.MethodDelete, "/v2/downtime/"+url.PathEscape(downtimeID), nil)
	return err
}
