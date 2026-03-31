package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the application config loaded from ~/.agent-mesh/config.toml.
type Config struct {
	Server ServerConfig `toml:"server"`
}

// ServerConfig holds server settings.
type ServerConfig struct {
	Addr   string `toml:"addr"`
	APIKey string `toml:"api_key"` // private key for API access
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

// Load reads config from ~/.agent-mesh/config.toml.
func Load(dataDir string) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Addr: ":8090",
		},
	}

	path := configPath(dataDir)
	if _, err := toml.DecodeFile(path, cfg); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if v := os.Getenv("AM_ADDR"); v != "" {
		cfg.Server.Addr = v
	}

	// Auto-generate API key if missing
	if cfg.Server.APIKey == "" {
		cfg.Server.APIKey = generateKey()
		Save(dataDir, cfg)
	}

	return cfg, nil
}

// Save writes config to ~/.agent-mesh/config.toml.
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

func generateKey() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
