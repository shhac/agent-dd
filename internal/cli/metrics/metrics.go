package metrics

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/cli/shared"
	agenterrors "github.com/shhac/agent-dd/internal/errors"
	"github.com/shhac/agent-dd/internal/output"
)

func Register(root *cobra.Command, globals func() *shared.GlobalFlags) {
	met := &cobra.Command{
		Use:   "metrics",
		Short: "Metric querying",
	}

	registerQuery(met, globals)
	registerList(met, globals)
	registerMetadata(met, globals)
	registerLLMHelp(met)

	root.AddCommand(met)
}

func registerQuery(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var query, from, to string

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query metrics",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			if query == "" {
				output.WriteError(os.Stderr, agenterrors.New("--query is required", agenterrors.FixableByAgent).
					WithHint("Example: --query 'avg:system.cpu.user{host:web-1}'"))
				return nil
			}

			fromTime, toTime, err := shared.ParseTimeRange(from, to)
			if err != nil {
				output.WriteError(os.Stderr, err)
				return nil
			}

			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				resp, err := client.QueryMetrics(ctx, query, fromTime.Unix(), toTime.Unix())
				if err != nil {
					return err
				}

				// Compact output: just metric, tags, points
				compact := make([]map[string]any, len(resp.Series))
				for i, s := range resp.Series {
					compact[i] = map[string]any{
						"metric": s.Metric,
						"tags":   s.Tags,
						"points": s.Points,
					}
				}
				output.PrintJSON(map[string]any{"series": compact}, true)
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
				names := make([]string, len(resp.Data))
				for i, m := range resp.Data {
					names[i] = m.ID
				}
				output.PrintJSON(map[string]any{"metrics": names}, true)
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
				output.PrintJSON(meta, true)
				return nil
			})
		},
	}
	parent.AddCommand(cmd)
}
