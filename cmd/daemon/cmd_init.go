package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vutran/agent-mesh/pkg/config"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Set up agent-mesh (configure API keys)",
		RunE: func(cmd *cobra.Command, args []string) error {
			reader := bufio.NewReader(os.Stdin)

			if cfg.Secrets.ComposioAPIKey != "" {
				fmt.Printf("Composio key already set (%s...%s)\n",
					cfg.Secrets.ComposioAPIKey[:8],
					cfg.Secrets.ComposioAPIKey[len(cfg.Secrets.ComposioAPIKey)-4:])
				fmt.Print("Overwrite? [y/N] ")
				answer, _ := reader.ReadString('\n')
				if strings.TrimSpace(strings.ToLower(answer)) != "y" {
					return nil
				}
			}

			fmt.Println("Get your Composio API key from: https://app.composio.dev/settings")
			fmt.Print("\nComposio API key: ")
			key, _ := reader.ReadString('\n')
			key = strings.TrimSpace(key)
			if key == "" {
				return fmt.Errorf("key cannot be empty")
			}

			cfg.Secrets.ComposioAPIKey = key
			if err := config.Save(dataDir, cfg); err != nil {
				return err
			}

			fmt.Printf("Saved to %s/config.toml\n", dataDir)
			return nil
		},
	}
}
