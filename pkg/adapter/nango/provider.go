package nango

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/vutran/agent-mesh/pkg/config"
	"github.com/vutran/agent-mesh/pkg/provider"
	"github.com/vutran/agent-mesh/pkg/silo"
)

func init() {
	provider.Register("nango", func(dataDir string) (provider.Provider, error) {
		return NewProvider(dataDir)
	})
}

var serviceConfigs = map[string]ServiceConfig{
	"gmail": GmailConfig,
	"slack": SlackConfig,
	"gcal":  GCalConfig,
}

var integrationKeys = map[string]string{
	"gmail": "google-mail",
	"slack": "slack",
	"gcal":  "google-calendar",
}

// Provider implements provider.Provider for Nango.
type Provider struct {
	client *Client
}

// NewProvider creates a Nango provider from the config directory.
func NewProvider(dataDir string) (*Provider, error) {
	cfg, err := config.Load(dataDir)
	if err != nil {
		return nil, err
	}
	secretKey, err := cfg.NangoKey()
	if err != nil {
		return nil, err
	}
	return &Provider{client: NewClient(secretKey)}, nil
}

func (p *Provider) Name() string { return "nango" }

func (p *Provider) Services() []string {
	keys := make([]string, 0, len(serviceConfigs))
	for k := range serviceConfigs {
		keys = append(keys, k)
	}
	return keys
}

func (p *Provider) Connect(ctx context.Context, service, label string) (*provider.ConnectResult, error) {
	integrationKey, ok := integrationKeys[service]
	if !ok {
		return nil, fmt.Errorf("unsupported service %q", service)
	}

	sess, err := p.client.CreateConnectSession(ctx, CreateConnectSessionInput{
		EndUserID:           fmt.Sprintf("agent-mesh:%s:%s", service, label),
		AllowedIntegrations: []string{integrationKey},
	})
	if err != nil {
		return nil, fmt.Errorf("create connect session: %w", err)
	}

	return &provider.ConnectResult{AuthURL: sess.ConnectLink}, nil
}

func (p *Provider) ConfirmConnection(ctx context.Context, service, label string) (string, error) {
	integrationKey, ok := integrationKeys[service]
	if !ok {
		return "", fmt.Errorf("unsupported service %q", service)
	}

	before, _ := p.client.ListConnections(ctx, "")
	existingIDs := make(map[string]bool)
	for _, c := range before {
		existingIDs[c.ConnectionID] = true
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	timeout := time.After(30 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("authorization timed out")
		case <-ticker.C:
			conns, err := p.client.ListConnections(ctx, "")
			if err != nil {
				continue
			}
			for _, c := range conns {
				if c.ProviderConfigKey == integrationKey && !existingIDs[c.ConnectionID] {
					return c.ConnectionID, nil
				}
			}
		}
	}
}

func (p *Provider) NewPoller(service, label, connectionID string, logger *slog.Logger) (silo.Poller, error) {
	svcCfg, ok := serviceConfigs[service]
	if !ok {
		return nil, fmt.Errorf("unsupported service %q", service)
	}
	return NewAdapter(p.client, svcCfg, connectionID, label, logger), nil
}
