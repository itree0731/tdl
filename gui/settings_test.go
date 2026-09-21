package main

import (
	"path/filepath"
	"testing"
)

func TestDesktopSettingsAtomicRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tmt.json")
	settings := defaultDesktopSettings()
	settings.Threads = 9
	settings.Proxy = "socks5://127.0.0.1:1080"
	if err := saveDesktopSettings(path, settings); err != nil {
		t.Fatal(err)
	}
	settings.Threads = 11
	if err := saveDesktopSettings(path, settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadDesktopSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Threads != 11 || loaded.Proxy != settings.Proxy {
		t.Fatalf("loaded=%+v", loaded)
	}
}

func TestValidateDesktopSettingsRejectsInvalidValues(t *testing.T) {
	settings := defaultDesktopSettings()
	settings.Limit = 0
	if err := validateDesktopSettings(settings); err == nil {
		t.Fatal("expected invalid limit error")
	}
	settings = defaultDesktopSettings()
	settings.Proxy = "not-a-proxy"
	if err := validateDesktopSettings(settings); err == nil {
		t.Fatal("expected invalid proxy error")
	}
}
