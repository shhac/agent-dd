package monitors

import (
	"context"
	"maps"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/cli/shared"
	agenterrors "github.com/shhac/agent-dd/internal/errors"
	"github.com/shhac/agent-dd/internal/output"
)

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
				shared.WriteItem(forOutput(created), g.Format)
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
					"monitor":    forOutput(updated),
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

// forOutput renames Datadog's `overall_state` to `status` on a raw monitor map.
// create and update echo the API response verbatim to preserve fields this CLI
// does not model, which would otherwise make them the only commands naming the
// state differently from list, get and search.
func forOutput(monitor map[string]any) map[string]any {
	if monitor == nil {
		return nil
	}
	state, ok := monitor["overall_state"]
	if !ok {
		return monitor
	}
	out := maps.Clone(monitor)
	out["status"] = state
	delete(out, "overall_state")
	return out
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
//
// The substring match is the weak point: Datadog returns no machine-readable
// error code for this, so a reworded message would silently stop matching, and
// an unrelated 400 mentioning a "reference" would wrongly suggest --force.
// It only adds a hint — never changes control flow — so the blast radius of
// either failure is a misleading sentence, not a wrong action.
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
