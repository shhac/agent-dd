package mockdd_test

import (
	"context"
	"testing"

	"github.com/shhac/agent-dd/internal/mockdd/mockddtest"
)

// Mock state must be per-server, not per-package. Handlers that close over
// package-level vars leak writes between every server in the test binary,
// which turns any write-path test into an order-dependent flake — and the
// point of red-green is that red means something.

func TestMockddStateIsNotSharedBetweenServers(t *testing.T) {
	writer := mockddtest.NewTestClient(t)
	if _, err := writer.CreateDowntime(context.Background(), 1001, 0, "written by the first server"); err != nil {
		t.Fatalf("CreateDowntime: %v", err)
	}

	fresh := mockddtest.NewTestClient(t)
	downtimes, err := fresh.ListActiveDowntimes(context.Background(), 1001)
	if err != nil {
		t.Fatalf("ListActiveDowntimes: %v", err)
	}
	if len(downtimes) != 0 {
		t.Errorf("fresh server sees %d downtime(s) created against a different server — mock state is shared", len(downtimes))
	}
}

func TestMockddFixtureMutationDoesNotPersistAcrossServers(t *testing.T) {
	first := mockddtest.NewTestClient(t)
	before, err := first.ListMonitors(context.Background(), "", nil, "")
	if err != nil {
		t.Fatalf("ListMonitors: %v", err)
	}

	second := mockddtest.NewTestClient(t)
	after, err := second.ListMonitors(context.Background(), "", nil, "")
	if err != nil {
		t.Fatalf("ListMonitors: %v", err)
	}

	if len(before) != len(after) {
		t.Errorf("monitor fixture count drifted between servers: %d then %d", len(before), len(after))
	}
}
