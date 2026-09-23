package up

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadCleanupErrorReportsAlreadyUploaded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uploaded.mp4")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	originalRemove := removeUploadedSource
	removeUploadedSource = func(string) error { return fmt.Errorf("test permission denied") }
	t.Cleanup(func() { removeUploadedSource = originalRemove })
	err = finalizeUploadFile(&iterElem{file: &uploaderFile{File: file}, remove: true}, nil)
	if err == nil || !strings.Contains(err.Error(), "uploaded successfully; source deletion failed") {
		t.Fatalf("cleanup error = %v", err)
	}
}

func TestFailedUploadKeepsSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-uploaded.mp4")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	transferErr := errors.New("network failed")
	err = finalizeUploadFile(&iterElem{file: &uploaderFile{File: file}, remove: true}, transferErr)
	if !errors.Is(err, transferErr) {
		t.Fatalf("error = %v", err)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatalf("source removed after upload failure: %v", err)
	}
}
