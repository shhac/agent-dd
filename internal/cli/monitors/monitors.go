package monitors

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/cli/shared"
	"github.com/shhac/agent-dd/internal/output"
)

func toCompactMonitors(monitors []api.Monitor) []api.MonitorCompact {
	compact := make([]api.MonitorCompact, len(monitors))
	for i, m := range monitors {
		compact[i] = api.MonitorCompact{
			ID:     m.ID,
			Name:   m.Name,
			Status: m.Status,
			Type:   m.Type,
		}
	}
	return compact
}

func Register(root *cobra.Command, globals func() *shared.GlobalFlags) {
	mon := &cobra.Command{
		Use:   "monitors",
		Short: "Monitor status and management",
	}

	registerList(mon, globals)
	registerGet(mon, globals)
	registerSearch(mon, globals)
	registerMute(mon, globals)
	registerUnmute(mon, globals)
	registerUsage(mon)

	root.AddCommand(mon)
}

func registerList(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var search, status, tag string
	var full bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List monitors",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				monitors, err := client.ListMonitors(ctx, search, shared.SingleTag(tag), status)
				if err != nil {
					return err
				}

				if full {
					shared.WritePaginatedList(shared.ToAnySlice(monitors), nil, g.Format)
					return nil
				}

				shared.WritePaginatedList(shared.ToAnySlice(toCompactMonitors(monitors)), nil, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&search, "search", "", "Filter by name")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (alert, warn, ok, no_data)")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag")
	cmd.Flags().BoolVar(&full, "full", false, "Show full monitor details")
	parent.AddCommand(cmd)
}

func registerGet(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get monitor details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			id, ok := shared.ParseIntArg("monitor ID", args[0])
			if !ok {
				return nil
			}
			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				monitor, err := client.GetMonitor(ctx, id)
				if err != nil {
					return err
				}
				shared.WriteItem(monitor, g.Format)
				return nil
			})
		},
	}
	parent.AddCommand(cmd)
}

func registerSearch(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var query, status string

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search monitors",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			if !shared.RequireFlag("query", query, "") {
				return nil
			}
			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				monitors, err := client.SearchMonitors(ctx, query, status)
				if err != nil {
					return err
				}
				shared.WritePaginatedList(shared.ToAnySlice(toCompactMonitors(monitors)), nil, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Search query (required)")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (alert, warn, ok, no_data)")
	parent.AddCommand(cmd)
}

func registerMute(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var end, reason string

	cmd := &cobra.Command{
		Use:   "mute <id>",
		Short: "Mute a monitor (creates a downtime)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			id, ok := shared.ParseIntArg("monitor ID", args[0])
			if !ok {
				return nil
			}

			var endEpoch int64
			if end != "" {
				t, err := shared.ParseTime(end)
				if err != nil {
					output.WriteError(os.Stderr, err)
					return nil
				}
				endEpoch = t.Unix()
			}

			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				downtime, err := client.CreateDowntime(ctx, id, endEpoch, reason)
				if err != nil {
					return err
				}
				shared.WriteItem(map[string]any{
					"status":      "muted",
					"monitor_id":  id,
					"downtime_id": downtime.ID,
				}, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&end, "end", "", "Mute end time (relative or absolute)")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for muting")
	parent.AddCommand(cmd)
}

func registerUnmute(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "unmute <id>",
		Short: "Unmute a monitor (cancels active downtimes)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			id, ok := shared.ParseIntArg("monitor ID", args[0])
			if !ok {
				return nil
			}
			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				downtimes, err := client.ListActiveDowntimes(ctx, id)
				if err != nil {
					return err
				}

				cancelled := make([]string, 0, len(downtimes))
				for _, dt := range downtimes {
					if err := client.CancelDowntime(ctx, dt.ID); err != nil {
						return err
					}
					cancelled = append(cancelled, dt.ID)
				}

				shared.WriteItem(map[string]any{
					"status":              "unmuted",
					"monitor_id":          id,
					"downtimes_cancelled": len(cancelled),
				}, g.Format)
				return nil
			})
		},
	}
	parent.AddCommand(cmd)
}
