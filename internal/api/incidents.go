package api

import (
	"context"
	"net/http"
	"net/url"
)

type IncidentListResponse struct {
	Data []Incident         `json:"data"`
	Meta *IncidentListMeta  `json:"meta,omitempty"`
}

type IncidentListMeta struct {
	Pagination *IncidentPagination `json:"pagination,omitempty"`
}

type IncidentPagination struct {
	Offset     int `json:"offset"`
	NextOffset int `json:"next_offset"`
	Size       int `json:"size"`
}

func (c *Client) ListIncidents(ctx context.Context, status string) (*IncidentListResponse, error) {
	params := url.Values{}
	if status != "" {
		params.Set("filter[status]", status)
	}

	return doAndDecode[IncidentListResponse](c, ctx, http.MethodGet, buildPath("/v2/incidents", params), nil)
}

// HasMore returns true if there are more pages of incidents.
func (r *IncidentListResponse) HasMore() bool {
	return r.Meta != nil && r.Meta.Pagination != nil && r.Meta.Pagination.NextOffset > r.Meta.Pagination.Offset
}

func (c *Client) GetIncident(ctx context.Context, id string) (*Incident, error) {
	type resp struct {
		Data Incident `json:"data"`
	}
	return doAndDecodeField[resp, Incident](c, ctx, http.MethodGet, "/v2/incidents/"+url.PathEscape(id), nil, func(r *resp) *Incident { return &r.Data })
}

func (c *Client) CreateIncident(ctx context.Context, title, severity, commanderHandle string) (*Incident, error) {
	data := map[string]any{
		"type": "incidents",
		"attributes": map[string]any{
			"title": title,
			"fields": map[string]any{
				"severity": map[string]any{
					"type":  "dropdown",
					"value": severity,
				},
			},
		},
	}

	if commanderHandle != "" {
		data["relationships"] = map[string]any{
			"commander_user": map[string]any{
				"data": map[string]any{
					"type": "users",
					"id":   commanderHandle,
				},
			},
		}
	}

	type resp struct {
		Data Incident `json:"data"`
	}
	return doAndDecodeField[resp, Incident](c, ctx, http.MethodPost, "/v2/incidents", map[string]any{"data": data}, func(r *resp) *Incident { return &r.Data })
}

func (c *Client) UpdateIncident(ctx context.Context, id string, status, severity string) (*Incident, error) {
	attrs := map[string]any{}
	if status != "" {
		attrs["status"] = status
	}
	if severity != "" {
		attrs["fields"] = map[string]any{
			"severity": map[string]any{
				"type":  "dropdown",
				"value": severity,
			},
		}
	}

	body := map[string]any{
		"data": map[string]any{
			"type":       "incidents",
			"id":         id,
			"attributes": attrs,
		},
	}

	type resp struct {
		Data Incident `json:"data"`
	}
	return doAndDecodeField[resp, Incident](c, ctx, http.MethodPatch, "/v2/incidents/"+url.PathEscape(id), body, func(r *resp) *Incident { return &r.Data })
}
