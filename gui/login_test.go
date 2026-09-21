package main

import "testing"

func TestStartQRLoginRejectsUnsafeNamespaceBeforeOpeningStorage(t *testing.T) {
	app := NewApp()
	for _, value := range []string{"", "has space", `..\\account`, "账号"} {
		if err := app.StartQRLogin(value, ""); err == nil {
			t.Fatalf("namespace %q should be rejected", value)
		}
	}
}
