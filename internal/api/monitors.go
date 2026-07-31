package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Monitor represents a Datadog monitor. `Muted`, `LastTriggeredTs`, and
// `Priority` are present on both the v1 monitor object and the search-result
// monitor object — they're directly useful for triage and were previously
// decoded as zero values because they were missing from the struct.
type Monitor struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Query   string   `json:"query,omitempty"`
	Message string   `json:"message,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	// Emitted as `status`, matching MonitorCompact and the CLI's documented
	// vocabulary. Datadog names it `overall_state` on /v1/monitor and `status`
	// on /v1/monitor/search; UnmarshalJSON accepts either, and output settles
	// on one so a caller never sees the key change between commands.
	Status          string           `json:"status,omitempty"`
	Muted           bool             `json:"muted,omitempty"`
	Priority        int              `json:"priority,omitempty"`
	LastTriggeredTs int64            `json:"last_triggered_ts,omitempty"`
	Created         string           `json:"created,omitempty"`
	Modified        string           `json:"modified,omitempty"`
	Options         *json.RawMessage `json:"options,omitempty"`
}

// UnmarshalJSON reconciles the two shapes Datadog returns for the same entity.
// /v1/monitor and /v1/monitor/{id} report state as `overall_state` with RFC3339
// timestamps; /v1/monitor/search reports it as `status` with epoch-second
// timestamps and omits `overall_state` entirely. Decoding both into one struct
// naively failed the search response outright, and would have left Status empty
// even if it hadn't — so `--status` on a search matched nothing.
//
// Timestamps normalise to RFC3339 so a caller cannot tell which endpoint a
// monitor came from.
func (m *Monitor) UnmarshalJSON(data []byte) error {
	// alias drops the custom unmarshaller so the embedded decode doesn't recurse.
	type alias Monitor
	aux := struct {
		Created      json.RawMessage `json:"created"`
		Modified     json.RawMessage `json:"modified"`
		OverallState string          `json:"overall_state"`
		SearchStatus string          `json:"status"`
		*alias
	}{alias: (*alias)(m)}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	m.Status = aux.OverallState
	if m.Status == "" {
		m.Status = aux.SearchStatus
	}
	m.Created = decodeMonitorTime(aux.Created)
	m.Modified = decodeMonitorTime(aux.Modified)
	return nil
}

// decodeMonitorTime accepts either an RFC3339 string or epoch seconds and
// always yields a string, so both endpoints report timestamps the same way.
func decodeMonitorTime(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}

	var asEpoch int64
	if err := json.Unmarshal(raw, &asEpoch); err == nil && asEpoch > 0 {
		return time.Unix(asEpoch, 0).UTC().Format(time.RFC3339)
	}
	return ""
}

// MonitorCompact is the token-efficient view of a monitor. Muted and
// LastTriggeredTs are included because they're high-signal for triage:
// "is this firing right now / when did it last fire" answers the most
// common follow-up question in an alert-triage loop.
type MonitorCompact struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	Type            string `json:"type"`
	Muted           bool   `json:"muted,omitempty"`
	Priority        int    `json:"priority,omitempty"`
	LastTriggeredTs int64  `json:"last_triggered_ts,omitempty"`
}

// MonitorSearchResponse is the /v1/monitor/search envelope. `counts` summarises
// the result set by state/muted/tag/type, useful for triage rollups; `metadata`
// carries pagination info.
type MonitorSearchResponse struct {
	Monitors []Monitor             `json:"monitors"`
	Counts   *MonitorSearchCounts  `json:"counts,omitempty"`
	Metadata *MonitorSearchMetaSet `json:"metadata,omitempty"`
}

// MonitorSearchCounts mirrors the buckets Datadog returns in the search envelope.
// Each entry is { name, count }; we surface them as flat maps so the JSON output
// stays readable without modelling every possible state value.
type MonitorSearchCounts struct {
	Status []MonitorSearchBucket `json:"status,omitempty"`
	Muted  []MonitorSearchBucket `json:"muted,omitempty"`
	Tag    []MonitorSearchBucket `json:"tag,omitempty"`
	Type   []MonitorSearchBucket `json:"type,omitempty"`
}

// MonitorSearchBucket is one {name, count} entry. `name` is not consistently
// typed across buckets: `status`, `type` and `tag` name themselves with
// strings, but `muted` names itself with a bool. Decoding straight into a
// string field therefore failed the whole response — `monitors search` was
// unusable against any real org. Normalise to a string so callers get one
// contract regardless of which bucket they're reading.
type MonitorSearchBucket struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (b *MonitorSearchBucket) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name  any `json:"name"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	b.Count = raw.Count
	b.Name = bucketName(raw.Name)
	return nil
}

func bucketName(value any) string {
	switch name := value.(type) {
	case nil:
		return ""
	case string:
		return name
	case bool:
		return strconv.FormatBool(name)
	case float64:
		return strconv.FormatFloat(name, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", name)
	}
}

type MonitorSearchMetaSet struct {
	Total        int `json:"total"`
	Page         int `json:"page"`
	PerPage      int `json:"per_page"`
	PageCount    int `json:"page_count"`
	TotalResults int `json:"total_results"`
}

func (c *Client) ListMonitors(ctx context.Context, search string, tags []string, status string) ([]Monitor, error) {
	params := url.Values{}
	if search != "" {
		params.Set("name", search)
	}
	for _, tag := range tags {
		params.Add("monitor_tags", tag)
	}
	// Status filtering: Datadog v1 monitor list takes group_states, but the
	// CLI doesn't expose that yet — clients filter client-side instead.

	monitors, err := doAndDecode[[]Monitor](c, ctx, http.MethodGet, buildPath("/v1/monitor", params), nil)
	if err != nil {
		return nil, err
	}

	return filterMonitorsByStatus(*monitors, status), nil
}

func (c *Client) GetMonitor(ctx context.Context, id int) (*Monitor, error) {
	path := fmt.Sprintf("/v1/monitor/%d", id)
	return doAndDecode[Monitor](c, ctx, http.MethodGet, path, nil)
}

// SearchMonitors hits /v1/monitor/search and returns the full envelope so
// callers can surface the result-set rollups (counts by state/muted/tag/type).
// Status filtering is still applied client-side to the returned monitor list.
func (c *Client) SearchMonitors(ctx context.Context, query string, status string) (*MonitorSearchResponse, error) {
	params := url.Values{}
	if query != "" {
		params.Set("query", query)
	}

	resp, err := doAndDecode[MonitorSearchResponse](c, ctx, http.MethodGet, buildPath("/v1/monitor/search", params), nil)
	if err != nil {
		return nil, err
	}
	resp.Monitors = filterMonitorsByStatus(resp.Monitors, status)
	return resp, nil
}

// filterMonitorsByStatus matches on a normalised form because the two sides
// spell the same state differently: Datadog returns "Alert", "OK", "No Data",
// "Warn", while the CLI has always documented --status as alert|warn|ok|no_data.
// Comparing them raw meant `--status alert` matched nothing against a real org.
func filterMonitorsByStatus(monitors []Monitor, status string) []Monitor {
	if status == "" {
		return monitors
	}
	want := normalizeMonitorStatus(status)
	filtered := make([]Monitor, 0, len(monitors))
	for _, m := range monitors {
		if normalizeMonitorStatus(m.Status) == want {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// normalizeMonitorStatus folds case and treats spaces and underscores alike,
// so "No Data", "no_data" and "no data" are one value.
func normalizeMonitorStatus(status string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(status)), " ", "_")
}
