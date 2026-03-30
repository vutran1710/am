package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/vutran/agent-mesh/pkg/silo"
	"golang.org/x/oauth2"
	googleapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// GmailClient abstracts the Gmail API for testability.
type GmailClient interface {
	ListMessages(ctx context.Context, query string, pageToken string, maxResults int64) (*MessageList, error)
	GetMessage(ctx context.Context, id string) (*RawEmail, error)
}

// MessageList is a simplified representation of a Gmail list response.
type MessageList struct {
	Messages      []MessageRef
	NextPageToken string
}

// MessageRef is a reference to a message (ID only, from list response).
type MessageRef struct {
	ID string
}

// RawEmail holds the data we extract from a full Gmail message.
type RawEmail struct {
	ID      string
	From    string
	Subject string
	Snippet string
	Date    time.Time
	Labels  []string
	Body    string // plain text body (best effort)
	Raw     json.RawMessage
}

// liveClient wraps the real Gmail API service.
type liveClient struct {
	svc    *googleapi.Service
	userID string
}

func newLiveClient(ctx context.Context, tokenSource oauth2.TokenSource, userID string) (*liveClient, error) {
	svc, err := googleapi.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("create gmail service: %w", err)
	}
	return &liveClient{svc: svc, userID: userID}, nil
}

func (c *liveClient) ListMessages(ctx context.Context, query string, pageToken string, maxResults int64) (*MessageList, error) {
	call := c.svc.Users.Messages.List(c.userID).Context(ctx).Q(query).MaxResults(maxResults)
	if pageToken != "" {
		call = call.PageToken(pageToken)
	}
	resp, err := call.Do()
	if err != nil {
		return nil, err
	}
	result := &MessageList{NextPageToken: resp.NextPageToken}
	for _, m := range resp.Messages {
		result.Messages = append(result.Messages, MessageRef{ID: m.Id})
	}
	return result, nil
}

func (c *liveClient) GetMessage(ctx context.Context, id string) (*RawEmail, error) {
	msg, err := c.svc.Users.Messages.Get(c.userID, id).Context(ctx).Format("full").Do()
	if err != nil {
		return nil, err
	}
	return parseGmailMessage(msg)
}

func parseGmailMessage(msg *googleapi.Message) (*RawEmail, error) {
	email := &RawEmail{
		ID:      msg.Id,
		Snippet: msg.Snippet,
		Labels:  msg.LabelIds,
	}

	for _, h := range msg.Payload.Headers {
		switch strings.ToLower(h.Name) {
		case "from":
			email.From = h.Value
		case "subject":
			email.Subject = h.Value
		case "date":
			if t, err := time.Parse(time.RFC1123Z, h.Value); err == nil {
				email.Date = t
			}
		}
	}

	if email.Date.IsZero() && msg.InternalDate != 0 {
		email.Date = time.UnixMilli(msg.InternalDate)
	}

	email.Body = extractPlainBody(msg.Payload)

	raw, _ := json.Marshal(msg)
	email.Raw = raw

	return email, nil
}

func extractPlainBody(payload *googleapi.MessagePart) string {
	if payload.MimeType == "text/plain" && payload.Body != nil && payload.Body.Data != "" {
		data, err := base64.URLEncoding.DecodeString(payload.Body.Data)
		if err == nil {
			return string(data)
		}
	}
	for _, part := range payload.Parts {
		if body := extractPlainBody(part); body != "" {
			return body
		}
	}
	return ""
}

// cursor tracks the last poll timestamp per account.
type cursor struct {
	AfterEpoch int64 `json:"after_epoch"` // unix seconds
}

// Adapter polls one Gmail account for new messages.
type Adapter struct {
	account    string // label for this account (e.g. "personal", "work")
	client     GmailClient
	logger     *slog.Logger
	maxResults int64
}

// NewAdapter creates a Gmail adapter for the given account.
// Use NewAdapterWithClient for testing.
func NewAdapter(ctx context.Context, account string, oauthCfg *oauth2.Config, tokenStore *TokenStore, logger *slog.Logger) (*Adapter, error) {
	tok, err := tokenStore.Load(account)
	if err != nil {
		return nil, fmt.Errorf("load token for %s: %w", account, err)
	}
	if tok == nil {
		return nil, fmt.Errorf("no token for account %q — run auth first", account)
	}

	tokenSource := oauthCfg.TokenSource(ctx, tok)

	client, err := newLiveClient(ctx, tokenSource, "me")
	if err != nil {
		return nil, err
	}

	return &Adapter{
		account:    account,
		client:     client,
		logger:     logger.With("adapter", "gmail", "account", account),
		maxResults: 50,
	}, nil
}

// NewAdapterWithClient creates an adapter with an injected client (for testing).
func NewAdapterWithClient(account string, client GmailClient, logger *slog.Logger) *Adapter {
	return &Adapter{
		account:    account,
		client:     client,
		logger:     logger.With("adapter", "gmail", "account", account),
		maxResults: 50,
	}
}

func (a *Adapter) Name() string           { return "gmail:" + a.account }
func (a *Adapter) Source() silo.Source    { return silo.SourceGmail }
func (a *Adapter) Mode() silo.AdapterMode { return silo.ModePoll }

func (a *Adapter) Poll(ctx context.Context, since silo.Cursor) ([]silo.Message, silo.Cursor, error) {
	var cur cursor
	if since != nil {
		if err := json.Unmarshal(since, &cur); err != nil {
			a.logger.Warn("invalid cursor, starting fresh", "err", err)
		}
	}

	// If no cursor, default to last 24 hours
	if cur.AfterEpoch == 0 {
		cur.AfterEpoch = time.Now().Add(-24 * time.Hour).Unix()
	}

	query := fmt.Sprintf("after:%d", cur.AfterEpoch)
	a.logger.Debug("polling", "query", query)

	var allMsgs []silo.Message
	latestEpoch := cur.AfterEpoch
	pageToken := ""

	for {
		list, err := a.client.ListMessages(ctx, query, pageToken, a.maxResults)
		if err != nil {
			return nil, nil, fmt.Errorf("list messages: %w", err)
		}

		for _, ref := range list.Messages {
			email, err := a.client.GetMessage(ctx, ref.ID)
			if err != nil {
				a.logger.Warn("failed to get message", "id", ref.ID, "err", err)
				continue
			}

			preview := email.Snippet
			if len(preview) > 500 {
				preview = preview[:500]
			}

			msg := silo.Message{
				ID:         fmt.Sprintf("gmail:%s:%s", a.account, email.ID),
				Source:     silo.SourceGmail,
				Sender:     email.From,
				Subject:    email.Subject,
				Preview:    preview,
				Raw:        email.Raw,
				CapturedAt: time.Now(),
				SourceTS:   email.Date,
			}
			allMsgs = append(allMsgs, msg)

			if email.Date.Unix() > latestEpoch {
				latestEpoch = email.Date.Unix()
			}
		}

		if list.NextPageToken == "" {
			break
		}
		pageToken = list.NextPageToken
	}

	newCursor := cursor{AfterEpoch: latestEpoch}
	cursorBytes, _ := json.Marshal(newCursor)

	return allMsgs, silo.Cursor(cursorBytes), nil
}
