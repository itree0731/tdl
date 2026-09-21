//go:build windows

package tui

import (
	"io/fs"
	"path/filepath"
	"syscall"
)

func isHiddenPath(path string, _ fs.FileInfo) bool {
	if len(filepath.Base(path)) > 1 && filepath.Base(path)[0] == '.' {
		return true
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attrs, err := syscall.GetFileAttributes(p)
	return err == nil && attrs&syscall.FILE_ATTRIBUTE_HIDDEN != 0
}
