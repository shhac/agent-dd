package mockdd

import (
	"maps"
	"slices"
	"sync"
)

// store holds the writable state of a single mock server.
//
// Anything a handler can write must live here rather than at package level.
// Handlers that close over package vars share state with every other server in
// the same test binary, so a write in one test leaks into the next — an
// order-dependent flake that makes red-green development worthless, because a
// failing test stops being evidence of anything. See
// TestMockddStateIsNotSharedBetweenServers.
//
// Read-only reference data (hosts, slos, services, logMessages) deliberately
// stays at package level: it is never written, so copying it per server would
// be noise without benefit.
type store struct {
	mu sync.Mutex

	monitors  []map[string]any
	incidents []map[string]any
	downtimes []map[string]any

	monitorCounter  int
	downtimeCounter int
}

// newStore seeds a server with its own copy of the writable fixtures.
// monitorCounter starts above the fixture IDs so a created monitor never
// collides with a seeded one.
func newStore() *store {
	return &store{
		monitors:       monitorFixtures(),
		incidents:      incidentFixtures(),
		downtimes:      make([]map[string]any, 0),
		monitorCounter: 9000,
	}
}

// The accessors below take the lock for the duration of the store access only,
// and return snapshots. Handlers must never hold the lock while calling each
// other — several dispatch onward (handleMonitors to handleMonitorSearch), and
// a lock held across that would deadlock.

func (s *store) allMonitors() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.monitors)
}

// monitorIndex locates a monitor by ID, or -1. Callers must already hold the
// lock — it exists so the three lookup-by-ID methods below share one definition
// of how a monitor is identified.
func (s *store) monitorIndex(id int) int {
	for i, m := range s.monitors {
		if mid, _ := m["id"].(int); mid == id {
			return i
		}
	}
	return -1
}

// findMonitor returns a copy. Handing out the stored map would let a caller
// mutate shared state outside the lock, which is the class of bug this whole
// store exists to rule out.
func (s *store) findMonitor(id int) (map[string]any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.monitorIndex(id)
	if i < 0 {
		return nil, false
	}
	return maps.Clone(s.monitors[i]), true
}

// Writes clone on the way in for the same reason reads clone on the way out:
// the store owns its state, and a caller that keeps hold of the map it passed
// must not be able to reach in and change it afterwards.
func (s *store) addMonitor(m map[string]any) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := maps.Clone(m)
	s.monitorCounter++
	stored["id"] = s.monitorCounter
	s.monitors = append(s.monitors, stored)
	return maps.Clone(stored)
}

// replaceMonitor swaps the stored monitor for `m`, preserving its position and
// its ID. Reports false when no monitor with that ID exists.
func (s *store) replaceMonitor(id int, m map[string]any) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.monitorIndex(id)
	if i < 0 {
		return false
	}
	stored := maps.Clone(m)
	stored["id"] = id
	s.monitors[i] = stored
	return true
}

func (s *store) removeMonitor(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.monitorIndex(id)
	if i < 0 {
		return false
	}
	s.monitors = append(s.monitors[:i], s.monitors[i+1:]...)
	return true
}

func (s *store) allIncidents() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.incidents)
}

func (s *store) allDowntimes() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.downtimes)
}

// addDowntime stamps the generated ID onto the caller's map, mirroring
// addMonitor so the store has one "add with generated ID" idiom rather than two.
func (s *store) addDowntime(dt map[string]any) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.downtimeCounter++
	dt["id"] = downtimeID(s.downtimeCounter)
	s.downtimes = append(s.downtimes, dt)
	return dt
}

func (s *store) removeDowntime(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, dt := range s.downtimes {
		if dtID, _ := dt["id"].(string); dtID == id {
			s.downtimes = append(s.downtimes[:i], s.downtimes[i+1:]...)
			return true
		}
	}
	return false
}
