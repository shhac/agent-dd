package traces

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/cli/shared"
	agenterrors "github.com/shhac/agent-dd/internal/errors"
	"github.com/shhac/agent-dd/internal/output"
)

func Register(root *cobra.Command, globals func() *shared.GlobalFlags) {
	tr := &cobra.Command{
		Use:   "traces",
		Short: "APM trace search",
	}

	registerSearch(tr, globals)
	registerServices(tr, globals)
	registerPercentile(tr, globals)
	registerUsage(tr)

	root.AddCommand(tr)
}

func registerSearch(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var query, service, from, to string
	var limit int

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search traces/spans",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			if query == "" && service == "" {
				output.WriteError(os.Stderr, agenterrors.New("--query or --service is required", agenterrors.FixableByAgent))
				return nil
			}

			fromTime, toTime, ok := shared.ParseTimeRangeOrWriteErr(from, to)
			if !ok {
				return nil
			}

			if limit == 0 {
				limit = 50
			}

			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				resp, err := client.SearchTraces(ctx, query, service,
					fromTime.Format(time.RFC3339),
					toTime.Format(time.RFC3339),
					limit)
				if err != nil {
					return err
				}

				spans := make([]map[string]any, len(resp.Data))
				for i, d := range resp.Data {
					spans[i] = map[string]any{
						"service":   d.Attributes.Service,
						"operation": d.Attributes.OperationName,
						"resource":  d.Attributes.ResourceName,
						"env":       d.Attributes.Env,
						"status":    d.Attributes.Status,
						"start":     d.Attributes.StartTimestamp,
						"end":       d.Attributes.EndTimestamp,
					}
					if len(d.Attributes.Tags) > 0 {
						spans[i]["tags"] = d.Attributes.Tags
					}
					if d.Attributes.Error != nil {
						spans[i]["error"] = d.Attributes.Error
					}
				}
				shared.WritePaginatedList(shared.ToAnySlice(spans), shared.CursorPagination(resp.Cursor()), g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Trace search query")
	cmd.Flags().StringVar(&service, "service", "", "Filter by service")
	cmd.Flags().StringVar(&from, "from", "", "Start time (default: now-1h)")
	cmd.Flags().StringVar(&to, "to", "", "End time (default: now)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	parent.AddCommand(cmd)
}

func registerPercentile(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var query, service, resource, from, to, percentile string

	cmd := &cobra.Command{
		Use:   "percentile",
		Short: "Compute span duration percentiles via aggregate API",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()

			// Build filter query from flags.
			parts := []string{}
			if service != "" {
				parts = append(parts, "service:"+service)
			}
			if resource != "" {
				parts = append(parts, "resource_name:\""+resource+"\"")
			}
			if query != "" {
				parts = append(parts, query)
			}
			filterQuery := "*"
			if len(parts) > 0 {
				filterQuery = strings.Join(parts, " ")
			}

			agg := percentile
			if agg == "" {
				agg = "pc95"
			}

			fromTime, toTime, ok := shared.ParseTimeRangeOrWriteErr(from, to)
			if !ok {
				return nil
			}

			groupBy := []string{"service", "resource_name"}

			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				buckets, err := client.AggregateSpans(
					ctx,
					filterQuery,
					fromTime.Format(time.RFC3339),
					toTime.Format(time.RFC3339),
					agg,
					"@duration",
					groupBy,
				)
				if err != nil {
					return err
				}

				rows := make([]map[string]any, 0, len(buckets))
				for _, b := range buckets {
					// @duration is in nanoseconds — convert to ms.
					for _, v := range b.Compute {
						row := map[string]any{
							"service":       b.By["service"],
							"resource_name": b.By["resource_name"],
							"percentile":    agg,
							"value_ms":      v / 1e6,
						}
						rows = append(rows, row)
					}
				}
				shared.WritePaginatedList(shared.ToAnySlice(rows), nil, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Additional span search filter")
	cmd.Flags().StringVar(&service, "service", "", "Filter by service")
	cmd.Flags().StringVar(&resource, "resource", "", "Filter by resource name")
	cmd.Flags().StringVar(&from, "from", "", "Start time (default: now-1h)")
	cmd.Flags().StringVar(&to, "to", "", "End time (default: now)")
	cmd.Flags().StringVar(&percentile, "percentile", "pc95", "Aggregation: pc75, pc90, pc95, pc98, pc99")
	parent.AddCommand(cmd)
}

func registerServices(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var search, env string

	cmd := &cobra.Command{
		Use:   "services",
		Short: "List APM services",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				services, err := client.ListServices(ctx, env, search)
				if err != nil {
					return err
				}
				shared.WritePaginatedList(shared.ToAnySlice(services), nil, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&search, "search", "", "Filter service names")
	cmd.Flags().StringVar(&env, "env", "", "Filter by environment (default: all)")
	parent.AddCommand(cmd)
}
