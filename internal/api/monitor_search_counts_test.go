package api

import (
	"encoding/json"
	"testing"
)

// Datadog's monitor-search `counts` envelope is not uniformly typed: the
// `status`, `type` and `tag` buckets name themselves with strings, but `muted`
// names itself with a bool. Decoding every bucket into a string field made
// `monitors search` fail outright against any real org:
//
//	json: cannot unmarshal bool into Go struct field
//	MonitorSearchBucket.counts.muted.name of type string
//
// The mock emitted "true"/"false" as strings, so the whole suite passed.
//
// This is a captured shape from a real response, trimmed of identifying data.
const realSearchCountsEnvelope = `{
  "monitors": [],
  "counts": {
    "status": [{"count": 11, "name": "No Data"}, {"count": 5, "name": "OK"}],
    "muted":  [{"count": 16, "name": false}],
    "type":   [{"count": 13, "name": "metric"}, {"count": 2, "name": "integration"}],
    "tag":    [{"count": 15, "name": "module:example"}]
  },
  "metadata": {"total_results": 16, "page": 0, "per_page": 30, "page_count": 1}
}`

func TestMonitorSearchDecodesRealCountsEnvelope(t *testing.T) {
	var resp MonitorSearchResponse
	if err := json.Unmarshal([]byte(realSearchCountsEnvelope), &resp); err != nil {
		t.Fatalf("decoding a real search envelope failed: %v", err)
	}

	if resp.Counts == nil {
		t.Fatal("counts were dropped")
	}

	if len(resp.Counts.Status) != 2 || resp.Counts.Status[0].Name != "No Data" {
		t.Errorf("status buckets = %+v, want the first named \"No Data\"", resp.Counts.Status)
	}

	// The bool bucket is the whole point.
	if len(resp.Counts.Muted) != 1 {
		t.Fatalf("muted buckets = %+v, want 1", resp.Counts.Muted)
	}
	if resp.Counts.Muted[0].Name != "false" {
		t.Errorf("muted bucket name = %q, want \"false\" — a bool name must normalise to a string", resp.Counts.Muted[0].Name)
	}
	if resp.Counts.Muted[0].Count != 16 {
		t.Errorf("muted bucket count = %d, want 16", resp.Counts.Muted[0].Count)
	}

	if len(resp.Counts.Type) != 2 || resp.Counts.Type[0].Name != "metric" {
		t.Errorf("type buckets = %+v", resp.Counts.Type)
	}
}

func TestMonitorSearchBucketAcceptsEveryNameShape(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`{"name": "Alert", "count": 3}`, "Alert"},
		{`{"name": true, "count": 1}`, "true"},
		{`{"name": false, "count": 2}`, "false"},
		{`{"name": 5, "count": 1}`, "5"},
		{`{"name": null, "count": 1}`, ""},
	}

	for _, tc := range cases {
		var bucket MonitorSearchBucket
		if err := json.Unmarshal([]byte(tc.raw), &bucket); err != nil {
			t.Errorf("%s: %v", tc.raw, err)
			continue
		}
		if bucket.Name != tc.want {
			t.Errorf("%s -> name %q, want %q", tc.raw, bucket.Name, tc.want)
		}
	}
}
