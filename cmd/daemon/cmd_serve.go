package main

import (
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the daemon (polls all connections)",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := NewDaemon(cfg, logger)
			if err != nil {
				return err
			}

			logger.Info("starting agent-mesh daemon", "addr", cfg.Daemon.Addr, "data_dir", dataDir)
			if err := d.Run(cmd.Context()); err != nil {
				return err
			}
			logger.Info("daemon stopped")
			return nil
		},
	}
}
