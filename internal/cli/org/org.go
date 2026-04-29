package org

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/cli/shared"
	"github.com/shhac/agent-dd/internal/config"
	"github.com/shhac/agent-dd/internal/credential"
	agenterrors "github.com/shhac/agent-dd/internal/errors"
	"github.com/shhac/agent-dd/internal/output"
)

func Register(root *cobra.Command) {
	org := &cobra.Command{
		Use:   "org",
		Short: "Manage Datadog organization credentials",
	}

	registerAdd(org)
	registerUpdate(org)
	registerRemove(org)
	registerList(org)
	registerSetDefault(org)
	registerTest(org)

	root.AddCommand(org)
}

func registerAdd(parent *cobra.Command) {
	var apiKey, appKey, site string

	cmd := &cobra.Command{
		Use:   "add <alias>",
		Short: "Add a Datadog organization",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := args[0]

			if apiKey == "" || appKey == "" {
				err := agenterrors.New("both --api-key and --app-key are required", agenterrors.FixableByAgent)
				output.WriteError(os.Stderr, err)
				return nil
			}

			storage, err := credential.Store(alias, credential.Credential{
				APIKey: apiKey,
				AppKey: appKey,
			})
			if err != nil {
				output.WriteError(os.Stderr, agenterrors.Wrap(err, agenterrors.FixableByHuman))
				return nil
			}

			org := config.Organization{}
			if site != "" {
				org.Site = site
			}
			if err := config.StoreOrganization(alias, org); err != nil {
				output.WriteError(os.Stderr, agenterrors.Wrap(err, agenterrors.FixableByHuman))
				return nil
			}

			output.PrintJSON(map[string]any{
				"status":  "added",
				"alias":   alias,
				"storage": storage,
				"site":    siteOrDefault(site),
			}, true)
			return nil
		},
	}
	cmd.Flags().StringVar(&apiKey, "api-key", "", "Datadog API key (required)")
	cmd.Flags().StringVar(&appKey, "app-key", "", "Datadog application key (required)")
	cmd.Flags().StringVar(&site, "site", "", "Datadog site (default: datadoghq.com)")
	parent.AddCommand(cmd)
}

func registerUpdate(parent *cobra.Command) {
	var apiKey, appKey, site string

	cmd := &cobra.Command{
		Use:   "update <alias>",
		Short: "Update a Datadog organization",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := args[0]

			if err := updateCredentials(alias, apiKey, appKey); err != nil {
				output.WriteError(os.Stderr, err)
				return nil
			}

			if err := updateSite(alias, site); err != nil {
				output.WriteError(os.Stderr, err)
				return nil
			}

			output.PrintJSON(map[string]any{
				"status": "updated",
				"alias":  alias,
			}, true)
			return nil
		},
	}
	cmd.Flags().StringVar(&apiKey, "api-key", "", "Datadog API key")
	cmd.Flags().StringVar(&appKey, "app-key", "", "Datadog application key")
	cmd.Flags().StringVar(&site, "site", "", "Datadog site")
	parent.AddCommand(cmd)
}

func registerRemove(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "remove <alias>",
		Short: "Remove a Datadog organization",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := args[0]

			if err := credential.Remove(alias); err != nil {
				output.WriteError(os.Stderr, agenterrors.Wrap(err, agenterrors.FixableByHuman))
				return nil
			}
			if err := config.RemoveOrganization(alias); err != nil {
				output.WriteError(os.Stderr, agenterrors.Wrap(err, agenterrors.FixableByHuman))
				return nil
			}

			output.PrintJSON(map[string]any{
				"status": "removed",
				"alias":  alias,
			}, true)
			return nil
		},
	}
	parent.AddCommand(cmd)
}

func registerList(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured organizations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Read()
			names, err := credential.List()
			if err != nil {
				output.WriteError(os.Stderr, agenterrors.Wrap(err, agenterrors.FixableByHuman))
				return nil
			}

			orgs := make([]map[string]any, 0, len(names))
			for _, name := range names {
				entry := map[string]any{
					"alias":      name,
					"is_default": name == cfg.DefaultOrg,
				}
				if org, ok := cfg.Organizations[name]; ok && org.Site != "" {
					entry["site"] = org.Site
				} else {
					entry["site"] = "datadoghq.com"
				}
				orgs = append(orgs, entry)
			}

			output.PrintJSON(map[string]any{"organizations": orgs}, true)
			return nil
		},
	}
	parent.AddCommand(cmd)
}

func registerSetDefault(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "set-default <alias>",
		Short: "Set the default organization",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := args[0]
			if err := config.SetDefault(alias); err != nil {
				output.WriteError(os.Stderr, agenterrors.Wrap(err, agenterrors.FixableByHuman))
				return nil
			}
			output.PrintJSON(map[string]any{
				"status": "default_set",
				"alias":  alias,
			}, true)
			return nil
		},
	}
	parent.AddCommand(cmd)
}

func registerTest(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test credentials against Datadog API",
		RunE: func(cmd *cobra.Command, args []string) error {
			orgFlag, _ := cmd.Flags().GetString("org")
			return shared.WithClient(orgFlag, 0, func(ctx context.Context, client *api.Client) error {
				if err := client.Validate(ctx); err != nil {
					return err
				}
				output.PrintJSON(map[string]any{
					"status": "ok",
				}, true)
				return nil
			})
		},
	}
	cmd.Flags().StringP("org", "o", "", "Organization alias to test")
	parent.AddCommand(cmd)
}

func siteOrDefault(site string) string {
	if site == "" {
		return "datadoghq.com"
	}
	return site
}

func updateCredentials(alias, apiKey, appKey string) error {
	if apiKey == "" && appKey == "" {
		return nil
	}

	existing, err := credential.Get(alias)
	if err != nil {
		return agenterrors.Wrap(err, agenterrors.FixableByHuman)
	}
	if apiKey == "" {
		apiKey = existing.APIKey
	}
	if appKey == "" {
		appKey = existing.AppKey
	}
	if _, err := credential.Store(alias, credential.Credential{
		APIKey: apiKey,
		AppKey: appKey,
	}); err != nil {
		return agenterrors.Wrap(err, agenterrors.FixableByHuman)
	}
	return nil
}

func updateSite(alias, site string) error {
	if site == "" {
		return nil
	}
	cfg := config.Read()
	org := cfg.Organizations[alias]
	org.Site = site
	if err := config.StoreOrganization(alias, org); err != nil {
		return agenterrors.Wrap(err, agenterrors.FixableByHuman)
	}
	return nil
}
