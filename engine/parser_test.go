package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefinitionAcceptsGameFieldAsRootName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "definition.json")
	data := []byte(`{
		"game": "The Legend of Zelda: A Link to the Past",
		"autostart": {
			"address": "0x10",
			"value": "0x05",
			"type": "eq"
		},
		"definitions": [
			{"name": "Inside House", "address": "0x8A", "value": "0x2C", "type": "eq"}
		]
	}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	definition, err := LoadDefinition(path)
	if err != nil {
		t.Fatalf("LoadDefinition() error = %v", err)
	}
	if definition.Name != "The Legend of Zelda: A Link to the Past" {
		t.Fatalf("Name = %q, want game title from 'game' field", definition.Name)
	}
}
