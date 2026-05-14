package mockdd_test

import (
	"context"
	"strings"
	"testing"

	"github.com/shhac/agent-dd/internal/mockdd/mockddtest"
)

// Each test drives a real api.Client against the canonical mockdd handler.
// The point is to catch drift between the fixture shapes mockdd emits and
// the structs the API client expects to decode — the kind of bug the
// traces error-object regression hid for months.

func TestMockddMonitorsListDecodesMutedPriorityLastTriggered(t *testing.T) {
	c := mockddtest.NewTestClient(t)

	monitors, err := c.ListMonitors(context.Background(), "", nil, "")
	if err != nil {
		t.Fatalf("ListMonitors: %v", err)
	}
	if len(monitors) == 0 {
		t.Fatal("expected monitors, got 0")
	}
	var sawPriority, sawMuted, sawLastTriggered bool
	for _, m := range monitors {
		if m.Priority > 0 {
			sawPriority = true
		}
		if m.Muted {
			sawMuted = true
		}
		if m.LastTriggeredTs > 0 {
			sawLastTriggered = true
		}
	}
	if !sawPriority {
		t.Error("no monitor had priority — fixture / decode mismatch")
	}
	if !sawMuted {
		t.Error("no monitor was muted — fixture / decode mismatch")
	}
	if !sawLastTriggered {
		t.Error("no monitor had last_triggered_ts — fixture / decode mismatch")
	}
}

func TestMockddIncidentsListResolvesCommanderViaIncluded(t *testing.T) {
	c := mockddtest.NewTestClient(t)

	resp, err := c.ListIncidents(context.Background(), "")
	if err != nil {
		t.Fatalf("ListIncidents: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected incidents, got 0")
	}
	if len(resp.Included) == 0 {
		t.Fatal("expected included docs when include=commander_user")
	}
	for i, inc := range resp.Data {
		if inc.Attributes.State == "" {
			t.Errorf("incident %d missing state — fixture still using legacy `status`?", i)
		}
		if h := resp.CommanderHandle(i); h == "" {
			t.Errorf("incident %d missing commander handle resolution", i)
		}
	}
}

func TestMockddIncidentsStateFilter(t *testing.T) {
	c := mockddtest.NewTestClient(t)

	resp, err := c.ListIncidents(context.Background(), "active")
	if err != nil {
		t.Fatalf("ListIncidents(active): %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected at least one active incident")
	}
	for _, inc := range resp.Data {
		if inc.Attributes.State != "active" {
			t.Errorf("filter[state]=active returned state=%q", inc.Attributes.State)
		}
	}
}

func TestMockddIncidentsGetByID(t *testing.T) {
	c := mockddtest.NewTestClient(t)

	doc, err := c.GetIncident(context.Background(), "inc-a1b2c3d4")
	if err != nil {
		t.Fatalf("GetIncident: %v", err)
	}
	if doc.Data.Attributes.State == "" {
		t.Error("GetIncident returned empty state — fixture wrong shape?")
	}
	if doc.CommanderHandle() == "" {
		t.Error("GetIncident did not resolve commander via included")
	}
}

func TestMockddEventsListDecodesSourceTypeName(t *testing.T) {
	c := mockddtest.NewTestClient(t)

	events, err := c.ListEvents(context.Background(), 0, 1, "", nil)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected events, got 0")
	}
	for _, e := range events {
		if e.SourceTypeName == "" {
			t.Errorf("event %d missing source_type_name (fixture still using `source`?)", e.ID)
		}
		if e.IDStr == "" {
			t.Errorf("event %d missing id_str", e.ID)
		}
	}
}

func TestMockddMetricsQueryHappyPath(t *testing.T) {
	c := mockddtest.NewTestClient(t)

	resp, err := c.QueryMetrics(context.Background(), "avg:system.cpu.user{*}", 0, 1)
	if err != nil {
		t.Fatalf("QueryMetrics: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("Status = %q, want ok", resp.Status)
	}
	if len(resp.Series) == 0 {
		t.Fatal("expected series, got 0")
	}
	s := resp.Series[0]
	if len(s.Pointlist) == 0 {
		t.Errorf("series missing pointlist")
	}
	if s.Length == 0 {
		t.Errorf("series missing length")
	}
	if s.Interval == 0 {
		t.Errorf("series missing interval")
	}
}

func TestMockddMetricsQuerySurfacesError(t *testing.T) {
	c := mockddtest.NewTestClient(t)

	_, err := c.QueryMetrics(context.Background(), "fail-query", 0, 1)
	if err == nil {
		t.Fatal("expected error for fail-query sentinel, got nil")
	}
	if !strings.Contains(err.Error(), "query parse error") {
		t.Errorf("expected error to mention parse error, got %q", err.Error())
	}
}

func TestMockddMetricsMetadataSetsNameFromArg(t *testing.T) {
	c := mockddtest.NewTestClient(t)

	meta, err := c.GetMetricMetadata(context.Background(), "system.cpu.user")
	if err != nil {
		t.Fatalf("GetMetricMetadata: %v", err)
	}
	if meta.Name != "system.cpu.user" {
		t.Errorf("Name = %q, want system.cpu.user (set from request arg)", meta.Name)
	}
	if meta.StatsdInterval == 0 {
		t.Error("StatsdInterval not decoded — fixture missing statsd_interval?")
	}
}

func TestMockddHostsListCombinesFilter(t *testing.T) {
	c := mockddtest.NewTestClient(t)

	resp, err := c.ListHosts(context.Background(), "checkout", []string{"team:platform"})
	if err != nil {
		t.Fatalf("ListHosts: %v", err)
	}
	// The mockdd handler uses a generic substring filter; we just verify
	// the request decodes through and the host list isn't malformed.
	if resp == nil {
		t.Fatal("nil response")
	}
}

func TestMockddSLOHistoryDecodesAllFields(t *testing.T) {
	c := mockddtest.NewTestClient(t)

	history, err := c.GetSLOHistory(context.Background(), "slo-aaa111", 0, 1)
	if err != nil {
		t.Fatalf("GetSLOHistory: %v", err)
	}
	if history.Overall == nil {
		t.Fatal("expected Overall to be non-nil")
	}
	if history.Overall.SLIValue == 0 {
		t.Error("Overall.SLIValue zero — sli_value not decoding from fixture?")
	}
	if history.Type == "" {
		t.Error("history.type not decoding")
	}
	if len(history.Thresholds) == 0 {
		t.Fatal("expected thresholds map, got empty")
	}
	for tf, th := range history.Thresholds {
		if th.Timeframe != tf {
			t.Errorf("threshold[%q].Timeframe = %q, want %q", tf, th.Timeframe, tf)
		}
		if th.Target == 0 {
			t.Errorf("threshold[%q].Target is zero", tf)
		}
	}
}

func TestMockddDowntimesListByMonitorID(t *testing.T) {
	c := mockddtest.NewTestClient(t)

	// CreateDowntime first so the list isn't empty.
	if _, err := c.CreateDowntime(context.Background(), 1001, 1700000000, "test"); err != nil {
		t.Fatalf("CreateDowntime: %v", err)
	}
	// Mockdd's list returns a synthetic single downtime per monitor query;
	// the test just ensures the response decodes cleanly.
	if _, err := c.ListActiveDowntimes(context.Background(), 1001); err != nil {
		t.Fatalf("ListActiveDowntimes: %v", err)
	}
}

func TestMockddLogsSearchDecodesEnvelope(t *testing.T) {
	c := mockddtest.NewTestClient(t)

	resp, err := c.SearchLogs(context.Background(), "*", "now-1h", "now", "", 10, "", "")
	if err != nil {
		t.Fatalf("SearchLogs: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected logs, got 0")
	}
	for _, d := range resp.Data {
		if d.ID == "" {
			t.Errorf("log entry missing id")
		}
		if d.Attributes.Timestamp == "" {
			t.Errorf("log entry missing timestamp")
		}
	}
}

func TestMockddMonitorSearchEnvelopeWithCounts(t *testing.T) {
	c := mockddtest.NewTestClient(t)

	resp, err := c.SearchMonitors(context.Background(), "*", "")
	if err != nil {
		t.Fatalf("SearchMonitors: %v", err)
	}
	if len(resp.Monitors) == 0 {
		t.Fatal("expected monitors, got 0")
	}
	if resp.Counts == nil {
		t.Fatal("expected Counts to be decoded from mockdd response")
	}
	if len(resp.Counts.Status) == 0 {
		t.Error("expected status buckets in Counts")
	}
	if len(resp.Counts.Muted) == 0 {
		t.Error("expected muted buckets in Counts")
	}
	if resp.Metadata == nil || resp.Metadata.Total == 0 {
		t.Error("expected metadata.total > 0")
	}
}
