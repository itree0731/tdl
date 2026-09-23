package up

import (
	"context"
	"errors"
	"testing"

	"github.com/iyear/tdl/core/uploader"
)

func TestPrepareFailureDoesNotHideLaterUpload(t *testing.T) {
	failed := errors.New("second file missing")
	i := &iter{files: []*File{{File: "first.mp4"}, {File: "second.mp4"}, {File: "third.mp4"}}}
	var seen []string
	i.prepare = func(_ context.Context, file *File) (uploader.Elem, error) {
		seen = append(seen, file.File)
		if file.File == "second.mp4" {
			return nil, failed
		}
		return &iterElem{}, nil
	}
	completed := 0
	for i.Next(context.Background()) {
		completed++
	}
	if completed != 2 || len(seen) != 3 || seen[2] != "third.mp4" || !errors.Is(i.Err(), failed) {
		t.Fatalf("completed=%d seen=%v err=%v", completed, seen, i.Err())
	}
}
