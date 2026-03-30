package provider

import (
	"context"
	"log/slog"

	"github.com/vutran/agent-mesh/pkg/silo"
)

// ConnectResult holds the outcome of initiating an OAuth flow.
type ConnectResult struct {
	AuthURL      string // URL the user should open in their browser
	ConnectionID string // pending connection ID (becomes active after auth)
}

// Provider abstracts an integration backend (Composio, Nango, etc.).
type Provider interface {
	// Name returns the provider key used in connections.toml.
	Name() string

	// Services returns the service slugs this provider supports.
	Services() []string

	// Connect initiates an OAuth flow for a service.
	Connect(ctx context.Context, service, label string) (*ConnectResult, error)

	// ConfirmConnection waits for a specific pending connection to become active.
	ConfirmConnection(ctx context.Context, connectionID string) (string, error)

	// NewPoller creates a silo.Poller for an existing connection.
	NewPoller(service, label, connectionID string, logger *slog.Logger) (silo.Poller, error)
}
