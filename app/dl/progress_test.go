package dl

import (
	"context"
	"errors"
	"github.com/iyear/tdl/core/downloader"
	"github.com/iyear/tdl/core/tmedia"
	xp "github.com/iyear/tdl/pkg/progress"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestFinalizeOnlyRegistersSuccessfulOutput(t *testing.T) {
	for _, scenario := range []string{"success", "transfer", "close", "rename"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "file.tmp")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			file.Write(make([]byte, 10))
			elem := &iterElem{to: file, file: &tmedia.Media{Size: 100}, logicalPos: 4}
			it := &iter{mu: &sync.Mutex{}, finished: map[int]struct{}{}}
			c := xp.NewCollector()
			p := newProgress(xp.WithSink(context.Background(), c), it, Options{})
			p.OnAdd(elem)
			p.OnDownload(elem, downloader.ProgressState{Downloaded: 10, Total: 100})
			var transferErr error
			switch scenario {
			case "transfer":
				transferErr = errors.New("transfer failed")
			case "close":
				file.Close()
			case "rename":
				os.Mkdir(filepath.Join(dir, "file"), 0700)
			}
			err = p.Finalize(elem, transferErr)
			p.OnDiscoveryDone()
			s := c.Finish(err)
			if scenario == "success" {
				if err != nil || s.Succeeded != 1 || len(it.Finished()) != 1 {
					t.Fatalf("%v %+v", err, s)
				}
			} else {
				if err == nil || s.Failed != 1 || s.CompletedBytes != 10 || len(it.Finished()) != 0 {
					t.Fatalf("%v %+v finished=%v", err, s, it.Finished())
				}
			}
		})
	}
}
