package composio

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vutran/agent-mesh/pkg/silo"
)

// fetchSlackMessages lists channels the user is in, then fetches recent
// messages from each via Composio tool execution.
func fetchSlackMessages(ctx context.Context, client *Client, connID, entityID string, since time.Time) ([]silo.Message, error) {

	// Step 1: List channels
	channelsResult, err := client.ExecuteTool(ctx, "SLACK_LIST_CONVERSATIONS", connID, entityID, map[string]any{
		"limit":            50,
		"exclude_archived": true,
	})
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}

	channelIDs := extractChannelIDs(channelsResult.Data)
	if len(channelIDs) == 0 {
		return nil, nil
	}

	// Step 2: Fetch history from each channel
	var allMsgs []silo.Message
	oldest := fmt.Sprintf("%d", since.Unix())

	for _, ch := range channelIDs {
		result, err := client.ExecuteTool(ctx, "SLACK_FETCH_CONVERSATION_HISTORY", connID, entityID, map[string]any{
			"channel": ch.ID,
			"limit":   20,
			"oldest":  oldest,
		})
		if err != nil {
			continue // skip channels that fail (e.g. no access)
		}

		msgs := mapSlackHistory(result.Data, ch.Name)
		allMsgs = append(allMsgs, msgs...)
	}

	return allMsgs, nil
}

type slackChannel struct {
	ID   string
	Name string
}

func extractChannelIDs(data json.RawMessage) []slackChannel {
	// Try {"channels": [{"id":"C...", "name":"general"}]}
	var wrapper struct {
		Channels []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"channels"`
	}
	if json.Unmarshal(data, &wrapper) == nil && len(wrapper.Channels) > 0 {
		var channels []slackChannel
		for _, c := range wrapper.Channels {
			channels = append(channels, slackChannel{ID: c.ID, Name: c.Name})
		}
		return channels
	}

	// Try flat array
	var arr []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if json.Unmarshal(data, &arr) == nil {
		var channels []slackChannel
		for _, c := range arr {
			channels = append(channels, slackChannel{ID: c.ID, Name: c.Name})
		}
		return channels
	}

	return nil
}

func mapSlackHistory(data json.RawMessage, channelName string) []silo.Message {
	// Try {"messages": [{"ts":"...", "text":"...", "user":"..."}]}
	var wrapper struct {
		Messages []struct {
			TS   string `json:"ts"`
			Text string `json:"text"`
			User string `json:"user"`
		} `json:"messages"`
	}
	if json.Unmarshal(data, &wrapper) != nil {
		return nil
	}

	var msgs []silo.Message
	for _, m := range wrapper.Messages {
		raw, _ := json.Marshal(m)

		preview := m.Text
		if len(preview) > 500 {
			preview = preview[:500]
		}

		msg := silo.Message{
			ID:      m.TS,
			Source:  silo.SourceSlack,
			Sender:  m.User,
			Subject: channelName,
			Preview: preview,
			Raw:     raw,
		}

		if ts, err := parseSlackTS(m.TS); err == nil {
			msg.SourceTS = ts
		}

		msgs = append(msgs, msg)
	}

	return msgs
}

func parseSlackTS(ts string) (time.Time, error) {
	var sec, usec int64
	_, err := fmt.Sscanf(ts, "%d.%d", &sec, &usec)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(sec, usec*1000), nil
}
