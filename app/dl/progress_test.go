package dl

import (
	"errors"
	"github.com/iyear/tdl/core/tmedia"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadFinalizationMarksOnlyLandedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.mp4"+tempExt)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	elem := &iterElem{to: file, file: &tmedia.Media{}}
	marked := false
	p := &progress{}
	if err = finalizeDownloadFile(elem, nil, p.donePost, func() { marked = true }); err != nil || !marked {
		t.Fatalf("finalize err=%v marked=%v", err, marked)
	}
	if _, err = os.Stat(filepath.Join(dir, "clip.mp4")); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadFailureRetainsPartialFileWithoutCheckpoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.mp4"+tempExt)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write([]byte("partial")); err != nil {
		t.Fatal(err)
	}
	marked := false
	networkErr := errors.New("network failed at ten percent")
	p := &progress{}
	err = finalizeDownloadFile(&iterElem{to: file, file: &tmedia.Media{}}, networkErr, p.donePost, func() { marked = true })
	if !errors.Is(err, networkErr) || marked {
		t.Fatalf("err=%v marked=%v", err, marked)
	}
	if data, readErr := os.ReadFile(path); readErr != nil || string(data) != "partial" {
		t.Fatalf("partial=%q err=%v", data, readErr)
	}
}

func TestDownloadRenameFailureNeverMarksDone(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "clip.mp4"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "clip.mp4"+tempExt)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	marked := false
	p := &progress{}
	err = finalizeDownloadFile(&iterElem{to: file, file: &tmedia.Media{}}, nil, p.donePost, func() { marked = true })
	if err == nil || marked {
		t.Fatalf("err=%v marked=%v", err, marked)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatalf("temporary file removed: %v", err)
	}
}
