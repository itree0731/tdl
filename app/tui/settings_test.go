package tui

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/iyear/tdl/pkg/consts"
)

// isolateSettings keeps model tests independent of the user's configuration and
// of previous tests. These tests must remain serial because DataDir is global.
func isolateSettings(t *testing.T) {
	t.Helper()
	original := consts.DataDir
	consts.DataDir = t.TempDir()
	t.Cleanup(func() { consts.DataDir = original })
}

func TestSettingsV1MigratesToV2(t *testing.T) {
	isolateSettings(t)
	old := []byte(`{"language":"zh","ns":"work","limit":"3"}`)
	if err := os.WriteFile(settingsPath(), old, 0600); err != nil {
		t.Fatal(err)
	}
	s := loadSettings()
	if s.SchemaVersion != settingsSchemaVersion || s.RecentDirs == nil || s.RecentChats == nil || s.NS != "work" {
		t.Fatalf("migration = %+v", s)
	}
	if err := saveSettings(s); err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	b, err := os.ReadFile(settingsPath())
	if err != nil || json.Unmarshal(b, &raw) != nil {
		t.Fatalf("saved settings unreadable: %v", err)
	}
	if raw["schema_version"] != float64(settingsSchemaVersion) {
		t.Fatalf("schema version not persisted: %v", raw)
	}
}
