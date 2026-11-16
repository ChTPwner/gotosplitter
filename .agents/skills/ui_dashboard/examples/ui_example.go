package examples

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

type UIState struct {
	SNIStatus       string `json:"sni_status"`
	SNIAttached     bool   `json:"sni_attached"`
	LiveSplitStatus string `json:"livesplit_status"`
	GameDefinition  string `json:"game_definition"`
	CurrentSplit    string `json:"current_split"`
}

type ModelStore struct {
	state UIState
	mu    sync.RWMutex
}

func NewModelStore() *ModelStore {
	return &ModelStore{
		state: UIState{
			SNIStatus:       "Disconnected",
			LiveSplitStatus: "Waiting",
		},
	}
}

func (m *ModelStore) UpdateSNI(status string, attached bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.SNIStatus = status
	m.state.SNIAttached = attached
}

func (m *ModelStore) UpdateTracking(gameDefinition, split string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.GameDefinition = gameDefinition
	m.state.CurrentSplit = split
}

func (m *ModelStore) GetState() UIState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// SaveConfig persists current user settings to config.json.
func SaveConfig(path string, data any) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0644)
}

// RunHeadlessMode provides CLI status updates if display server is missing.
func RunHeadlessMode(store *ModelStore) {
	log.Println("[UI] Running in Headless/CLI mode.")
	state := store.GetState()
	log.Printf("[UI Status] SNI: %s | LiveSplit: %s | Target: %s",
		state.SNIStatus, state.LiveSplitStatus, state.CurrentSplit)
}
