package dl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenDownloadTargetBacksUpPartialBeforeRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "video.mp4.tmp")
	if err := os.WriteFile(path, []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	file, skip, backup, err := openDownloadTarget(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if skip || backup == "" || !strings.HasSuffix(backup, ".partial.bak") {
		t.Fatalf("skip=%v backup=%q", skip, backup)
	}
	backedUp, err := os.ReadFile(backup)
	if err != nil || string(backedUp) != "partial" {
		t.Fatalf("backup content=%q err=%v", backedUp, err)
	}
	if stat, err := file.Stat(); err != nil || stat.Size() != 0 {
		t.Fatalf("new temp stat=%v err=%v", stat, err)
	}
}

func TestOpenDownloadTargetKeepsCompleteTempForFinalization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "video.mp4.tmp")
	content := []byte("complete")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	file, skip, backup, err := openDownloadTarget(path, int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if !skip || backup != "" {
		t.Fatalf("skip=%v backup=%q", skip, backup)
	}
	read, err := os.ReadFile(path)
	if err != nil || string(read) != string(content) {
		t.Fatalf("complete temp changed: %q err=%v", read, err)
	}
}
