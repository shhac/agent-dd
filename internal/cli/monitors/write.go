package monitors

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/cli/shared"
	agenterrors "github.com/shhac/agent-dd/internal/errors"
	"github.com/shhac/agent-dd/internal/output"
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

// optionFlags maps a flag name to where its value belongs inside `options`.
// Nested paths let --threshold-critical land in options.thresholds.critical
// without each command knowing the shape.
var optionFlags = []struct {
	flag string
	path []string
	read func(*monitorFlags) any
}{
	{"threshold-critical", []string{"thresholds", "critical"}, func(f *monitorFlags) any { return f.thresholdCritical }},
	{"threshold-warning", []string{"thresholds", "warning"}, func(f *monitorFlags) any { return f.thresholdWarning }},
	{"notify-no-data", []string{"notify_no_data"}, func(f *monitorFlags) any { return f.notifyNoData }},
	{"no-data-timeframe", []string{"no_data_timeframe"}, func(f *monitorFlags) any { return f.noDataTimeframe }},
	{"renotify-interval", []string{"renotify_interval"}, func(f *monitorFlags) any { return f.renotifyInterval }},
	{"evaluation-delay", []string{"evaluation_delay"}, func(f *monitorFlags) any { return f.evaluationDelay }},
}

var topLevelFlags = []struct {
	flag string
	key  string
	read func(*monitorFlags) any
}{
	{"type", "type", func(f *monitorFlags) any { return f.monitorType }},
	{"query", "query", func(f *monitorFlags) any { return f.query }},
	{"name", "name", func(f *monitorFlags) any { return f.name }},
	{"message", "message", func(f *monitorFlags) any { return f.message }},
	{"tag", "tags", func(f *monitorFlags) any { return f.tags }},
	{"priority", "priority", func(f *monitorFlags) any { return f.priority }},
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

	for _, f2 := range topLevelFlags {
		if cmd.Flags().Changed(f2.flag) {
			changes[f2.key] = f2.read(f)
		}
	}

	for _, opt := range optionFlags {
		if !cmd.Flags().Changed(opt.flag) {
			continue
		}
		setNested(changes, append([]string{"options"}, opt.path...), opt.read(f))
	}

	return changes, nil
}

// setNested writes value at a nested path, creating intermediate maps. It
// preserves anything already present alongside — so --body options and a
// --threshold-critical flag combine rather than one clobbering the other.
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

func registerCreate(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	flags := &monitorFlags{}
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a monitor",
		Long: `Create a monitor.

--type and --query are required (either as flags or inside --body). Use
--dry-run to validate the definition against Datadog without creating
anything.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()

			definition, err := flags.changes(cmd, cmd.InOrStdin())
			if err != nil {
				output.WriteError(os.Stderr, err)
				return nil
			}
			if !requireDefinitionField(definition, "type") || !requireDefinitionField(definition, "query") {
				return nil
			}

			return shared.WithClient(g.Org, g.TimeoutMS, g.Debug, func(ctx context.Context, client *api.Client) error {
				if dryRun {
					return reportValidation(client.ValidateMonitor(ctx, definition), definition, g.Format)
				}

				created, err := client.CreateMonitor(ctx, definition)
				if err != nil {
					return err
				}
				shared.WriteItem(created, g.Format)
				return nil
			})
		},
	}
	flags.bind(cmd)
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate the definition without creating it")
	parent.AddCommand(cmd)
}

func registerUpdate(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	flags := &monitorFlags{}
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a monitor",
		Long: `Update a monitor, changing only the fields you pass.

The monitor is read, your changes are layered on, and the whole definition is
written back — so fields this CLI does not model (restricted_roles, composite
sub-monitors, the long tail of options) survive untouched. Options merge
key-by-key, so raising a critical threshold will not clear the warning one.

The response reports a before/after diff of exactly what moved.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()

			id, ok := shared.ParseIntArg("monitor ID", args[0])
			if !ok {
				return nil
			}

			changes, err := flags.changes(cmd, cmd.InOrStdin())
			if err != nil {
				output.WriteError(os.Stderr, err)
				return nil
			}
			if len(changes) == 0 {
				output.WriteError(os.Stderr, agenterrors.New(
					"no changes given — pass at least one field flag or --body",
					agenterrors.FixableByAgent).
					WithHint("Run 'agent-dd monitors usage' to see the available field flags"))
				return nil
			}

			return shared.WithClient(g.Org, g.TimeoutMS, g.Debug, func(ctx context.Context, client *api.Client) error {
				prepared, err := client.PrepareMonitorUpdate(ctx, id, changes)
				if err != nil {
					return err
				}

				if dryRun {
					err := client.ValidateExistingMonitor(ctx, id, prepared.Definition)
					if err != nil {
						return err
					}
					shared.WriteItem(map[string]any{
						"status":     "valid",
						"dry_run":    true,
						"monitor_id": id,
						"changes":    prepared.Changes,
					}, g.Format)
					return nil
				}

				updated, err := client.UpdateMonitor(ctx, id, prepared.Definition)
				if err != nil {
					return err
				}
				shared.WriteItem(map[string]any{
					"status":     "updated",
					"monitor_id": id,
					"changes":    prepared.Changes,
					"monitor":    updated,
				}, g.Format)
				return nil
			})
		},
	}
	flags.bind(cmd)
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate the merged definition without writing it")
	parent.AddCommand(cmd)
}

func registerDelete(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var confirm, force bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a monitor (requires --yes)",
		Long: `Delete a monitor.

Requires --yes. Unlike create and update, a delete leaves nothing behind to
read or correct, so it cannot be reached by a half-constructed command line.

Datadog refuses to delete a monitor referenced by an SLO or composite monitor;
--force overrides that, but the refusal is real signal and worth reading first.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()

			id, ok := shared.ParseIntArg("monitor ID", args[0])
			if !ok {
				return nil
			}
			if !confirm {
				output.WriteError(os.Stderr, agenterrors.New(
					"delete requires --yes", agenterrors.FixableByAgent).
					WithHint("Re-run with --yes to confirm deleting this monitor"))
				return nil
			}

			return shared.WithClient(g.Org, g.TimeoutMS, g.Debug, func(ctx context.Context, client *api.Client) error {
				if err := client.DeleteMonitor(ctx, id, force); err != nil {
					return annotateReferencedMonitorError(err, force)
				}
				shared.WriteItem(map[string]any{
					"status":     "deleted",
					"monitor_id": id,
				}, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&confirm, "yes", false, "Confirm the deletion (required)")
	cmd.Flags().BoolVar(&force, "force", false, "Delete even when referenced by an SLO or composite monitor")
	parent.AddCommand(cmd)
}

// requireDefinitionField reports a missing required field the same way
// shared.RequireFlag does, but reads from the assembled definition so a value
// supplied via --body counts as present.
func requireDefinitionField(definition map[string]any, field string) bool {
	if value, ok := definition[field].(string); ok && value != "" {
		return true
	}
	output.WriteError(os.Stderr, agenterrors.Newf(agenterrors.FixableByAgent,
		"--%s is required", field).
		WithHint("Pass --"+field+", or include it in --body"))
	return false
}

func reportValidation(err error, definition map[string]any, format string) error {
	if err != nil {
		return err
	}
	shared.WriteItem(map[string]any{
		"status":     "valid",
		"dry_run":    true,
		"definition": definition,
	}, format)
	return nil
}

// annotateReferencedMonitorError turns Datadog's referential refusal into
// something an agent can act on. Without the hint it reads as a generic 400 and
// the next step isn't obvious.
func annotateReferencedMonitorError(err error, force bool) error {
	if force {
		return err
	}
	var aerr *agenterrors.APIError
	if !agenterrors.As(err, &aerr) || !strings.Contains(strings.ToLower(aerr.Error()), "referenc") {
		return err
	}
	return aerr.WithHint("This monitor is referenced by an SLO or composite monitor — re-run with --force to delete it anyway")
}
