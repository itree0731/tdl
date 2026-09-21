package up

import (
	"context"
	"errors"
	"github.com/iyear/tdl/core/uploader"
	xp "github.com/iyear/tdl/pkg/progress"
	"os"
	"path/filepath"
	"testing"
)

func TestUploadFinalizePreservesPartialBytesAndCleanupStage(t *testing.T) {
	for _, scenario := range []string{"success", "failed", "cancel", "close", "delete"} {
		t.Run(scenario, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "file")
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			e := &iterElem{file: &uploaderFile{File: f, size: 100}}
			c := xp.NewCollector()
			p := newProgress(xp.WithSink(context.Background(), c), 1)
			p.OnAdd(e)
			p.OnUpload(e, uploader.ProgressState{Uploaded: 10, Total: 100})
			var input error
			switch scenario {
			case "failed":
				input = errors.New("upload failed")
			case "cancel":
				input = context.Canceled
			case "close":
				f.Close()
			case "delete":
				e.remove = true
				f.Close()
				os.Remove(path)
				f, err = os.Open(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				e.file.File = f
				os.WriteFile(filepath.Join(f.Name(), "child"), []byte("x"), 0600)
			}
			err = p.Finalize(e, input)
			p.OnDiscoveryDone()
			s := c.Finish(err)
			if scenario == "success" {
				if err != nil || s.Succeeded != 1 {
					t.Fatalf("%v %+v", err, s)
				}
			} else {
				if err == nil || s.CompletedBytes != 10 || s.Failed+s.Canceled != 1 {
					t.Fatalf("%v %+v", err, s)
				}
			}
			if scenario == "delete" && s.Phase != "uploaded_cleanup" {
				t.Fatalf("phase=%s", s.Phase)
			}
		})
	}
}

func TestUploadFinalizeRemovesGeneratedThumbnail(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "video.mp4")
	thumbPath := filepath.Join(dir, "generated.jpg")
	media, err := os.Create(mediaPath)
	if err != nil {
		t.Fatal(err)
	}
	thumb, err := os.Create(thumbPath)
	if err != nil {
		t.Fatal(err)
	}
	e := &iterElem{
		file:           &uploaderFile{File: media, size: 100},
		thumb:          &uploaderFile{File: thumb},
		temporaryFiles: []string{thumbPath},
	}
	p := newProgress(context.Background(), 1)
	if err := p.Finalize(e, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(thumbPath); !os.IsNotExist(err) {
		t.Fatalf("temporary thumbnail still exists: %v", err)
	}
}
