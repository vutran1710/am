package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newConnectionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connections",
		Short: "List all configured connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(cfg.Connections) == 0 {
				fmt.Println("No connections configured. Run 'agent-mesh add <service> <label>' to add one.")
				return nil
			}

			fmt.Printf("Configured connections (%d):\n\n", len(cfg.Connections))
			for i, c := range cfg.Connections {
				prov := c.Provider
				if prov == "" {
					prov = "composio"
				}
				fmt.Printf("  %d. %s/%s (%s)\n", i+1, c.Service, c.Label, prov)
				fmt.Printf("     Connection ID: %s\n", c.ConnectionID)
				fmt.Printf("     Poll interval: %s\n", c.Interval.Duration())
				fmt.Println()
			}
			return nil
		},
	}
}
