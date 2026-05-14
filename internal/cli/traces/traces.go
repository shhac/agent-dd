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

// compactSpanSkippedFields lists the trace attributes that `traces search`
// drops from per-row output by default. Surfacing this list via a `@skipped`
// meta-line lets callers see what's available behind `--full` without
// guessing — the data is in the search response, not behind a follow-up API.
var compactSpanSkippedFields = []string{
	"trace_id", "span_id", "parent_id", "host", "type",
	"resource_hash", "single_span", "attributes", "custom",
}

func registerSearch(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var query, service, from, to, cursor string
	var limit int
	var full bool

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
					limit, cursor)
				if err != nil {
					return err
				}

				spans := make([]map[string]any, len(resp.Data))
				for i, d := range resp.Data {
					row := map[string]any{
						"service":   d.Attributes.Service,
						"operation": d.Attributes.OperationName,
						"resource":  d.Attributes.ResourceName,
						"env":       d.Attributes.Env,
						"status":    d.Attributes.Status,
						"start":     d.Attributes.StartTimestamp,
						"end":       d.Attributes.EndTimestamp,
					}
					if len(d.Attributes.Tags) > 0 {
						row["tags"] = d.Attributes.Tags
					}
					if d.Attributes.Error != nil {
						row["error"] = d.Attributes.Error
					}
					if full {
						row["trace_id"] = d.Attributes.TraceID
						row["span_id"] = d.Attributes.SpanID
						row["parent_id"] = d.Attributes.ParentID
						row["host"] = d.Attributes.Host
						row["type"] = d.Attributes.Type
						row["resource_hash"] = d.Attributes.ResourceHash
						row["single_span"] = d.Attributes.SingleSpan
						if len(d.Attributes.Attributes) > 0 {
							row["attributes"] = d.Attributes.Attributes
						}
						if len(d.Attributes.Custom) > 0 {
							row["custom"] = d.Attributes.Custom
						}
					}
					spans[i] = row
				}

				var meta map[string]any
				if !full && len(resp.Data) > 0 {
					meta = map[string]any{"@skipped": compactSpanSkippedFields}
				}
				shared.WritePaginatedListWithMeta(shared.ToAnySlice(spans), shared.CursorPagination(resp.Cursor()), meta, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Trace search query")
	cmd.Flags().StringVar(&service, "service", "", "Filter by service")
	cmd.Flags().StringVar(&from, "from", "", "Start time (default: now-1h)")
	cmd.Flags().StringVar(&to, "to", "", "End time (default: now)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a previous response's @pagination.next_cursor")
	cmd.Flags().BoolVar(&full, "full", false, "Include all decoded fields (trace_id, span_id, parent_id, host, attributes, custom, etc) — see @skipped meta-line")
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
