package videocover

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestVideoDocumentHasCoverAndStartsAtZero(t *testing.T) {
	file := &tg.InputFile{}
	thumb := &tg.InputFile{}
	cover := &tg.InputPhoto{ID: 3}
	media := VideoDocument(file, thumb, cover, "video/mp4", "clip.mp4", 30, 1920, 1080)
	if media.File != file || media.Thumb != thumb || media.VideoCover != cover || media.VideoTimestamp != 0 {
		t.Fatalf("video media = %+v", media)
	}
	media.SetFlags()
	if !media.Flags.Has(6) || !media.Flags.Has(2) || media.Flags.Has(7) {
		t.Fatalf("video cover, thumbnail, or playback flags are wrong: %v", media.Flags)
	}
	video, ok := media.Attributes[1].(*tg.DocumentAttributeVideo)
	if !ok || video.W != 1920 || video.H != 1080 || !video.SupportsStreaming {
		t.Fatalf("video attribute = %+v", media.Attributes)
	}
}
