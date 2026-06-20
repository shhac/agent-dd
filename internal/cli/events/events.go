package events

import (
	"context"

	libcli "github.com/shhac/lib-agent-cli/cli"
	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/cli/shared"
)

func Register(root *cobra.Command, globals func() *shared.GlobalFlags) {
	ev := &cobra.Command{
		Use:   "events",
		Short: "Event stream",
	}

	registerList(ev, globals)
	registerGet(ev, globals)

	libcli.HandleUnknownCommand(ev, "run 'agent-dd usage' to see the available domains")
	root.AddCommand(ev)
}

func registerList(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var from, to, source, tag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List events",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()

			fromTime, toTime, ok := shared.ParseTimeRangeOrWriteErr(from, to)
			if !ok {
				return nil
			}

			return shared.WithClient(g.Org, g.TimeoutMS, g.Debug, func(ctx context.Context, client *api.Client) error {
				events, err := client.ListEvents(ctx, fromTime.Unix(), toTime.Unix(), source, shared.SingleTag(tag))
				if err != nil {
					return err
				}
				shared.WritePaginatedList(events, nil, g.Format)
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
		Use:   "get <id>...",
		Short: "Get event details (one or more IDs)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return shared.GetEntities(globals(), args, func(ctx context.Context, client *api.Client, id string) (any, error) {
				n, err := shared.ParseIntID("event ID", id)
				if err != nil {
					return nil, err
				}
				return client.GetEvent(ctx, n)
			})
		},
	}
	parent.AddCommand(cmd)
}
