package mockdd

import (
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Search never reaches here: the mux registers "/api/v1/monitor" as an exact
// pattern, so /api/v1/monitor/search resolves to the "/api/v1/monitor/" subtree
// and lands in handleMonitorByID, which dispatches it.
func (s *server) handleMonitors(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, s.allMonitors())
	case http.MethodPost:
		s.createMonitor(w, r)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *server) createMonitor(w http.ResponseWriter, r *http.Request) {
	definition, ok := decodeValidMonitor(w, r)
	if !ok {
		return
	}

	definition["overall_state"] = "No Data"
	definition["created"] = nowRFC3339()
	definition["modified"] = nowRFC3339()
	writeJSON(w, 200, s.addMonitor(definition))
}

func (s *server) handleMonitorSearch(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	query := r.URL.Query().Get("query")
	results := make([]map[string]any, 0)
	statusBucket := map[string]int{}
	// Datadog names the muted buckets with bools, not strings — keyed here the
	// same way so the client's decoding of a mixed-type envelope stays covered.
	mutedBucket := map[bool]int{}
	for _, m := range s.allMonitors() {
		name, _ := m["name"].(string)
		// `*` is Datadog's match-all sentinel; treat it as such instead of
		// a literal substring (which matches nothing).
		if query == "" || query == "*" || strings.Contains(strings.ToLower(name), strings.ToLower(query)) {
			results = append(results, toSearchShape(m))
			if state, _ := m["overall_state"].(string); state != "" {
				statusBucket[state]++
			}
			muted, _ := m["muted"].(bool)
			mutedBucket[muted]++
		}
	}

	writeJSON(w, 200, map[string]any{
		"monitors": results,
		"counts": map[string]any{
			"status": bucketCounts(statusBucket),
			"muted":  bucketCounts(mutedBucket),
		},
		"metadata": map[string]any{
			"total":         len(results),
			"page":          0,
			"per_page":      30,
			"page_count":    1,
			"total_results": len(results),
		},
	})
}

// toSearchShape projects a stored monitor into the shape /v1/monitor/search
// returns, which is not the shape /v1/monitor returns for the same entity:
// state is named `status` rather than `overall_state`, and timestamps are epoch
// seconds rather than RFC3339. Serving list-shaped objects from search once hid
// a decode failure that made `monitors search` unusable against a real org.
func toSearchShape(m map[string]any) map[string]any {
	out := maps.Clone(m)

	if state, ok := out["overall_state"]; ok {
		out["status"] = state
		delete(out, "overall_state")
	}
	for _, field := range []string{"created", "modified"} {
		if ts, ok := out[field].(string); ok {
			out[field] = rfc3339ToEpoch(ts)
		}
	}
	// Search results carry a classification instead of the full options blob.
	delete(out, "options")
	out["classification"] = "metric"

	return out
}

func rfc3339ToEpoch(ts string) int64 {
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return 0
	}
	return parsed.Unix()
}

// bucketCounts converts a tally map into Datadog's [{name, count}] envelope
// shape used inside `counts.{status,muted,tag,type,...}` on the monitor
// search response.
//
// Generic over the key type because Datadog is: the status/type/tag buckets
// name themselves with strings while muted names itself with a bool, and
// emitting a bool as the string "true" here once hid a client-side decode
// failure that made `monitors search` unusable against a real org.
func bucketCounts[K comparable](b map[K]int) []map[string]any {
	out := make([]map[string]any, 0, len(b))
	for name, count := range b {
		out = append(out, map[string]any{"name": name, "count": count})
	}
	return out
}

func (s *server) handleMonitorByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/monitor/")
	if path == "search" {
		s.handleMonitorSearch(w, r)
		return
	}
	if path == "validate" {
		s.validateNewMonitor(w, r)
		return
	}

	parts := strings.Split(path, "/")
	if len(parts) > 1 && parts[1] == "validate" {
		s.validateExistingMonitor(w, r, parts[0])
		return
	}

	id, err := strconv.Atoi(parts[0])
	if err != nil {
		writeError(w, 400, "invalid monitor ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getMonitor(w, id)
	case http.MethodPut:
		s.updateMonitor(w, r, id)
	case http.MethodDelete:
		s.deleteMonitor(w, r, id)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
	}
}

func (s *server) getMonitor(w http.ResponseWriter, id int) {
	m, ok := s.findMonitor(id)
	if !ok {
		writeError(w, 404, "Monitor not found")
		return
	}
	writeJSON(w, 200, m)
}

func (s *server) updateMonitor(w http.ResponseWriter, r *http.Request, id int) {
	definition, ok := decodeValidMonitor(w, r)
	if !ok {
		return
	}

	definition["modified"] = nowRFC3339()
	if !s.replaceMonitor(id, definition) {
		writeError(w, 404, "Monitor not found")
		return
	}

	updated, _ := s.findMonitor(id)
	writeJSON(w, 200, updated)
}

func (s *server) deleteMonitor(w http.ResponseWriter, r *http.Request, id int) {
	// Datadog refuses to delete a monitor referenced by an SLO or composite
	// monitor unless force is set. Modelling the refusal is the point — a mock
	// where every delete succeeds would never exercise the --force path.
	if monitorIsReferenced(id) && r.URL.Query().Get("force") == "" {
		writeError(w, 400, "Monitor is referenced in an SLO and cannot be deleted; retry with force")
		return
	}
	if !s.removeMonitor(id) {
		writeError(w, 404, "Monitor not found")
		return
	}
	writeJSON(w, 200, map[string]any{"deleted_monitor_id": id})
}

func (s *server) validateNewMonitor(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, ok := decodeValidMonitor(w, r); !ok {
		return
	}
	writeJSON(w, 200, map[string]any{})
}

func (s *server) validateExistingMonitor(w http.ResponseWriter, r *http.Request, idStr string) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, 400, "invalid monitor ID")
		return
	}
	if _, found := s.findMonitor(id); !found {
		writeError(w, 404, "Monitor not found")
		return
	}
	if _, ok := decodeValidMonitor(w, r); !ok {
		return
	}
	writeJSON(w, 200, map[string]any{})
}

// decodeValidMonitor is the single entry point every monitor write and validate
// handler goes through: decode the body, apply Datadog's required-field checks,
// and report false with the error already written when either fails.
func decodeValidMonitor(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	var definition map[string]any
	if err := json.NewDecoder(r.Body).Decode(&definition); err != nil || definition == nil {
		writeError(w, 400, "request body is not a JSON object")
		return nil, false
	}
	if !validMonitorDefinition(w, definition) {
		return nil, false
	}
	return definition, true
}

// validMonitorDefinition mirrors the checks Datadog's own validate endpoint
// applies: type and query are required, and the query has to look like one.
// The query check is deliberately crude — enough to make a malformed query fail
// in tests, not an attempt to reimplement Datadog's parser.
func validMonitorDefinition(w http.ResponseWriter, definition map[string]any) bool {
	monitorType, _ := definition["type"].(string)
	if monitorType == "" {
		writeError(w, 400, "The value provided for parameter 'type' is invalid")
		return false
	}

	query, _ := definition["query"].(string)
	if query == "" {
		writeError(w, 400, "The value provided for parameter 'query' is invalid")
		return false
	}
	if !strings.ContainsAny(query, "(){}\"") {
		writeError(w, 400, "The value provided for parameter 'query' is invalid: unable to parse the query")
		return false
	}
	return true
}

// monitorIsReferenced reports whether a fixture monitor is pointed at by an SLO
// in the fixture set, so delete can model Datadog's referential refusal.
func monitorIsReferenced(id int) bool {
	return slices.Contains(referencedMonitorIDs, id)
}
