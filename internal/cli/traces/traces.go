package traces

import (
	"context"
	"os"
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
						"service":  d.Attributes.Service,
						"name":     d.Attributes.Name,
						"resource": d.Attributes.Resource,
						"duration": d.Attributes.Duration,
						"status":   d.Attributes.Status,
						"error":    d.Attributes.Error,
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
