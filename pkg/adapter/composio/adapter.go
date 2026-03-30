package composio

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/vutran/agent-mesh/pkg/silo"
)

// cursor tracks what we've already fetched.
type cursor struct {
	After time.Time `json:"after"`
}

// FetchFunc is a custom fetch implementation that bypasses Composio tool execution.
// Used when the service's native API is more capable (e.g. Slack search.messages).
type FetchFunc func(ctx context.Context, client *Client, connID, entityID string, since time.Time) ([]silo.Message, error)

// ServiceConfig defines how to poll a Composio integration.
type ServiceConfig struct {
	Source    silo.Source
	AppName   string
	FetchTool string                                    // composio tool slug (ignored if FetchFn set)
	InputFn   func(since time.Time) map[string]any      // builds tool input (ignored if FetchFn set)
	MapFn     func(data json.RawMessage) []silo.Message  // maps tool output (ignored if FetchFn set)
	FetchFn   FetchFunc                                  // custom fetch, bypasses tool execution
}

var GmailConfig = ServiceConfig{
	Source:    silo.SourceGmail,
	AppName:   "gmail",
	FetchTool: "GMAIL_FETCH_EMAILS",
	InputFn: func(since time.Time) map[string]any {
		m := map[string]any{
			"max_results": 50,
		}
		if !since.IsZero() {
			m["query"] = fmt.Sprintf("after:%d", since.Unix())
		}
		return m
	},
	MapFn: mapGmailEmails,
}

var SlackConfig = ServiceConfig{
	Source:  silo.SourceSlack,
	AppName: "slack",
	FetchFn: fetchSlackMessages,
}

var GCalConfig = ServiceConfig{
	Source:    silo.SourceGCal,
	AppName:   "google_calendar",
	FetchTool: "GOOGLECALENDAR_FIND_EVENT",
	InputFn: func(since time.Time) map[string]any {
		return map[string]any{}
	},
	MapFn: mapGenericItems,
}

var DiscordConfig = ServiceConfig{
	Source:  silo.SourceDiscord,
	AppName: "discordbot",
	FetchFn: fetchDiscordMessages,
}

// Adapter polls a Composio integration via tool execution.
type Adapter struct {
	name               string
	connectedAccountID string
	entityID           string
	service            ServiceConfig
	client             *Client
	logger             *slog.Logger
}

// NewAdapter creates a Composio adapter.
func NewAdapter(client *Client, service ServiceConfig, connectedAccountID, label string, logger *slog.Logger) *Adapter {
	name := fmt.Sprintf("composio:%s:%s", service.AppName, label)
	entityID := fmt.Sprintf("agent-mesh:%s:%s", service.AppName, label)
	return &Adapter{
		name:               name,
		connectedAccountID: connectedAccountID,
		entityID:           entityID,
		service:            service,
		client:             client,
		logger:             logger.With("adapter", name),
	}
}

func (a *Adapter) Name() string           { return a.name }
func (a *Adapter) Source() silo.Source     { return a.service.Source }
func (a *Adapter) Mode() silo.AdapterMode { return silo.ModePoll }

func (a *Adapter) Poll(ctx context.Context, since silo.Cursor) ([]silo.Message, silo.Cursor, error) {
	var cur cursor
	if since != nil {
		json.Unmarshal(since, &cur)
	}
	if cur.After.IsZero() {
		cur.After = time.Now().Add(-24 * time.Hour)
	}

	var msgs []silo.Message
	var err error

	if a.service.FetchFn != nil {
		// Custom fetch (e.g. direct Slack API)
		msgs, err = a.service.FetchFn(ctx, a.client, a.connectedAccountID, a.entityID, cur.After)
	} else {
		// Standard Composio tool execution
		msgs, err = a.pollViaTool(ctx, cur.After)
	}
	if err != nil {
		return nil, nil, err
	}

	// Prefix IDs
	for i := range msgs {
		msgs[i].ID = fmt.Sprintf("%s:%s:%s", a.service.Source, a.connectedAccountID, msgs[i].ID)
		msgs[i].CapturedAt = time.Now()
	}

	newCursor := cursor{After: time.Now()}
	cursorBytes, _ := json.Marshal(newCursor)

	return msgs, silo.Cursor(cursorBytes), nil
}

func (a *Adapter) pollViaTool(ctx context.Context, since time.Time) ([]silo.Message, error) {
	input := a.service.InputFn(since)
	a.logger.Debug("executing tool", "tool", a.service.FetchTool)

	result, err := a.client.ExecuteTool(ctx, a.service.FetchTool, a.connectedAccountID, a.entityID, input)
	if err != nil {
		return nil, fmt.Errorf("execute %s: %w", a.service.FetchTool, err)
	}

	return a.service.MapFn(result.Data), nil
}

// --- Mappers ---

// gmailEmail matches the actual Composio GMAIL_FETCH_EMAILS response shape.
type gmailEmail struct {
	MessageID        string          `json:"messageId"`
	Sender           string          `json:"sender"`
	Subject          string          `json:"subject"`
	MessageTimestamp string          `json:"messageTimestamp"`
	ThreadID         string          `json:"threadId"`
	To               string          `json:"to"`
	LabelIDs         []string        `json:"labelIds"`
	Preview          json.RawMessage `json:"preview"` // can be object {"body":"..."} or string
}

func mapGmailEmails(data json.RawMessage) []silo.Message {
	// Response is {"messages": [...]}
	var wrapper struct {
		Messages []gmailEmail `json:"messages"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		// Fallback: try as flat array
		var arr []gmailEmail
		if json.Unmarshal(data, &arr) == nil {
			wrapper.Messages = arr
		}
	}

	var msgs []silo.Message
	for _, e := range wrapper.Messages {
		raw, _ := json.Marshal(e)

		preview := extractPreview(e.Preview)
		if len(preview) > 500 {
			preview = preview[:500]
		}

		msg := silo.Message{
			ID:      e.MessageID,
			Source:  silo.SourceGmail,
			Sender:  e.Sender,
			Subject: e.Subject,
			Preview: preview,
			Raw:     raw,
		}

		if t, err := time.Parse(time.RFC3339, e.MessageTimestamp); err == nil {
			msg.SourceTS = t
		}
		msgs = append(msgs, msg)
	}
	return msgs
}

// extractPreview handles preview being either {"body":"..."} or a plain string.
func extractPreview(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try object with body field
	var obj struct {
		Body string `json:"body"`
	}
	if json.Unmarshal(raw, &obj) == nil && obj.Body != "" {
		return obj.Body
	}
	// Try plain string
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

func mapGenericItems(data json.RawMessage) []silo.Message {
	return []silo.Message{
		{
			ID:  fmt.Sprintf("%d", time.Now().UnixNano()),
			Raw: data,
		},
	}
}
