package incidents

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
	inc := &cobra.Command{
		Use:   "incidents",
		Short: "Incident management",
	}

	registerList(inc, globals)
	registerGet(inc, globals)
	registerCreate(inc, globals)
	registerUpdate(inc, globals)
	registerUsage(inc)

	root.AddCommand(inc)
}

func registerList(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var status string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List incidents",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				resp, err := client.ListIncidents(ctx, status)
				if err != nil {
					return err
				}
				compact := make([]map[string]any, len(resp.Data))
				for i, inc := range resp.Data {
					entry := map[string]any{"id": inc.ID}
					if inc.Attributes != nil {
						entry["title"] = inc.Attributes.Title
						entry["state"] = inc.Attributes.State
						entry["severity"] = inc.Attributes.Severity
						entry["customer_impacted"] = inc.Attributes.CustomerImpacted
						if inc.Attributes.PublicID != 0 {
							entry["public_id"] = inc.Attributes.PublicID
						}
					}
					if h := resp.CommanderHandle(i); h != "" {
						entry["commander"] = h
					}
					compact[i] = entry
				}
				var pagination *output.Pagination
				if resp.HasMore() {
					pagination = &output.Pagination{HasMore: true}
				}
				shared.WritePaginatedList(shared.ToAnySlice(compact), pagination, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&status, "state", "", "Filter by state (active, stable, resolved)")
	parent.AddCommand(cmd)
}

func registerGet(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get incident details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				doc, err := client.GetIncident(ctx, args[0])
				if err != nil {
					return err
				}
				// Surface the resolved commander handle alongside the raw
				// incident so callers don't have to walk the included array.
				out := map[string]any{"incident": doc.Data}
				if h := doc.CommanderHandle(); h != "" {
					out["commander"] = h
				}
				shared.WriteItem(out, g.Format)
				return nil
			})
		},
	}
	parent.AddCommand(cmd)
}

func registerCreate(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var title, severity, commanderUUID string
	var customerImpacted bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an incident",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			if !shared.RequireFlag("title", title, "") {
				return nil
			}
			if !shared.RequireFlag("severity", severity, "SEV-1 through SEV-5") {
				return nil
			}

			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				incident, err := client.CreateIncident(ctx, title, severity, commanderUUID, customerImpacted)
				if err != nil {
					return err
				}
				shared.WriteItem(incident, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Incident title (required)")
	cmd.Flags().StringVar(&severity, "severity", "", "Severity: SEV-1 through SEV-5 (required)")
	cmd.Flags().StringVar(&commanderUUID, "commander-uuid", "", "Incident commander Datadog user UUID")
	cmd.Flags().BoolVar(&customerImpacted, "customer-impacted", false, "Mark the incident as customer-impacting")
	parent.AddCommand(cmd)
}

func registerUpdate(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var state, severity string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an incident",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			if state == "" && severity == "" {
				output.WriteError(os.Stderr, agenterrors.New("at least --state or --severity is required", agenterrors.FixableByAgent))
				return nil
			}
			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				incident, err := client.UpdateIncident(ctx, args[0], state, severity)
				if err != nil {
					return err
				}
				shared.WriteItem(incident, g.Format)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "New state (active, stable, resolved)")
	cmd.Flags().StringVar(&severity, "severity", "", "New severity (SEV-1 through SEV-5)")
	parent.AddCommand(cmd)
}
