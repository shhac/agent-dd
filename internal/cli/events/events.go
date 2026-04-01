package events

import (
	"context"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/cli/shared"
	agenterrors "github.com/shhac/agent-dd/internal/errors"
	"github.com/shhac/agent-dd/internal/output"
)

func Register(root *cobra.Command, globals func() *shared.GlobalFlags) {
	ev := &cobra.Command{
		Use:   "events",
		Short: "Event stream",
	}

	registerList(ev, globals)
	registerGet(ev, globals)

	root.AddCommand(ev)
}

func registerList(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var from, to, source, tag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List events",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()

			fromTime, toTime, err := shared.ParseTimeRange(from, to)
			if err != nil {
				output.WriteError(os.Stderr, err)
				return nil
			}

			var tags []string
			if tag != "" {
				tags = []string{tag}
			}

			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				events, err := client.ListEvents(ctx, fromTime.Unix(), toTime.Unix(), source, tags)
				if err != nil {
					return err
				}
				shared.WritePaginatedList(shared.ToAnySlice(events), nil, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Start time (default: now-1h)")
	cmd.Flags().StringVar(&to, "to", "", "End time (default: now)")
	cmd.Flags().StringVar(&source, "source", "", "Filter by source")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag")
	parent.AddCommand(cmd)
}

func registerGet(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get event details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				output.WriteError(os.Stderr, agenterrors.Newf(agenterrors.FixableByAgent, "invalid event ID %q", args[0]))
				return nil
			}
			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				event, err := client.GetEvent(ctx, id)
				if err != nil {
					return err
				}
				output.PrintJSON(event, true)
				return nil
			})
		},
	}
	parent.AddCommand(cmd)
}
