package mockdd_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/shhac/agent-dd/internal/mockdd"
	"github.com/shhac/agent-dd/internal/mockdd/mockddtest"
)

// Every route must reject the verbs it doesn't implement. A handler that
// answers any method identically will happily return 200 to a write the mock
// never implemented, so a write-path test would pass against a mock that does
// nothing — the exact false green this package exists to avoid.

func TestEveryRouteRejectsDisallowedMethods(t *testing.T) {
	srv := httptest.NewServer(mockdd.NewHandler())
	t.Cleanup(srv.Close)

	cases := []struct {
		path       string
		disallowed string
	}{
		{"/api/v1/validate", http.MethodPost},
		{"/api/v1/monitor", http.MethodPatch},
		{"/api/v1/monitor/1001", http.MethodPatch},
		{"/api/v2/downtime", http.MethodPut},
		{"/api/v2/downtime/dt-000001", http.MethodGet},
		{"/api/v2/logs/events/search", http.MethodGet},
		{"/api/v2/logs/analytics/aggregate", http.MethodGet},
		{"/api/v1/query", http.MethodPost},
		{"/api/v2/metrics", http.MethodPost},
		{"/api/v1/metrics/system.cpu.user", http.MethodDelete},
		{"/api/v1/events", http.MethodPost},
		{"/api/v1/events/5001", http.MethodDelete},
		{"/api/v1/hosts", http.MethodPost},
		{"/api/v1/host/example/mute", http.MethodGet},
		{"/api/v2/spans/events/search", http.MethodGet},
		{"/api/v2/apm/services", http.MethodPost},
		{"/api/v2/incidents", http.MethodDelete},
		{"/api/v2/incidents/inc-a1b2c3d4", http.MethodDelete},
		{"/api/v1/slo", http.MethodPost},
		{"/api/v1/slo/slo-aaa111", http.MethodDelete},
	}

	for _, tc := range cases {
		t.Run(tc.disallowed+" "+tc.path, func(t *testing.T) {
			req, err := http.NewRequest(tc.disallowed, srv.URL+tc.path, nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			req.Header.Set("DD-API-KEY", "test-api-key")
			req.Header.Set("DD-APPLICATION-KEY", "test-app-key")

			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want 405", resp.StatusCode)
			}
			if allow := resp.Header.Get("Allow"); allow == "" {
				t.Error("405 response is missing the Allow header")
			} else if strings.Contains(allow, tc.disallowed) {
				t.Errorf("Allow = %q, must not advertise the rejected method %s", allow, tc.disallowed)
			}
		})
	}
}

// The store is reached concurrently — httptest serves each request on its own
// goroutine. This test is only meaningful under -race, which is why `make test`
// passes it.
func TestStoreSurvivesConcurrentAccess(t *testing.T) {
	client := mockddtest.NewTestClient(t)

	var wg sync.WaitGroup
	for i := range 25 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx := t.Context()
			if _, err := client.CreateDowntime(ctx, 1001+i%5, 0, "concurrent"); err != nil {
				t.Errorf("CreateDowntime: %v", err)
			}
			if _, err := client.ListMonitors(ctx, "", nil, ""); err != nil {
				t.Errorf("ListMonitors: %v", err)
			}
			if _, err := client.ListActiveDowntimes(ctx, 1001); err != nil {
				t.Errorf("ListActiveDowntimes: %v", err)
			}
			if _, err := client.GetMonitor(ctx, 1001); err != nil {
				t.Errorf("GetMonitor: %v", err)
			}
		}(i)
	}
	wg.Wait()
}
