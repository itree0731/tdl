package tui

import (
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
