package gmail

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googleapi "google.golang.org/api/gmail/v1"
)

var gmailScopes = []string{
	googleapi.GmailReadonlyScope,
}

// TokenStore handles reading/writing OAuth2 tokens from disk.
type TokenStore struct {
	dir string // directory where tokens are stored
}

// NewTokenStore creates a token store rooted at the given directory.
func NewTokenStore(dir string) *TokenStore {
	return &TokenStore{dir: dir}
}

func (ts *TokenStore) tokenPath(account string) string {
	return filepath.Join(ts.dir, fmt.Sprintf("gmail_%s_token.json", account))
}

// Load reads a saved token for the given account. Returns nil if not found.
func (ts *TokenStore) Load(account string) (*oauth2.Token, error) {
	data, err := os.ReadFile(ts.tokenPath(account))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read token: %w", err)
	}
	var tok oauth2.Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}
	return &tok, nil
}

// Save persists a token for the given account.
func (ts *TokenStore) Save(account string, tok *oauth2.Token) error {
	if err := os.MkdirAll(ts.dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ts.tokenPath(account), data, 0o600)
}

// OAuthConfig loads the OAuth2 config from a credentials JSON file.
func OAuthConfig(credentialsFile string) (*oauth2.Config, error) {
	data, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	cfg, err := google.ConfigFromJSON(data, gmailScopes...)
	if err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	return cfg, nil
}

// AuthorizeInteractive runs the OAuth2 authorization code flow.
// It starts a local server to receive the callback.
func AuthorizeInteractive(ctx context.Context, cfg *oauth2.Config) (*oauth2.Token, error) {
	cfg.RedirectURL = "http://localhost:19876/callback"

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	srv := &http.Server{Addr: ":19876", Handler: mux}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code in callback")
			fmt.Fprint(w, "Error: no code received")
			return
		}
		codeCh <- code
		fmt.Fprint(w, "Authorization successful! You can close this tab.")
	})

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	defer srv.Shutdown(ctx)

	authURL := cfg.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("\nOpen this URL in your browser to authorize:\n\n%s\n\nWaiting for authorization...\n", authURL)

	select {
	case code := <-codeCh:
		return cfg.Exchange(ctx, code)
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
