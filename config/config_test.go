package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultSNIAddress(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(sniAddressEnv, "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SNIAddress != "ws://127.0.0.1:23074" {
		t.Fatalf("SNIAddress = %q, want ws://127.0.0.1:23074", cfg.SNIAddress)
	}
	if cfg.ResetSNESOnTimerReset {
		t.Fatal("ResetSNESOnTimerReset = true, want false by default")
	}
	wantFolder := filepath.Join(home, ".config", "gotosplitter")
	if cfg.GameDefinitionFolder != wantFolder {
		t.Fatalf("GameDefinitionFolder = %q, want %q", cfg.GameDefinitionFolder, wantFolder)
	}

	configPath := filepath.Join(home, ".config", "gotosplitter", "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("default config file was not created: %v", err)
	}
}

func TestLoadCreatesDefaultConfigBeforeEnvironmentOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(sniAddressEnv, "ws://env-host:23074")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SNIAddress != "ws://env-host:23074" {
		t.Fatalf("SNIAddress = %q, want environment value", cfg.SNIAddress)
	}

	configPath := filepath.Join(home, ".config", "gotosplitter", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read default config file: %v", err)
	}
	if string(data) == "" {
		t.Fatal("default config file is empty")
	}
	var persisted Config
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("parse default config file: %v", err)
	}
	if persisted.SNIAddress != "ws://127.0.0.1:23074" {
		t.Fatalf("persisted SNIAddress = %q, want default address", persisted.SNIAddress)
	}
}

func TestLoadConfigAndEnvironmentOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".config", "gotosplitter", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"sni_address":"ws://config-host:23074"}`), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv(sniAddressEnv, "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SNIAddress != "ws://config-host:23074" {
		t.Fatalf("SNIAddress = %q, want config file value", cfg.SNIAddress)
	}

	t.Setenv(sniAddressEnv, "ws://env-host:23074")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() with environment override error = %v", err)
	}
	if cfg.SNIAddress != "ws://env-host:23074" {
		t.Fatalf("SNIAddress = %q, want environment value", cfg.SNIAddress)
	}
}

func TestSaveWritesConfigUnderHomeConfigDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(sniAddressEnv, "")

	want := &Config{
		SNIAddress:       "ws://custom-host:23074",
		LiveSplitAddress: "ws://localhost:16834",
	}
	if err := Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() after Save() error = %v", err)
	}
	if got.SNIAddress != want.SNIAddress {
		t.Fatalf("SNIAddress = %q, want %q", got.SNIAddress, want.SNIAddress)
	}
}

func TestLoadResetSNESPreferenceDefaultsFalseAndPersists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(sniAddressEnv, "")

	configPath := filepath.Join(home, ".config", "gotosplitter", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"sni_address":"ws://config-host:23074"}`), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ResetSNESOnTimerReset {
		t.Fatal("missing reset preference = true, want false")
	}

	cfg.ResetSNESOnTimerReset = true
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() preference error = %v", err)
	}
	reloaded, err := Load()
	if err != nil {
		t.Fatalf("Load() after preference save error = %v", err)
	}
	if !reloaded.ResetSNESOnTimerReset {
		t.Fatal("reloaded reset preference = false, want true")
	}
}

func TestLoadMigratesLegacyGameDefinitionPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(sniAddressEnv, "")

	configPath := filepath.Join(home, ".config", "gotosplitter", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(home, "definitions", "game.json")
	if err := os.WriteFile(configPath, []byte(`{"game_definition":"`+legacyPath+`"}`), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.GameDefinitionFolder != filepath.Dir(legacyPath) {
		t.Fatalf("GameDefinitionFolder = %q, want %q", cfg.GameDefinitionFolder, filepath.Dir(legacyPath))
	}
	if cfg.CurrentGameDefinition != "game.json" {
		t.Fatalf("CurrentGameDefinition = %q, want game.json", cfg.CurrentGameDefinition)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("parse migrated config: %v", err)
	}
	if persisted["game_definition_folder"] != filepath.Dir(legacyPath) || persisted["current_game_definition"] != "game.json" {
		t.Fatalf("migrated config has unexpected definition settings: %v", persisted)
	}
	if _, ok := persisted["game_definition"]; ok {
		t.Fatal("migrated config still contains legacy game_definition")
	}
}

func TestLoadPrefersNewDefinitionSettingsOverLegacyPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, ".config", "gotosplitter", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	data := `{"game_definition":"/legacy/old.json","game_definition_folder":"/new/folder","current_game_definition":""}`
	if err := os.WriteFile(configPath, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.GameDefinitionFolder != "/new/folder" || cfg.CurrentGameDefinition != "" {
		t.Fatalf("definition settings = (%q, %q), want (/new/folder, empty)", cfg.GameDefinitionFolder, cfg.CurrentGameDefinition)
	}
}

func TestResolveGameDefinitionUsesFirstAlphabeticalJSONFile(t *testing.T) {
	folder := t.TempDir()
	for _, name := range []string{"zeta.json", "Alpha.json", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(folder, name), []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(folder, "nested.json"), 0700); err != nil {
		t.Fatal(err)
	}

	path, filename, err := ResolveGameDefinition(folder, "")
	if err != nil {
		t.Fatalf("ResolveGameDefinition() error = %v", err)
	}
	if filename != "Alpha.json" {
		t.Fatalf("filename = %q, want Alpha.json", filename)
	}
	if path != filepath.Join(folder, "Alpha.json") {
		t.Fatalf("path = %q, want %q", path, filepath.Join(folder, "Alpha.json"))
	}
}

func TestResolveGameDefinitionRejectsPathsOutsideFolder(t *testing.T) {
	if _, _, err := ResolveGameDefinition(t.TempDir(), "../outside.json"); err == nil {
		t.Fatal("ResolveGameDefinition() accepted a path instead of a filename")
	}
}
