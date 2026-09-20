package cmd

import (
	"context"
	"testing"

	"github.com/iyear/tdl/pkg/kv"
)

func TestNestedCommandReusesStorage(t *testing.T) {
	stg, err := kv.New(kv.DriverBolt, map[string]any{"path": t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stg.Close() }()

	root := New()
	root.SetContext(kv.WithOwned(context.Background(), stg))
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := stg.Open("still-open"); err != nil {
		t.Fatalf("outer storage was closed by nested command: %v", err)
	}
}
