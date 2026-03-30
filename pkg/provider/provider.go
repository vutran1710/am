package provider

import (
	"context"
	"log/slog"

	"github.com/vutran/agent-mesh/pkg/silo"
)

// ConnectResult holds the outcome of initiating an OAuth flow.
type ConnectResult struct {
	AuthURL string // URL the user should open in their browser
}

// Provider abstracts an integration backend (Composio, Nango, etc.).
type Provider interface {
	// Name returns the provider key used in connections.toml.
	Name() string

	// Services returns the service slugs this provider supports.
	Services() []string

	// Connect initiates an OAuth flow for a service.
	Connect(ctx context.Context, service, label string) (*ConnectResult, error)

	// ConfirmConnection verifies that OAuth completed and returns the connection ID.
	ConfirmConnection(ctx context.Context, service, label string) (string, error)

	// NewPoller creates a silo.Poller for an existing connection.
	NewPoller(service, label, connectionID string, logger *slog.Logger) (silo.Poller, error)
}
