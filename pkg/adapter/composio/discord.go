package composio

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vutran/agent-mesh/pkg/silo"
)

// fetchDiscordMessages lists guilds → channels → fetches recent messages.
func fetchDiscordMessages(ctx context.Context, client *Client, connID, entityID string, since time.Time) ([]silo.Message, error) {
	// Step 1: List guilds
	guildsResult, err := client.ExecuteTool(ctx, "DISCORDBOT_LIST_MY_GUILDS", connID, entityID, map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("list guilds: %w", err)
	}

	guildIDs := extractGuildIDs(guildsResult.Data)
	if len(guildIDs) == 0 {
		return nil, nil
	}

	// Step 2: For each guild, list text channels
	var allMsgs []silo.Message
	for _, guild := range guildIDs {
		channelsResult, err := client.ExecuteTool(ctx, "DISCORDBOT_LIST_GUILD_CHANNELS", connID, entityID, map[string]any{
			"guild_id": guild.ID,
		})
		if err != nil {
			continue
		}

		channels := extractTextChannels(channelsResult.Data)

		// Step 3: Fetch messages from each text channel (limit to first 5 channels)
		limit := 5
		if len(channels) < limit {
			limit = len(channels)
		}
		for _, ch := range channels[:limit] {
			msgsResult, err := client.ExecuteTool(ctx, "DISCORDBOT_LIST_MESSAGES", connID, entityID, map[string]any{
				"channel_id": ch.ID,
				"limit":      20,
			})
			if err != nil {
				continue
			}

			msgs := mapDiscordMessages(msgsResult.Data, guild.Name, ch.Name)
			allMsgs = append(allMsgs, msgs...)
		}
	}

	return allMsgs, nil
}

type discordGuild struct {
	ID   string
	Name string
}

type discordChannel struct {
	ID   string
	Name string
}

func extractGuildIDs(data json.RawMessage) []discordGuild {
	var arr []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if json.Unmarshal(data, &arr) == nil {
		var guilds []discordGuild
		for _, g := range arr {
			guilds = append(guilds, discordGuild{ID: g.ID, Name: g.Name})
		}
		return guilds
	}
	return nil
}

func extractTextChannels(data json.RawMessage) []discordChannel {
	// Discord channel type 0 = text channel
	var arr []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type int    `json:"type"`
	}
	if json.Unmarshal(data, &arr) == nil {
		var channels []discordChannel
		for _, c := range arr {
			if c.Type == 0 { // text channel
				channels = append(channels, discordChannel{ID: c.ID, Name: c.Name})
			}
		}
		return channels
	}
	return nil
}

func mapDiscordMessages(data json.RawMessage, guildName, channelName string) []silo.Message {
	var arr []struct {
		ID        string `json:"id"`
		Content   string `json:"content"`
		Timestamp string `json:"timestamp"`
		Author    struct {
			Username string `json:"username"`
		} `json:"author"`
	}
	if json.Unmarshal(data, &arr) != nil {
		return nil
	}

	var msgs []silo.Message
	for _, m := range arr {
		raw, _ := json.Marshal(m)

		preview := m.Content
		if len(preview) > 500 {
			preview = preview[:500]
		}

		msg := silo.Message{
			ID:      m.ID,
			Source:  silo.SourceDiscord,
			Sender:  m.Author.Username,
			Subject: fmt.Sprintf("%s/%s", guildName, channelName),
			Preview: preview,
			Raw:     raw,
		}

		if t, err := time.Parse(time.RFC3339, m.Timestamp); err == nil {
			msg.SourceTS = t
		}

		msgs = append(msgs, msg)
	}

	return msgs
}
