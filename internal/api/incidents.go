package api

import (
	"context"
	"net/http"
	"net/url"
	"regexp"

	agenterrors "github.com/shhac/agent-dd/internal/errors"
)

// Incident represents a Datadog incident.
type Incident struct {
	ID         string              `json:"id"`
	Type       string              `json:"type,omitempty"`
	Attributes *IncidentAttributes `json:"attributes,omitempty"`
}

// IncidentAttributes mirrors the v2 incident attributes object. The lifecycle
// field is `state` (active/stable/resolved), not `status` — Datadog renamed
// it in the v2 API and never populated the old name.
type IncidentAttributes struct {
	Title            string `json:"title,omitempty"`
	State            string `json:"state,omitempty"`
	Severity         string `json:"severity,omitempty"`
	PublicID         int64  `json:"public_id,omitempty"`
	CustomerImpacted bool   `json:"customer_impacted,omitempty"`
	Created          string `json:"created,omitempty"`
	Modified         string `json:"modified,omitempty"`
}

type IncidentListResponse struct {
	Data []Incident        `json:"data"`
	Meta *IncidentListMeta `json:"meta,omitempty"`
}

type IncidentListMeta struct {
	Pagination *IncidentPagination `json:"pagination,omitempty"`
}

type IncidentPagination struct {
	Offset     int `json:"offset"`
	NextOffset int `json:"next_offset"`
	Size       int `json:"size"`
}

func (c *Client) ListIncidents(ctx context.Context, state string) (*IncidentListResponse, error) {
	params := url.Values{}
	if state != "" {
		params.Set("filter[state]", state)
	}

	return doAndDecode[IncidentListResponse](c, ctx, http.MethodGet, buildPath("/v2/incidents", params), nil)
}

// HasMore returns true if there are more pages of incidents.
func (r *IncidentListResponse) HasMore() bool {
	return r.Meta != nil && r.Meta.Pagination != nil && r.Meta.Pagination.NextOffset > r.Meta.Pagination.Offset
}

func (c *Client) GetIncident(ctx context.Context, id string) (*Incident, error) {
	return doAndDecodeData[Incident](c, ctx, http.MethodGet, "/v2/incidents/"+url.PathEscape(id), nil)
}

// uuidPattern recognises the canonical 8-4-4-4-12 hex UUID shape used for
// Datadog user IDs. The v2 incidents API rejects anything else here.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// CreateIncident creates a v2 incident.
//
// `customerImpacted` is required by the API (boolean, no default). `severity`
// goes in `attributes.severity` per spec — the older `fields.severity` shape
// is undocumented. `commanderUUID` must be a Datadog user UUID, not a handle
// or email; passing a non-UUID returns an error before the request fires
// because DD silently rejects it server-side with a 400.
func (c *Client) CreateIncident(ctx context.Context, title, severity, commanderUUID string, customerImpacted bool) (*Incident, error) {
	attrs := map[string]any{
		"title":             title,
		"customer_impacted": customerImpacted,
	}
	if severity != "" {
		attrs["severity"] = severity
	}

	data := map[string]any{
		"type":       "incidents",
		"attributes": attrs,
	}

	if commanderUUID != "" {
		if !uuidPattern.MatchString(commanderUUID) {
			return nil, agenterrors.New("commander must be a Datadog user UUID, not a handle/email", agenterrors.FixableByAgent).
				WithHint("Find the UUID in the Datadog UI under Team > Users, or omit --commander-uuid to leave unassigned")
		}
		data["relationships"] = map[string]any{
			"commander_user": map[string]any{
				"data": map[string]any{
					"type": "users",
					"id":   commanderUUID,
				},
			},
		}
	}

	return doAndDecodeData[Incident](c, ctx, http.MethodPost, "/v2/incidents", map[string]any{"data": data})
}

// UpdateIncident patches a v2 incident. The lifecycle field is `state` and
// the canonical write path is `fields.state.value` — `attributes.status` was
// the legacy v1 shape and is a silent no-op on v2.
func (c *Client) UpdateIncident(ctx context.Context, id string, state, severity string) (*Incident, error) {
	attrs := map[string]any{}
	fields := map[string]any{}
	if state != "" {
		fields["state"] = map[string]any{
			"type":  "dropdown",
			"value": state,
		}
	}
	if severity != "" {
		attrs["severity"] = severity
	}
	if len(fields) > 0 {
		attrs["fields"] = fields
	}

	body := map[string]any{
		"data": map[string]any{
			"type":       "incidents",
			"id":         id,
			"attributes": attrs,
		},
	}

	return doAndDecodeData[Incident](c, ctx, http.MethodPatch, "/v2/incidents/"+url.PathEscape(id), body)
}
