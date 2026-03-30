package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/vutran/agent-mesh/pkg/config"
	"github.com/vutran/agent-mesh/pkg/log"

	// Register providers via init()
	_ "github.com/vutran/agent-mesh/pkg/adapter/composio"
	_ "github.com/vutran/agent-mesh/pkg/adapter/nango"
)

var (
	dataDir string
	cfg     *config.Config
	logger  = log.New("info")
)

func main() {
	dataDir = config.DataDir()

	var err error
	cfg, err = config.Load(dataDir)
	if err != nil {
		logger.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	logger = log.New(cfg.Daemon.LogLevel)

	root := &cobra.Command{
		Use:   "agent-mesh",
		Short: "Unified message silo daemon",
	}

	root.AddCommand(
		newInitCmd(),
		newServeCmd(),
		newAddCmd(),
		newPollCmd(),
		newMessagesCmd(),
		newConnectionsCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
