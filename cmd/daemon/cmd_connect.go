package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/vutran/agent-mesh/pkg/config"
	"github.com/vutran/agent-mesh/pkg/provider"
)

func newAddCmd() *cobra.Command {
	var (
		providerName string
		token        string
	)

	cmd := &cobra.Command{
		Use:   "add <service> <label>",
		Short: "Connect a service (gmail, slack, gcal, discord)",
		Example: `  agent-mesh add gmail personal
  agent-mesh add slack work
  agent-mesh add discord myserver --token <bot-token>`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			service, label := args[0], args[1]

			// Discord uses its own provider with token-based auth
			if service == "discord" {
				if token == "" {
					return fmt.Errorf("discord requires --token <bot-token>\n\n" +
						"1. Create a bot at https://discord.com/developers/applications\n" +
						"2. Copy the bot token\n" +
						"3. Run: agent-mesh add discord %s --token <bot-token>", label)
				}
				providerName = "discord"
			}

			p, err := provider.Get(providerName, dataDir)
			if err != nil {
				return err
			}

			var connID string

			if token != "" {
				// Token-based flow (Discord)
				fmt.Printf("Connecting %s (%s) with token...\n", service, label)
				connID, err = p.ConfirmConnection(ctx, token)
				if err != nil {
					return err
				}
			} else {
				// OAuth flow (Composio, Nango)
				fmt.Printf("Connecting %s (%s) via %s...\n", service, label, p.Name())
				result, err := p.Connect(ctx, service, label)
				if err != nil {
					return err
				}

				fmt.Printf("\nOpen this link to authorize:\n\n  %s\n\n", result.AuthURL)
				fmt.Print("Complete the authorization in your browser, then press Enter...")
				fmt.Scanln()

				fmt.Print("Verifying connection...")
				connID, err = p.ConfirmConnection(ctx, result.ConnectionID)
				if err != nil {
					return fmt.Errorf("authorization not confirmed: %w", err)
				}
			}

			fmt.Printf("Connected! ID: %s\n", connID)

			conn := config.Connection{
				Provider:     providerName,
				Service:      service,
				Label:        label,
				ConnectionID: connID,
				Interval:     config.ToDuration(defaultInterval(service)),
			}

			// Store token on the connection for token-based providers
			if token != "" {
				conn.Token = token
			}

			cfg.AddConnection(conn)

			if err := config.Save(dataDir, cfg); err != nil {
				return err
			}

			fmt.Printf("Saved to %s/config.toml\n", dataDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&providerName, "provider", "composio", "Provider backend (composio, nango, discord)")
	cmd.Flags().StringVar(&token, "token", "", "Service-specific token (e.g. Discord bot token)")

	return cmd
}

func defaultInterval(service string) time.Duration {
	return 5 * time.Minute
}
