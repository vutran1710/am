package discord

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/vutran/agent-mesh/pkg/provider"
	"github.com/vutran/agent-mesh/pkg/silo"
)

func init() {
	provider.Register("discord", func(dataDir string) (provider.Provider, error) {
		return &Provider{}, nil
	})
}

// Provider implements provider.Provider for Discord (direct bot token).
// The bot token lives on each connection, not in global secrets.
type Provider struct{}

func (p *Provider) Name() string       { return "discord" }
func (p *Provider) Services() []string { return []string{"discord"} }

func (p *Provider) Connect(ctx context.Context, service, label string) (*provider.ConnectResult, error) {
	return nil, fmt.Errorf("discord requires a bot token\n\n" +
		"1. Create a bot at https://discord.com/developers/applications\n" +
		"2. Copy the bot token\n" +
		"3. Run: agent-mesh add discord <label> --token <bot-token>")
}

func (p *Provider) ConfirmConnection(ctx context.Context, connectionID string) (string, error) {
	// connectionID is the bot token for discord
	session, err := discordgo.New("Bot " + connectionID)
	if err != nil {
		return "", fmt.Errorf("invalid bot token: %w", err)
	}

	app, err := session.Application("@me")
	if err != nil {
		return "", fmt.Errorf("cannot connect with token: %w", err)
	}

	// Generate invite link
	inviteURL := fmt.Sprintf(
		"https://discord.com/api/oauth2/authorize?client_id=%s&permissions=66560&scope=bot",
		app.ID,
	)
	fmt.Printf("\nInvite the bot to your server:\n\n  %s\n\n", inviteURL)

	return fmt.Sprintf("bot:%s", app.ID), nil
}

func (p *Provider) NewPoller(service, label, connectionID string, logger *slog.Logger) (silo.Poller, error) {
	// connectionID is the bot token for discord
	if connectionID == "" {
		return nil, fmt.Errorf("no bot token for discord connection %s", label)
	}
	return NewAdapter(connectionID, label, logger)
}
