//go:build !windows

package tui

import (
	"io/fs"
	"path/filepath"
	"strings"
)

func isHiddenPath(path string, _ fs.FileInfo) bool {
	return strings.HasPrefix(filepath.Base(path), ".")
}
