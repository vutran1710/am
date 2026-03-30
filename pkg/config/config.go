package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// Config is the complete application config loaded from ~/.agent-mesh/config.toml.
type Config struct {
	Daemon      DaemonConfig `toml:"daemon"`
	Secrets     Secrets      `toml:"secrets"`
	Connections []Connection `toml:"connection"`
}

// DaemonConfig holds daemon runtime settings.
type DaemonConfig struct {
	Addr     string `toml:"addr"`
	LogLevel string `toml:"log_level"`
}

// Secrets holds sensitive credentials.
type Secrets struct {
	ComposioAPIKey string `toml:"composio_api_key"`
	ComposioMCPURL string `toml:"composio_mcp_url"`
	NangoSecretKey string `toml:"nango_secret_key"`
}

// Connection defines a service connection to poll.
type Connection struct {
	Provider     string       `toml:"provider"`
	Service      string       `toml:"service"`
	Label        string       `toml:"label"`
	ConnectionID string       `toml:"connection_id"`
	Interval     TomlDuration `toml:"interval"`
}

// TomlDuration wraps time.Duration for TOML string parsing.
type TomlDuration time.Duration

func (d *TomlDuration) UnmarshalText(text []byte) error {
	dur, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = TomlDuration(dur)
	return nil
}

func (d TomlDuration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

func (d TomlDuration) Duration() time.Duration {
	return time.Duration(d)
}

// ToDuration converts a time.Duration to a TomlDuration.
func ToDuration(d time.Duration) TomlDuration {
	return TomlDuration(d)
}

// DataDir returns the config directory path.
func DataDir() string {
	if v := os.Getenv("DATA_DIR"); v != "" {
		return v
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".agent-mesh")
	}
	return ".agent-mesh"
}

func configPath(dataDir string) string {
	return filepath.Join(dataDir, "config.toml")
}

// Load reads the full config from ~/.agent-mesh/config.toml.
// Missing file returns defaults. Env vars override daemon settings.
func Load(dataDir string) (*Config, error) {
	cfg := &Config{
		Daemon: DaemonConfig{
			Addr:     ":8080",
			LogLevel: "info",
		},
	}

	path := configPath(dataDir)
	if _, err := toml.DecodeFile(path, cfg); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Env var overrides
	if v := os.Getenv("DAEMON_ADDR"); v != "" {
		cfg.Daemon.Addr = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Daemon.LogLevel = v
	}

	return cfg, nil
}

// Save writes the full config to ~/.agent-mesh/config.toml (0600 for secrets).
func Save(dataDir string, cfg *Config) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(configPath(dataDir), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

// ComposioKey returns the Composio API key or an error.
func (c *Config) ComposioKey() (string, error) {
	if c.Secrets.ComposioAPIKey == "" {
		return "", fmt.Errorf("composio_api_key not set — run 'agent-mesh init'")
	}
	return c.Secrets.ComposioAPIKey, nil
}

// NangoKey returns the Nango secret key or an error.
func (c *Config) NangoKey() (string, error) {
	if c.Secrets.NangoSecretKey == "" {
		return "", fmt.Errorf("nango_secret_key not set — add it to config.toml [secrets]")
	}
	return c.Secrets.NangoSecretKey, nil
}

// FindConnection looks up a connection by service+label.
func (c *Config) FindConnection(service, label string) *Connection {
	for i := range c.Connections {
		if c.Connections[i].Service == service && c.Connections[i].Label == label {
			return &c.Connections[i]
		}
	}
	return nil
}

// AddConnection adds or replaces a connection.
func (c *Config) AddConnection(conn Connection) {
	for i := range c.Connections {
		if c.Connections[i].Service == conn.Service && c.Connections[i].Label == conn.Label {
			c.Connections[i] = conn
			return
		}
	}
	c.Connections = append(c.Connections, conn)
}
