package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/vutran/agent-mesh/pkg/silo"
)

// cursor tracks the last message ID fetched per channel.
type cursor struct {
	After time.Time `json:"after"`
}

// Adapter polls Discord guilds for messages using a bot token.
type Adapter struct {
	name    string
	label   string
	session *discordgo.Session
	logger  *slog.Logger
}

// NewAdapter creates a Discord adapter with a bot token.
func NewAdapter(token, label string, logger *slog.Logger) (*Adapter, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}

	name := fmt.Sprintf("discord:%s", label)
	return &Adapter{
		name:    name,
		label:   label,
		session: session,
		logger:  logger.With("adapter", name),
	}, nil
}

func (a *Adapter) Name() string           { return a.name }
func (a *Adapter) Source() silo.Source     { return silo.SourceDiscord }
func (a *Adapter) Mode() silo.AdapterMode { return silo.ModePoll }

func (a *Adapter) Poll(ctx context.Context, since silo.Cursor) ([]silo.Message, silo.Cursor, error) {
	var cur cursor
	if since != nil {
		json.Unmarshal(since, &cur)
	}
	if cur.After.IsZero() {
		cur.After = time.Now().Add(-24 * time.Hour)
	}

	guilds, err := a.session.UserGuilds(100, "", "", false)
	if err != nil {
		return nil, nil, fmt.Errorf("list guilds: %w", err)
	}

	var allMsgs []silo.Message

	for _, guild := range guilds {
		channels, err := a.session.GuildChannels(guild.ID)
		if err != nil {
			a.logger.Debug("skip guild (no access)", "guild", guild.Name, "err", err)
			continue
		}

		// Only text channels, limit to 5 per guild
		var textChannels []*discordgo.Channel
		for _, ch := range channels {
			if ch.Type == discordgo.ChannelTypeGuildText {
				textChannels = append(textChannels, ch)
			}
		}
		if len(textChannels) > 5 {
			textChannels = textChannels[:5]
		}

		for _, ch := range textChannels {
			messages, err := a.session.ChannelMessages(ch.ID, 20, "", "", "")
			if err != nil {
				a.logger.Debug("skip channel (no access)", "channel", ch.Name, "err", err)
				continue
			}

			for _, m := range messages {
				ts := time.Time(m.Timestamp)
				if ts.Before(cur.After) {
					continue
				}

				raw, _ := json.Marshal(m)
				preview := m.Content
				if len(preview) > 500 {
					preview = preview[:500]
				}

				msg := silo.Message{
					ID:         fmt.Sprintf("discord:%s:%s", a.label, m.ID),
					Source:     silo.SourceDiscord,
					Sender:     m.Author.Username,
					Subject:    fmt.Sprintf("%s/%s", guild.Name, ch.Name),
					Preview:    preview,
					Raw:        raw,
					CapturedAt: time.Now(),
					SourceTS:   ts,
				}
				allMsgs = append(allMsgs, msg)
			}
		}
	}

	newCursor := cursor{After: time.Now()}
	cursorBytes, _ := json.Marshal(newCursor)

	return allMsgs, silo.Cursor(cursorBytes), nil
}
