// Package config manages the configuration file for GoToSplitter, including loading, saving, and migrating legacy configurations.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultSNIAddress = "ws://127.0.0.1:23074"
	sniAddressEnv     = "GOTOSPLITTER_SNI_ADDRESS"
)

type Config struct {
	SNIAddress            string `json:"sni_address"`
	LiveSplitAddress      string `json:"livesplit_address"`
	GameDefinitionFolder  string `json:"game_definition_folder"`
	CurrentGameDefinition string `json:"current_game_definition"`
	EventFile             string `json:"event_file"`
}

func Load() (*Config, error) {
	cfg := &Config{
		SNIAddress:       defaultSNIAddress,
		LiveSplitAddress: "ws://localhost:16834",
	}
	defaultFolder, err := DefaultGameDefinitionFolder()
	if err != nil {
		return cfg, err
	}
	cfg.GameDefinitionFolder = defaultFolder

	path, err := configFilePath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return cfg, fmt.Errorf("read config file %s: %w", path, err)
	}
	if err == nil {
		var legacy struct {
			GameDefinition        string  `json:"game_definition"`
			GameDefinitionFolder  *string `json:"game_definition_folder"`
			CurrentGameDefinition *string `json:"current_game_definition"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return cfg, fmt.Errorf("parse config file %s: %w", path, err)
		}
		if err := json.Unmarshal(data, cfg); err != nil {
			return cfg, fmt.Errorf("parse config file %s: %w", path, err)
		}
		if cfg.GameDefinitionFolder == "" {
			cfg.GameDefinitionFolder = defaultFolder
		}
		if legacy.GameDefinition != "" && legacy.GameDefinitionFolder == nil && legacy.CurrentGameDefinition == nil {
			folder := filepath.Dir(legacy.GameDefinition)
			if folder == "." {
				folder = defaultFolder
			}
			cfg.GameDefinitionFolder = folder
			cfg.CurrentGameDefinition = filepath.Base(legacy.GameDefinition)
			if err := Save(cfg); err != nil {
				return cfg, fmt.Errorf("migrate game definition config: %w", err)
			}
		}
	} else if err := Save(cfg); err != nil {
		return cfg, fmt.Errorf("create default config file: %w", err)
	}

	if address := strings.TrimSpace(os.Getenv(sniAddressEnv)); address != "" {
		cfg.SNIAddress = address
	}

	return cfg, nil
}

// DefaultGameDefinitionFolder returns the directory containing config.json.
func DefaultGameDefinitionFolder() (string, error) {
	path, err := configFilePath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

// ListGameDefinitions returns JSON definition filenames in alphabetical order.
func ListGameDefinitions(folder string) ([]string, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, fmt.Errorf("read game definition folder %s: %w", folder, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// ResolveGameDefinition returns the selected definition path and filename.
// An empty filename resolves to the first alphabetically sorted JSON file.
func ResolveGameDefinition(folder, filename string) (string, string, error) {
	if folder == "" {
		defaultFolder, err := DefaultGameDefinitionFolder()
		if err != nil {
			return "", "", err
		}
		folder = defaultFolder
	}
	if filename == "" {
		names, err := ListGameDefinitions(folder)
		if err != nil {
			return "", "", err
		}
		if len(names) == 0 {
			return "", "", fmt.Errorf("no JSON game definitions found in %s", folder)
		}
		filename = names[0]
	}
	if filepath.Base(filename) != filename {
		return "", "", fmt.Errorf("game definition must be a filename inside %s", folder)
	}
	return filepath.Join(folder, filename), filename, nil
}

func Save(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("cannot save a nil config")
	}

	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
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
