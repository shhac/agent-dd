package slo

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/cli/shared"
)

func Register(root *cobra.Command, globals func() *shared.GlobalFlags) {
	s := &cobra.Command{
		Use:   "slo",
		Short: "SLO status and history",
	}

	registerList(s, globals)
	registerGet(s, globals)
	registerHistory(s, globals)
	registerUsage(s)

	root.AddCommand(s)
}

func registerList(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var search, tag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List SLOs",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				slos, err := client.ListSLOs(ctx, search, shared.SingleTag(tag))
				if err != nil {
					return err
				}
				compact := make([]map[string]any, len(slos))
				for i, s := range slos {
					compact[i] = map[string]any{
						"id":   s.ID,
						"name": s.Name,
						"type": s.Type,
					}
					if s.Status != nil {
						compact[i]["status"] = s.Status.Status
						compact[i]["error_budget_remaining"] = s.Status.ErrorBudgetRemaining
					}
				}
				shared.WritePaginatedList(shared.ToAnySlice(compact), nil, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&search, "search", "", "Search SLOs by name")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag")
	parent.AddCommand(cmd)
}

func registerGet(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get SLO details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				s, err := client.GetSLO(ctx, args[0])
				if err != nil {
					return err
				}
				shared.WriteItem(s, g.Format)
				return nil
			})
		},
	}
	parent.AddCommand(cmd)
}

func registerHistory(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var from, to string

	cmd := &cobra.Command{
		Use:   "history <id>",
		Short: "Get SLO history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()

			if !shared.RequireFlag("from", from, "Example: --from now-7d --to now") {
				return nil
			}

			fromTime, toTime, ok := shared.ParseTimeRangeOrWriteErr(from, to)
			if !ok {
				return nil
			}

			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				history, err := client.GetSLOHistory(ctx, args[0], fromTime.Unix(), toTime.Unix())
				if err != nil {
					return err
				}
				shared.WriteItem(history, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Start time (required)")
	cmd.Flags().StringVar(&to, "to", "", "End time (default: now)")
	parent.AddCommand(cmd)
}
