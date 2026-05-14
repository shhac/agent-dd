package hosts

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/cli/shared"
	"github.com/shhac/agent-dd/internal/output"
)

func Register(root *cobra.Command, globals func() *shared.GlobalFlags) {
	h := &cobra.Command{
		Use:   "hosts",
		Short: "Infrastructure hosts",
	}

	registerList(h, globals)
	registerGet(h, globals)
	registerMute(h, globals)

	root.AddCommand(h)
}

func registerList(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var search, tag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List hosts",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				resp, err := client.ListHosts(ctx, search, shared.SingleTag(tag))
				if err != nil {
					return err
				}
				compact := make([]map[string]any, len(resp.HostList))
				for i, h := range resp.HostList {
					compact[i] = map[string]any{
						"name":     h.Name,
						"up":       h.Up,
						"is_muted": h.IsMuted,
					}
				}
				var pagination *output.Pagination
				if resp.TotalMatching > resp.TotalReturned {
					pagination = &output.Pagination{HasMore: true, TotalItems: resp.TotalMatching}
				}
				shared.WritePaginatedList(shared.ToAnySlice(compact), pagination, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&search, "search", "", "Filter by hostname")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag")
	parent.AddCommand(cmd)
}

func registerGet(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "get <hostname>",
		Short: "Get host details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				host, err := client.GetHost(ctx, args[0])
				if err != nil {
					return err
				}
				shared.WriteItem(host, g.Format)
				return nil
			})
		},
	}
	parent.AddCommand(cmd)
}

func registerMute(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var end, reason string
	var override bool

	cmd := &cobra.Command{
		Use:   "mute <hostname>",
		Short: "Mute a host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			hostname := args[0]

			var endEpoch int64
			if end != "" {
				t, err := shared.ParseTime(end)
				if err != nil {
					output.WriteError(os.Stderr, err)
					return nil
				}
				endEpoch = t.Unix()
			}

			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				if err := client.MuteHost(ctx, hostname, endEpoch, reason, override); err != nil {
					return err
				}
				shared.WriteItem(map[string]any{
					"status":   "muted",
					"hostname": hostname,
				}, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&end, "end", "", "Mute end time")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for muting")
	cmd.Flags().BoolVar(&override, "override", false, "Override an existing mute (re-mute a host already muted)")
	parent.AddCommand(cmd)
}
