package api

import (
	"context"
	"net/http"
	"net/url"
	"regexp"

	agenterrors "github.com/shhac/agent-dd/internal/errors"
)

// Incident represents a Datadog incident. The commander's name/email lives
// in the JSON:API `included` array, not directly on the incident — use the
// `relationships.commander_user.data.id` UUID to look it up. ResolveCommander
// (on IncidentListResponse / IncidentDocument) handles the indirection.
type Incident struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type,omitempty"`
	Attributes    *IncidentAttributes    `json:"attributes,omitempty"`
	Relationships *IncidentRelationships `json:"relationships,omitempty"`
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

type IncidentRelationships struct {
	CommanderUser *IncidentRelation `json:"commander_user,omitempty"`
	CreatedByUser *IncidentRelation `json:"created_by_user,omitempty"`
}

type IncidentRelation struct {
	Data *IncidentRelationData `json:"data,omitempty"`
}

type IncidentRelationData struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// IncludedResource models a single entry in the JSON:API `included` array.
// Attributes is left as a free-form map because the resource shape varies
// by Type (users vs services vs attachments, etc).
type IncludedResource struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// commanderHandle returns the handle/email of the commander_user related to
// `inc`, looked up via `included`. Returns "" if no commander is set.
func commanderHandle(inc *Incident, included []IncludedResource) string {
	if inc == nil || inc.Relationships == nil || inc.Relationships.CommanderUser == nil ||
		inc.Relationships.CommanderUser.Data == nil {
		return ""
	}
	want := inc.Relationships.CommanderUser.Data.ID
	for _, r := range included {
		if r.ID == want && r.Type == "users" {
			if h, _ := r.Attributes["handle"].(string); h != "" {
				return h
			}
			if e, _ := r.Attributes["email"].(string); e != "" {
				return e
			}
			if n, _ := r.Attributes["name"].(string); n != "" {
				return n
			}
		}
	}
	return ""
}

type IncidentListResponse struct {
	Data     []Incident         `json:"data"`
	Included []IncludedResource `json:"included,omitempty"`
	Meta     *IncidentListMeta  `json:"meta,omitempty"`
}

// CommanderHandle returns the commander_user handle for the incident at
// index i, resolved via the response-level Included array. Returns "" if
// the incident has no commander or the included entry is absent.
func (r *IncidentListResponse) CommanderHandle(i int) string {
	if i < 0 || i >= len(r.Data) {
		return ""
	}
	return commanderHandle(&r.Data[i], r.Included)
}

// IncidentDocument is the {data, included} envelope returned by GET
// /v2/incidents/{id}. Mirrors the list shape so commander lookup uses the
// same path.
type IncidentDocument struct {
	Data     Incident           `json:"data"`
	Included []IncludedResource `json:"included,omitempty"`
}

func (d *IncidentDocument) CommanderHandle() string {
	return commanderHandle(&d.Data, d.Included)
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
	// `include=commander_user` expands the commander into the `included`
	// array so callers can resolve the handle without a second /v2/users call.
	params.Set("include", "commander_user")

	return doAndDecode[IncidentListResponse](c, ctx, http.MethodGet, buildPath("/v2/incidents", params), nil)
}

// HasMore returns true if there are more pages of incidents.
func (r *IncidentListResponse) HasMore() bool {
	return r.Meta != nil && r.Meta.Pagination != nil && r.Meta.Pagination.NextOffset > r.Meta.Pagination.Offset
}

// GetIncident returns the {data, included} envelope so the commander handle
// can be resolved without a second call. Use IncidentDocument.CommanderHandle
// to look it up.
func (c *Client) GetIncident(ctx context.Context, id string) (*IncidentDocument, error) {
	path := "/v2/incidents/" + url.PathEscape(id) + "?include=commander_user"
	return doAndDecode[IncidentDocument](c, ctx, http.MethodGet, path, nil)
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
