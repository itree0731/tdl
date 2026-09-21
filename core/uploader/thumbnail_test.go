package uploader

import (
	"bytes"
	"context"
	"testing"

	"github.com/gotd/td/tg"
)

type thumbnailUploadClient struct {
	smallParts int
	bigParts   int
}

func (c *thumbnailUploadClient) UploadSaveFilePart(_ context.Context, _ *tg.UploadSaveFilePartRequest) (bool, error) {
	c.smallParts++
	return true, nil
}

func (c *thumbnailUploadClient) UploadSaveBigFilePart(_ context.Context, _ *tg.UploadSaveBigFilePartRequest) (bool, error) {
	c.bigParts++
	return true, nil
}

func TestThumbnailUsesSmallInputFileWithKnownSize(t *testing.T) {
	data := []byte("jpeg thumbnail bytes")
	file := preparationTestFile{Reader: bytes.NewReader(data)}
	client := &thumbnailUploadClient{}

	uploaded, err := uploadThumbnail(context.Background(), client, file)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := uploaded.(*tg.InputFile); !ok {
		t.Fatalf("thumbnail type=%T, want *tg.InputFile", uploaded)
	}
	if client.smallParts == 0 || client.bigParts != 0 {
		t.Fatalf("smallParts=%d bigParts=%d", client.smallParts, client.bigParts)
	}
}
