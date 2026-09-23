package videocover

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
)

// Upload sends the generated JPEGs as temporary Telegram files and converts
// the high-resolution cover into an InputPhoto for VideoCover.
func Upload(ctx context.Context, client *tg.Client, peer tg.InputPeerClass, prepared Prepared) (tg.InputFileClass, *tg.InputPhoto, error) {
	thumb, err := uploadJPEG(ctx, client, prepared.Thumb)
	if err != nil {
		return nil, nil, fmt.Errorf("upload video thumbnail: %w", err)
	}
	coverFile, err := uploadJPEG(ctx, client, prepared.Cover)
	if err != nil {
		return nil, nil, fmt.Errorf("upload video cover file: %w", err)
	}
	media, err := client.MessagesUploadMedia(ctx, &tg.MessagesUploadMediaRequest{
		Peer:  peer,
		Media: &tg.InputMediaUploadedPhoto{File: coverFile},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("register video cover: %w", err)
	}
	photoMedia, ok := media.(*tg.MessageMediaPhoto)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected cover media %T", media)
	}
	photo, ok := photoMedia.Photo.(*tg.Photo)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected cover photo %T", photoMedia.Photo)
	}
	return thumb, &tg.InputPhoto{ID: photo.ID, AccessHash: photo.AccessHash, FileReference: photo.FileReference}, nil
}

func uploadJPEG(ctx context.Context, client *tg.Client, path string) (tg.InputFileClass, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	return uploader.NewUploader(client).Upload(ctx, uploader.NewUpload(filepath.Base(path), file, stat.Size()))
}

// VideoDocument intentionally leaves VideoTimestamp unset. Telegram clients
// can interpret that field as the playback start rather than the cover frame.
func VideoDocument(file, thumb tg.InputFileClass, cover *tg.InputPhoto, mime, name string, duration, width, height int) *tg.InputMediaUploadedDocument {
	return &tg.InputMediaUploadedDocument{
		File: file, Thumb: thumb, VideoCover: cover, MimeType: mime,
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeFilename{FileName: name},
			&tg.DocumentAttributeVideo{Duration: float64(duration), W: width, H: height, SupportsStreaming: true},
		},
	}
}
