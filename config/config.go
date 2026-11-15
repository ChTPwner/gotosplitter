package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultSNIAddress = "ws://127.0.0.1:23074"
	sniAddressEnv     = "GOTOSPLITTER_SNI_ADDRESS"
)

type Config struct {
	SNIAddress       string `json:"sni_address"`
	LiveSplitAddress string `json:"livesplit_address"`
	GameDefinition   string `json:"game_definition"`
	EventFile        string `json:"event_file"`
}

func Load() (*Config, error) {
	cfg := &Config{
		SNIAddress:       defaultSNIAddress,
		LiveSplitAddress: "ws://localhost:16834",
	}

	path, err := configFilePath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return cfg, fmt.Errorf("read config file %s: %w", path, err)
	}
	if err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return cfg, fmt.Errorf("parse config file %s: %w", path, err)
		}
	}

	if address := strings.TrimSpace(os.Getenv(sniAddressEnv)); address != "" {
		cfg.SNIAddress = address
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("cannot save a nil config")
	}

	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config file %s: %w", path, err)
	}
	return nil
}

func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".config", "gotosplitter", "config.json"), nil
}
