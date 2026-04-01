package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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

// CreateDowntime creates a v2 downtime to mute a monitor.
func (c *Client) CreateDowntime(ctx context.Context, monitorID int, end int64, reason string) (*Downtime, error) {
	schedule := map[string]any{
		"start": nil,
	}
	if end > 0 {
		schedule["end"] = fmt.Sprintf("%d", end)
	}

	body := map[string]any{
		"data": map[string]any{
			"type": "downtime",
			"attributes": map[string]any{
				"message":            reason,
				"scope":              fmt.Sprintf("monitor_id:%d", monitorID),
				"monitor_identifier": map[string]any{"monitor_id": monitorID},
				"schedule":           schedule,
			},
		},
	}

	type resp struct {
		Data Downtime `json:"data"`
	}
	return doAndDecodeField[resp, Downtime](c, ctx, http.MethodPost, "/v2/downtime", body, func(r *resp) *Downtime { return &r.Data })
}

// ListActiveDowntimes returns active downtimes for a specific monitor.
func (c *Client) ListActiveDowntimes(ctx context.Context, monitorID int) ([]Downtime, error) {
	params := url.Values{
		"filter[monitor_id]": {fmt.Sprintf("%d", monitorID)},
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
