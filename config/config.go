package config

import (
    "encoding/json"
    "os"
)

type Config struct {
    SNIAddress       string `json:"sni_address"`
    LiveSplitAddress string `json:"livesplit_address"`
    GameDefinition   string `json:"game_definition"`
    EventFile        string `json:"event_file"`
}

func Load() *Config {
    // Default config
    cfg := &Config{
        SNIAddress:       "localhost:8080",
        LiveSplitAddress: "ws://localhost:16834",
        GameDefinition:   "",
        EventFile:        "",
    }

    // Try to load from disk
    if _, err := os.Stat("config.json"); err == nil {
        data, _ := os.ReadFile("config.json")
        json.Unmarshal(data, cfg)
    }

    return cfg
}

func Save(cfg *Config) {
    data, _ := json.MarshalIndent(cfg, "", "  ")
    os.WriteFile("config.json", data, 0644)
}
