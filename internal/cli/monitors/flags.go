package monitors

import (
	"encoding/json"
	"io"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/shared"
	agenterrors "github.com/shhac/agent-dd/internal/errors"
)

// monitorFlags is the field set shared by create and update. Typed flags cover
// the handful of options that actually come up after an incident; --body is the
// full-fidelity escape hatch for everything else, since an agent can't be
// expected to reconstruct Datadog's whole options schema from memory.
type monitorFlags struct {
	monitorType       string
	query             string
	name              string
	message           string
	tags              []string
	priority          int
	thresholdCritical float64
	thresholdWarning  float64
	notifyNoData      bool
	noDataTimeframe   int
	renotifyInterval  int
	evaluationDelay   int
	body              string
}

func (f *monitorFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.monitorType, "type", "", "Monitor type (e.g. \"metric alert\", \"log alert\", \"service check\")")
	cmd.Flags().StringVar(&f.query, "query", "", "Monitor query")
	cmd.Flags().StringVar(&f.name, "name", "", "Monitor name")
	cmd.Flags().StringVar(&f.message, "message", "", "Notification message (supports @handles)")
	cmd.Flags().StringArrayVar(&f.tags, "tag", nil, "Tag in key:value form (repeatable)")
	cmd.Flags().IntVar(&f.priority, "priority", 0, "Priority 1 (high) to 5 (low)")
	cmd.Flags().Float64Var(&f.thresholdCritical, "threshold-critical", 0, "Critical threshold")
	cmd.Flags().Float64Var(&f.thresholdWarning, "threshold-warning", 0, "Warning threshold")
	cmd.Flags().BoolVar(&f.notifyNoData, "notify-no-data", false, "Alert when the monitor reports no data")
	cmd.Flags().IntVar(&f.noDataTimeframe, "no-data-timeframe", 0, "Minutes without data before alerting")
	cmd.Flags().IntVar(&f.renotifyInterval, "renotify-interval", 0, "Minutes between re-notifications while alerting")
	cmd.Flags().IntVar(&f.evaluationDelay, "evaluation-delay", 0, "Seconds to delay evaluation")
	cmd.Flags().StringVar(&f.body, "body", "", "Full monitor JSON: inline, @file, or @- for stdin")
}

// fieldFlags maps each flag to where its value belongs in a monitor
// definition. A one-element path is a top-level field; a longer one descends,
// so --threshold-critical lands in options.thresholds.critical without any
// command needing to know that shape.
var fieldFlags = []struct {
	flag string
	path []string
	read func(*monitorFlags) any
}{
	{"type", []string{"type"}, func(f *monitorFlags) any { return f.monitorType }},
	{"query", []string{"query"}, func(f *monitorFlags) any { return f.query }},
	{"name", []string{"name"}, func(f *monitorFlags) any { return f.name }},
	{"message", []string{"message"}, func(f *monitorFlags) any { return f.message }},
	{"tag", []string{"tags"}, func(f *monitorFlags) any { return f.tags }},
	{"priority", []string{"priority"}, func(f *monitorFlags) any { return f.priority }},
	{"threshold-critical", []string{"options", "thresholds", "critical"}, func(f *monitorFlags) any { return f.thresholdCritical }},
	{"threshold-warning", []string{"options", "thresholds", "warning"}, func(f *monitorFlags) any { return f.thresholdWarning }},
	{"notify-no-data", []string{"options", "notify_no_data"}, func(f *monitorFlags) any { return f.notifyNoData }},
	{"no-data-timeframe", []string{"options", "no_data_timeframe"}, func(f *monitorFlags) any { return f.noDataTimeframe }},
	{"renotify-interval", []string{"options", "renotify_interval"}, func(f *monitorFlags) any { return f.renotifyInterval }},
	{"evaluation-delay", []string{"options", "evaluation_delay"}, func(f *monitorFlags) any { return f.evaluationDelay }},
}

// changes builds the definition fragment the caller asked for: --body if given,
// with explicitly-set flags layered on top. Only flags the user actually typed
// are included — an unset flag must not overwrite an existing value with a zero.
func (f *monitorFlags) changes(cmd *cobra.Command, stdin io.Reader) (map[string]any, error) {
	changes := map[string]any{}

	raw, err := shared.ResolveBody(f.body, stdin)
	if err != nil {
		return nil, err
	}
	if raw != nil {
		if err := json.Unmarshal(raw, &changes); err != nil {
			return nil, agenterrors.Wrap(err, agenterrors.FixableByAgent).
				WithHint("--body must be a JSON object describing a monitor")
		}
	}

	for _, field := range fieldFlags {
		if !cmd.Flags().Changed(field.flag) {
			continue
		}
		setNested(changes, field.path, field.read(f))
	}

	return changes, nil
}

// setNested writes value at a nested path, creating intermediate maps. It
// preserves anything already present alongside — so --body options and a
// --threshold-critical flag combine rather than one clobbering the other. A
// one-element path is just a top-level assignment.
func setNested(target map[string]any, path []string, value any) {
	for _, key := range path[:len(path)-1] {
		next, ok := target[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			target[key] = next
		}
		target = next
	}
	target[path[len(path)-1]] = value
}
