package metrics

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/cli/shared"
)

func Register(root *cobra.Command, globals func() *shared.GlobalFlags) {
	met := &cobra.Command{
		Use:   "metrics",
		Short: "Metric querying",
	}

	registerQuery(met, globals)
	registerList(met, globals)
	registerMetadata(met, globals)
	registerUsage(met)

	root.AddCommand(met)
}

func registerQuery(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var query, from, to string

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query metrics",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			if !shared.RequireFlag("query", query, "Example: --query 'avg:system.cpu.user{host:web-1}'") {
				return nil
			}

			fromTime, toTime, ok := shared.ParseTimeRangeOrWriteErr(from, to)
			if !ok {
				return nil
			}

			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				resp, err := client.QueryMetrics(ctx, query, fromTime.Unix(), toTime.Unix())
				if err != nil {
					return err
				}

				// Compact output uses Datadog's native field names so the
				// shape matches the v1 /query response and downstream
				// tooling familiar with it.
				compact := make([]map[string]any, len(resp.Series))
				for i, s := range resp.Series {
					row := map[string]any{
						"metric":    s.Metric,
						"scope":     s.Scope,
						"pointlist": s.Pointlist,
					}
					if len(s.TagSet) > 0 {
						row["tag_set"] = s.TagSet
					}
					compact[i] = row
				}
				shared.WritePaginatedList(shared.ToAnySlice(compact), nil, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Datadog metric query (required)")
	cmd.Flags().StringVar(&from, "from", "", "Start time (default: now-1h)")
	cmd.Flags().StringVar(&to, "to", "", "End time (default: now)")
	parent.AddCommand(cmd)
}

func registerList(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var search, tag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List/search metric names",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				resp, err := client.ListMetrics(ctx, search, tag)
				if err != nil {
					return err
				}
				shared.WritePaginatedList(shared.ToAnySlice(resp.Data), nil, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&search, "search", "", "Search text for metric names")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag")
	parent.AddCommand(cmd)
}

func registerMetadata(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "metadata <metric-name>",
		Short: "Get metric metadata (type, unit, description)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				meta, err := client.GetMetricMetadata(ctx, args[0])
				if err != nil {
					return err
				}
				shared.WriteItem(meta, g.Format)
				return nil
			})
		},
	}
	parent.AddCommand(cmd)
}
