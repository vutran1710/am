package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/vutran/agent-mesh/pkg/config"
	"github.com/vutran/agent-mesh/pkg/provider"
)

func newAddCmd() *cobra.Command {
	var providerName string

	cmd := &cobra.Command{
		Use:   "add <service> <label>",
		Short: "Connect a service (gmail, slack, gcal, discord)",
		Example: `  agent-mesh add gmail personal
  agent-mesh add gmail work
  agent-mesh add slack work --provider nango`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			service, label := args[0], args[1]

			p, err := provider.Get(providerName, dataDir)
			if err != nil {
				return err
			}

			fmt.Printf("Connecting %s (%s) via %s...\n", service, label, p.Name())

			result, err := p.Connect(ctx, service, label)
			if err != nil {
				return err
			}

			fmt.Printf("\nOpen this link to authorize:\n\n  %s\n\n", result.AuthURL)
			fmt.Print("Complete the authorization in your browser, then press Enter...")
			fmt.Scanln()

			connID, err := p.ConfirmConnection(ctx, service, label)
			if err != nil {
				return fmt.Errorf("authorization not confirmed: %w", err)
			}

			fmt.Printf("Connected! ID: %s\n", connID)

			cfg.AddConnection(config.Connection{
				Provider:     providerName,
				Service:      service,
				Label:        label,
				ConnectionID: connID,
				Interval:     config.ToDuration(defaultInterval(service)),
			})

			if err := config.Save(dataDir, cfg); err != nil {
				return err
			}

			fmt.Printf("Saved to %s/config.toml\n", dataDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&providerName, "provider", "composio", "Provider backend (composio, nango)")

	return cmd
}

func defaultInterval(service string) time.Duration {
	switch service {
	case "slack", "discord":
		return 1 * time.Minute
	case "gcal":
		return 10 * time.Minute
	default:
		return 5 * time.Minute
	}
}
