package composio

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/vutran/agent-mesh/pkg/config"
	"github.com/vutran/agent-mesh/pkg/provider"
	"github.com/vutran/agent-mesh/pkg/silo"
)

func init() {
	provider.Register("composio", func(dataDir string) (provider.Provider, error) {
		return NewProvider(dataDir)
	})
}

var serviceConfigs = map[string]ServiceConfig{
	"gmail":   GmailConfig,
	"slack":   SlackConfig,
	"gcal":    GCalConfig,
	"discord": DiscordConfig,
}

var appNames = map[string]string{
	"gmail":   "gmail",
	"slack":   "slack",
	"gcal":    "googlecalendar",
	"discord": "discordbot",
}

// Provider implements provider.Provider for Composio.
type Provider struct {
	client *Client
}

// NewProvider creates a Composio provider from the config directory.
func NewProvider(dataDir string) (*Provider, error) {
	cfg, err := config.Load(dataDir)
	if err != nil {
		return nil, err
	}
	apiKey, err := cfg.ComposioKey()
	if err != nil {
		return nil, err
	}
	var opts []ClientOption
	if cfg.Secrets.ComposioMCPURL != "" {
		opts = append(opts, WithMCPURL(cfg.Secrets.ComposioMCPURL))
	}
	return &Provider{client: NewClient(apiKey, opts...)}, nil
}

func (p *Provider) Name() string { return "composio" }

func (p *Provider) Services() []string {
	keys := make([]string, 0, len(serviceConfigs))
	for k := range serviceConfigs {
		keys = append(keys, k)
	}
	return keys
}

func (p *Provider) Connect(ctx context.Context, service, label string) (*provider.ConnectResult, error) {
	appName, ok := appNames[service]
	if !ok {
		return nil, fmt.Errorf("unsupported service %q", service)
	}

	authConfigID, err := p.client.GetOrCreateAuthConfigID(ctx, appName)
	if err != nil {
		return nil, fmt.Errorf("auth config: %w", err)
	}

	entityID := fmt.Sprintf("agent-mesh:%s:%s", service, label)
	link, err := p.client.CreateConnectLink(ctx, CreateConnectLinkInput{
		AuthConfigID: authConfigID,
		UserID:       entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("create connect link: %w", err)
	}

	return &provider.ConnectResult{
		AuthURL:      link.RedirectURL,
		ConnectionID: link.ConnectedAccountID,
	}, nil
}

func (p *Provider) ConfirmConnection(ctx context.Context, connectionID string) (string, error) {
	return p.client.WaitForActive(ctx, connectionID)
}

func (p *Provider) NewPoller(service, label, connectionID string, logger *slog.Logger) (silo.Poller, error) {
	svcCfg, ok := serviceConfigs[service]
	if !ok {
		return nil, fmt.Errorf("unsupported service %q", service)
	}
	return NewAdapter(p.client, svcCfg, connectionID, label, service, logger), nil
}
