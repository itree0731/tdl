package uploader

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestBuildVideoDocumentIncludesDistinctCoverAndThumbnail(t *testing.T) {
	file := &tg.InputFile{ID: 1}
	thumb := &tg.InputFile{ID: 2}
	cover := &tg.InputPhoto{ID: 3, AccessHash: 4, FileReference: []byte{5}}
	doc := buildVideoDocument(file, thumb, cover, "video/mp4", "clip.mp4", 42, 720, 1280)
	if doc.File != file || doc.Thumb != thumb || doc.VideoCover != cover {
		t.Fatalf("video media fields = %+v", doc)
	}
	if doc.VideoTimestamp != 0 {
		t.Fatalf("video timestamp=%d; playback must begin at 0:00", doc.VideoTimestamp)
	}
	if len(doc.Attributes) != 2 {
		t.Fatalf("attributes = %+v", doc.Attributes)
	}
	video, ok := doc.Attributes[1].(*tg.DocumentAttributeVideo)
	if !ok || !video.SupportsStreaming || video.W != 720 || video.H != 1280 || video.Duration != 42 {
		t.Fatalf("video attribute = %+v", doc.Attributes[1])
	}
}
