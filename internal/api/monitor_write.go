package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// Monitor writes deal in map[string]any rather than the Monitor struct.
// Datadog monitors carry far more configuration than this CLI models, and an
// update has to write back everything it read — see monitor_merge.go for why.
// Create and validate use the same representation so a definition can be built
// once and sent to either endpoint.

// CreateMonitor posts a new monitor definition. `type` and `query` are required
// by Datadog; the caller is responsible for supplying them.
func (c *Client) CreateMonitor(ctx context.Context, definition map[string]any) (map[string]any, error) {
	return c.doAndDecodeMonitor(ctx, http.MethodPost, "/v1/monitor", definition)
}

// GetMonitorRaw fetches a monitor as its decoded JSON object rather than a
// Monitor. `monitors update` needs every field, including the ones this CLI has
// no struct field for, because it writes the whole object back.
func (c *Client) GetMonitorRaw(ctx context.Context, id int) (map[string]any, error) {
	return c.doAndDecodeMonitor(ctx, http.MethodGet, monitorPath(id), nil)
}

// MonitorUpdate is a prepared read-modify-write: the definition that would be
// sent, and what it changes relative to what is there now.
type MonitorUpdate struct {
	// Definition is the full monitor to write — every field that was read
	// back, with the caller's changes layered on.
	Definition map[string]any
	// Changes lists only what actually differs, keyed by dotted path.
	Changes map[string]FieldChange
}

// PrepareMonitorUpdate reads a monitor and layers `changes` onto it, returning
// the definition that would be written and a diff — without writing anything.
//
// The caller decides what to do with the result: PUT it, or send it to the
// validate endpoint for a dry run. Building it here keeps the "don't destroy
// unmodelled fields" rule in one place rather than in each command.
func (c *Client) PrepareMonitorUpdate(ctx context.Context, id int, changes map[string]any) (*MonitorUpdate, error) {
	existing, err := c.GetMonitorRaw(ctx, id)
	if err != nil {
		return nil, err
	}

	writable := stripReadOnlyMonitorFields(existing)
	updated := applyMonitorUpdates(writable, changes)

	return &MonitorUpdate{
		Definition: updated,
		Changes:    diffMonitorFields(writable, updated),
	}, nil
}

// UpdateMonitor writes a complete monitor definition back to an existing ID.
func (c *Client) UpdateMonitor(ctx context.Context, id int, definition map[string]any) (map[string]any, error) {
	return c.doAndDecodeMonitor(ctx, http.MethodPut, monitorPath(id), definition)
}

// doAndDecodeMonitor decodes into the untyped monitor representation these
// endpoints deal in, saving each caller the wrapper-deref dance.
func (c *Client) doAndDecodeMonitor(ctx context.Context, method, path string, body any) (map[string]any, error) {
	monitor, err := doAndDecode[map[string]any](c, ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	return *monitor, nil
}

// DeleteMonitor removes a monitor. Datadog refuses when the monitor is
// referenced by another resource (an SLO, or a composite monitor) unless force
// is set; that refusal is a real safety signal, so the caller has to opt in
// rather than it being sent by default.
func (c *Client) DeleteMonitor(ctx context.Context, id int, force bool) error {
	params := url.Values{}
	if force {
		params.Set("force", "true")
	}
	_, err := c.do(ctx, http.MethodDelete, buildPath(monitorPath(id), params), nil)
	return err
}

// ValidateMonitor asks Datadog whether a definition is well-formed without
// creating anything. This is what backs `--dry-run`: the query is parsed by the
// same engine that would run it, so a malformed one is caught before any state
// changes.
func (c *Client) ValidateMonitor(ctx context.Context, definition map[string]any) error {
	_, err := c.do(ctx, http.MethodPost, "/v1/monitor/validate", definition)
	return err
}

// ValidateExistingMonitor is ValidateMonitor for an edit to a monitor that
// already exists.
func (c *Client) ValidateExistingMonitor(ctx context.Context, id int, definition map[string]any) error {
	_, err := c.do(ctx, http.MethodPost, monitorPath(id)+"/validate", definition)
	return err
}

func monitorPath(id int) string {
	return fmt.Sprintf("/v1/monitor/%d", id)
}
