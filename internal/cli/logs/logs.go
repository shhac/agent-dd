package logs

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/cli/shared"
	"github.com/shhac/agent-dd/internal/output"
)

func toCompactLogs(data []api.LogData) []map[string]any {
	compact := make([]map[string]any, len(data))
	for i, d := range data {
		row := map[string]any{
			"id":        d.ID,
			"timestamp": d.Attributes.Timestamp,
			"service":   d.Attributes.Service,
			"status":    d.Attributes.Status,
			"host":      d.Attributes.Host,
			"message":   d.Attributes.Message,
		}
		if len(d.Attributes.Tags) > 0 {
			row["tags"] = d.Attributes.Tags
		}
		compact[i] = row
	}
	return compact
}

func Register(root *cobra.Command, globals func() *shared.GlobalFlags) {
	logs := &cobra.Command{
		Use:   "logs",
		Short: "Log search and analysis",
	}

	registerSearch(logs, globals)
	registerTail(logs, globals)
	registerFacets(logs, globals)
	registerUsage(logs)

	root.AddCommand(logs)
}

// compactLogSkippedFields lists the LogAttributes fields hidden from the
// default `logs search` output. Identifiers and triage context (id, host,
// tags) stay in the default view — only the potentially-large free-form
// `attributes` blob is hidden. Surfaced as a `@skipped` meta-line.
var compactLogSkippedFields = []string{"attributes"}

func registerSearch(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var query, from, to, sort, cursor, storageTier string
	var limit int
	var full bool

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			if !shared.RequireFlag("query", query, "") {
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
				resp, err := client.SearchLogs(ctx, query,
					fromTime.Format(time.RFC3339),
					toTime.Format(time.RFC3339),
					sort, limit, cursor, storageTier)
				if err != nil {
					return err
				}

				pagination := shared.CursorPagination(resp.Cursor())

				if full {
					entries := make([]api.LogEntry, len(resp.Data))
					for i, d := range resp.Data {
						entries[i] = api.LogEntry{
							ID:         d.ID,
							Timestamp:  d.Attributes.Timestamp,
							Service:    d.Attributes.Service,
							Status:     d.Attributes.Status,
							Message:    d.Attributes.Message,
							Host:       d.Attributes.Host,
							Tags:       d.Attributes.Tags,
							Attributes: d.Attributes.Attributes,
						}
					}
					shared.WritePaginatedList(shared.ToAnySlice(entries), pagination, g.Format)
					return nil
				}

				var meta map[string]any
				if len(resp.Data) > 0 {
					meta = map[string]any{output.MetaKeySkipped: compactLogSkippedFields}
				}
				shared.WritePaginatedListWithMeta(shared.ToAnySlice(toCompactLogs(resp.Data)), pagination, meta, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Log search query (required)")
	cmd.Flags().StringVar(&from, "from", "", "Start time (default: now-1h)")
	cmd.Flags().StringVar(&to, "to", "", "End time (default: now)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort order: asc or desc")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a previous response")
	cmd.Flags().StringVar(&storageTier, "storage-tier", "", "Retention tier: indexes (default), online-archives, or flex")
	cmd.Flags().BoolVar(&full, "full", false, "Include the free-form attributes blob (large) — see @skipped meta-line")
	parent.AddCommand(cmd)
}

func registerTail(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var query, source, service string
	var interval int
	var follow bool

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Poll recent logs (streams with --follow)",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()

			q := query
			if source != "" {
				q += " source:" + source
			}
			if service != "" {
				q += " service:" + service
			}
			if !shared.RequireFlag("query", q, "") {
				return nil
			}

			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				w := output.NewNDJSONWriter(os.Stdout)
				from := time.Now().Add(-5 * time.Minute).Format(time.RFC3339)
				to := time.Now().Format(time.RFC3339)

				resp, err := client.SearchLogs(ctx, q, from, to, "timestamp", 20, "", "")
				if err != nil {
					return err
				}
				for _, c := range toCompactLogs(resp.Data) {
					_ = w.WriteItem(c)
				}

				if !follow {
					return nil
				}

				ticker := time.NewTicker(time.Duration(interval) * time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return nil
					case <-ticker.C:
						from = to
						to = time.Now().Format(time.RFC3339)
						resp, err = client.SearchLogs(ctx, q, from, to, "timestamp", 100, "", "")
						if err != nil {
							return err
						}
						for _, c := range toCompactLogs(resp.Data) {
							_ = w.WriteItem(c)
						}
					}
				}
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Log search query (required)")
	cmd.Flags().StringVar(&source, "source", "", "Filter by source")
	cmd.Flags().StringVar(&service, "service", "", "Filter by service")
	cmd.Flags().BoolVar(&follow, "follow", false, "Stream continuously")
	cmd.Flags().IntVar(&interval, "interval", 5, "Poll interval in seconds (with --follow)")
	parent.AddCommand(cmd)
}

func registerFacets(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var query, from, to string

	cmd := &cobra.Command{
		Use:   "facets",
		Short: "Top facet values for a log query",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			if !shared.RequireFlag("query", query, "") {
				return nil
			}

			fromTime, toTime, ok := shared.ParseTimeRangeOrWriteErr(from, to)
			if !ok {
				return nil
			}

			groupBy := []string{"service", "status", "host"}

			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				resp, err := client.AggregateLogs(ctx, query,
					fromTime.Format(time.RFC3339),
					toTime.Format(time.RFC3339),
					groupBy)
				if err != nil {
					return err
				}
				shared.WritePaginatedList(shared.ToAnySlice(resp.Data.Buckets), nil, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Log search query (required)")
	cmd.Flags().StringVar(&from, "from", "", "Start time (default: now-1h)")
	cmd.Flags().StringVar(&to, "to", "", "End time (default: now)")
	parent.AddCommand(cmd)
}
