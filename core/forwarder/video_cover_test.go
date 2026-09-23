package forwarder

import (
	"context"
	"testing"

	"github.com/gotd/td/tg"
	"github.com/iyear/tdl/core/tmedia"
)

type countProgress struct{ bytes int64 }

func (c *countProgress) add(n int64) { c.bytes += n }

func TestCloneReuploadsVideoEvenWhenUnprotected(t *testing.T) {
	video := &tg.MessageMediaDocument{Document: &tg.Document{MimeType: "video/mp4", Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeVideo{W: 640, H: 360}}}}
	photo := &tg.MessageMediaPhoto{}
	document := &tg.MessageMediaDocument{Document: &tg.Document{MimeType: "application/pdf"}}
	if !shouldReupload(video, false) || !shouldReupload(video, true) {
		t.Fatal("clone video reused the original Telegram media reference")
	}
	if shouldReupload(photo, false) || shouldReupload(document, false) {
		t.Fatal("non-video clone changed the original unprotected behavior")
	}
	if !shouldReupload(photo, true) || !shouldReupload(document, true) {
		t.Fatal("protected media stopped cloning")
	}
}

func TestCloneDryRunDoesNotDownloadOrGenerateCover(t *testing.T) {
	progress := &countProgress{}
	forwarder := New(Options{VideoCover: true})
	result, err := forwarder.cloneMedia(context.Background(), cloneOptions{
		media: &tmedia.Media{Size: 123}, progress: progress, cover: true,
	}, true)
	if err != nil || result.file == nil || result.cover != nil || progress.bytes != 246 {
		t.Fatalf("result=%+v progress=%d err=%v", result, progress.bytes, err)
	}
}
