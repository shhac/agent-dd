package logs

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
	logs := &cobra.Command{
		Use:   "logs",
		Short: "Log search and analysis",
	}

	registerSearch(logs, globals)
	registerTail(logs, globals)
	registerFacets(logs, globals)
	registerLLMHelp(logs)

	root.AddCommand(logs)
}

func registerSearch(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var query, from, to, sort string
	var limit int
	var full bool

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			if query == "" {
				output.WriteError(os.Stderr, agenterrors.New("--query is required", agenterrors.FixableByAgent))
				return nil
			}

			fromTime, toTime, err := shared.ParseTimeRange(from, to)
			if err != nil {
				output.WriteError(os.Stderr, err)
				return nil
			}

			if limit == 0 {
				limit = 50
			}

			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				resp, err := client.SearchLogs(ctx, query,
					fromTime.Format(time.RFC3339),
					toTime.Format(time.RFC3339),
					sort, limit)
				if err != nil {
					return err
				}

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
					shared.WritePaginatedList(shared.ToAnySlice(entries), nil, g.Format)
					return nil
				}

				compact := make([]api.LogEntryCompact, len(resp.Data))
				for i, d := range resp.Data {
					compact[i] = api.LogEntryCompact{
						Timestamp: d.Attributes.Timestamp,
						Service:   d.Attributes.Service,
						Status:    d.Attributes.Status,
						Message:   d.Attributes.Message,
					}
				}
				shared.WritePaginatedList(shared.ToAnySlice(compact), nil, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Log search query (required)")
	cmd.Flags().StringVar(&from, "from", "", "Start time (default: now-1h)")
	cmd.Flags().StringVar(&to, "to", "", "End time (default: now)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort order: asc or desc")
	cmd.Flags().BoolVar(&full, "full", false, "Show full log entries with all attributes")
	parent.AddCommand(cmd)
}

func registerTail(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var query, source, service string

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Poll recent logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()

			q := query
			if source != "" {
				q += " source:" + source
			}
			if service != "" {
				q += " service:" + service
			}
			if q == "" {
				output.WriteError(os.Stderr, agenterrors.New("--query is required", agenterrors.FixableByAgent))
				return nil
			}

			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				now := time.Now()
				from := now.Add(-5 * time.Minute).Format(time.RFC3339)
				to := now.Format(time.RFC3339)

				resp, err := client.SearchLogs(ctx, q, from, to, "timestamp", 20)
				if err != nil {
					return err
				}

				compact := make([]api.LogEntryCompact, len(resp.Data))
				for i, d := range resp.Data {
					compact[i] = api.LogEntryCompact{
						Timestamp: d.Attributes.Timestamp,
						Service:   d.Attributes.Service,
						Status:    d.Attributes.Status,
						Message:   d.Attributes.Message,
					}
				}
				shared.WritePaginatedList(shared.ToAnySlice(compact), nil, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Log search query (required)")
	cmd.Flags().StringVar(&source, "source", "", "Filter by source")
	cmd.Flags().StringVar(&service, "service", "", "Filter by service")
	parent.AddCommand(cmd)
}

func registerFacets(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var query, from, to string

	cmd := &cobra.Command{
		Use:   "facets",
		Short: "Top facet values for a log query",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			if query == "" {
				output.WriteError(os.Stderr, agenterrors.New("--query is required", agenterrors.FixableByAgent))
				return nil
			}

			fromTime, toTime, err := shared.ParseTimeRange(from, to)
			if err != nil {
				output.WriteError(os.Stderr, err)
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
				output.PrintJSON(resp.Data.Buckets, true)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Log search query (required)")
	cmd.Flags().StringVar(&from, "from", "", "Start time (default: now-1h)")
	cmd.Flags().StringVar(&to, "to", "", "End time (default: now)")
	parent.AddCommand(cmd)
}
