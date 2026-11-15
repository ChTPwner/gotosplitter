package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultSNIAddress(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(sniAddressEnv, "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SNIAddress != "ws://127.0.0.1:23074" {
		t.Fatalf("SNIAddress = %q, want ws://127.0.0.1:23074", cfg.SNIAddress)
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
