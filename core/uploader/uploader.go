package uploader

import (
	"context"
	"io"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/entity"
	"github.com/gotd/td/telegram/message/styling"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/iyear/tdl/core/transfer"
	"github.com/samber/lo"

	"github.com/iyear/tdl/core/util/fsutil"
	"github.com/iyear/tdl/core/util/mediautil"
)

// MaxPartSize refer to https://core.telegram.org/api/files#uploading-files
const MaxPartSize = 512 * 1024

type Uploader struct {
	opts Options
}

type Options struct {
	Client   *tg.Client
	Threads  int
	Iter     Iter
	Progress Progress
}

func New(o Options) *Uploader {
	return &Uploader{opts: o}
}

func (u *Uploader) Upload(ctx context.Context, limit int) error {
	return transfer.Run(ctx, limit, u.opts.Iter.Next, func() Elem {
		elem := u.opts.Iter.Value()
		if q, ok := u.opts.Progress.(interface{ OnQueued(Elem) }); ok {
			q.OnQueued(elem)
		}
		return elem
	}, u.opts.Iter.Err,
		func(workCtx context.Context, elem Elem) error {
			u.opts.Progress.OnAdd(elem)
			err := u.upload(workCtx, elem)
			if finalizer, ok := u.opts.Progress.(Completion); ok {
				return finalizer.Finalize(elem, err)
			}
			u.opts.Progress.OnDone(elem, err)
			return err
		}, func() {
			if observer, ok := u.opts.Progress.(interface{ OnDiscoveryDone() }); ok {
				observer.OnDiscoveryDone()
			}
		})
}

func (u *Uploader) upload(ctx context.Context, elem Elem) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if prepared, ok := elem.(PreparedElem); ok {
		if err := prepared.PreparationError(); err != nil {
			return errors.Wrap(err, "prepare upload")
		}
	}

	up := uploader.NewUploader(u.opts.Client).
		WithPartSize(MaxPartSize).
		WithThreads(u.opts.Threads).
		WithProgress(&wrapProcess{
			elem:    elem,
			process: u.opts.Progress,
		})

	f, err := up.Upload(ctx, uploader.NewUpload(elem.File().Name(), elem.File(), elem.File().Size()))
	if err != nil {
		return errors.Wrap(err, "upload file")
	}

	if _, err = elem.File().Seek(0, io.SeekStart); err != nil {
		return errors.Wrap(err, "seek file")
	}
	mime, err := mimetype.DetectReader(elem.File())
	if err != nil {
		return errors.Wrap(err, "detect mime")
	}

	// here convert underlying entities to formatters for message caption
	caption := styling.Custom(func(eb *entity.Builder) error {
		msg, entities := elem.Caption()
		eb.Format(msg, lo.Map(entities, func(item tg.MessageEntityClass, _ int) entity.Formatter {
			return func(_, _ int) tg.MessageEntityClass {
				return item
			}
		})...)
		return nil
	})

	doc := message.UploadedDocument(f, caption).MIME(mime.String()).Filename(elem.File().Name())
	var uploadedThumb tg.InputFileClass
	if thumb, ok := elem.Thumb(); ok {
		thumbFile, thumbErr := uploadThumbnail(ctx, u.opts.Client, thumb)
		if thumbErr != nil {
			return errors.Wrap(thumbErr, "upload thumbnail")
		}
		uploadedThumb = thumbFile
		doc = doc.Thumb(thumbFile)
	}

	var media message.MediaOption = doc

	switch {
	case mediautil.IsImage(mime.String()) && elem.AsPhoto():
		// webp should be uploaded as document
		if mime.String() == "image/webp" {
			break
		}
		// upload as photo
		media = message.UploadedPhoto(f, caption)
	case mediautil.IsVideo(mime.String()):
		// reset reader
		if _, err = elem.File().Seek(0, io.SeekStart); err != nil {
			return errors.Wrap(err, "seek file")
		}
		if dur, w, h, err := mediautil.GetMP4Info(elem.File()); err == nil {
			// #132. There may be some errors, but we can still upload the file
			video := doc.Video().
				Duration(time.Duration(dur)*time.Second).
				Resolution(w, h).
				SupportsStreaming()
			media = video
			if coverElem, ok := elem.(CoverElem); ok {
				if cover, exists := coverElem.Cover(); exists {
					coverFile, coverErr := uploadThumbnail(ctx, u.opts.Client, cover)
					if coverErr != nil {
						return errors.Wrap(coverErr, "upload video cover")
					}
					coverPhoto, coverErr := uploadVideoCover(ctx, u.opts.Client, elem.To(), coverFile)
					if coverErr != nil {
						return errors.Wrap(coverErr, "prepare video cover")
					}
					media = message.Media(buildVideoDocument(f, uploadedThumb, coverPhoto, coverElem.CoverTimestamp(), mime.String(), elem.File().Name(), dur, w, h), caption)
				}
			}
		}
	case mediautil.IsAudio(mime.String()):
		media = doc.Audio().Title(fsutil.GetNameWithoutExt(elem.File().Name()))
	}

	_, err = message.NewSender(u.opts.Client).
		WithUploader(up).
		To(elem.To()).
		Reply(elem.Thread()).
		Media(ctx, media)
	if err != nil {
		return errors.Wrap(err, "send message")
	}

	return nil
}

func uploadVideoCover(ctx context.Context, client *tg.Client, peer tg.InputPeerClass, file tg.InputFileClass) (*tg.InputPhoto, error) {
	media, err := message.NewSender(client).To(peer).UploadMedia(ctx, message.UploadedPhoto(file))
	if err != nil {
		return nil, err
	}
	photoMedia, ok := media.(*tg.MessageMediaPhoto)
	if !ok {
		return nil, errors.Errorf("unexpected uploaded cover media %T", media)
	}
	photo, ok := photoMedia.Photo.(*tg.Photo)
	if !ok {
		return nil, errors.Errorf("unexpected uploaded cover photo %T", photoMedia.Photo)
	}
	return &tg.InputPhoto{ID: photo.ID, AccessHash: photo.AccessHash, FileReference: photo.FileReference}, nil
}

func buildVideoDocument(file, thumb tg.InputFileClass, cover *tg.InputPhoto, timestamp int, mime, name string, duration, width, height int) *tg.InputMediaUploadedDocument {
	doc := &tg.InputMediaUploadedDocument{
		File: file, Thumb: thumb, MimeType: mime, VideoCover: cover, VideoTimestamp: timestamp,
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeFilename{FileName: name},
			&tg.DocumentAttributeVideo{Duration: float64(duration), W: width, H: height, SupportsStreaming: true},
		},
	}
	return doc
}

// uploadThumbnail always supplies the known byte size. FromReader represents
// unknown-size streams as InputFileBig; Telegram may silently drop such a file
// when it is used as the thumbnail of another big upload.
func uploadThumbnail(ctx context.Context, client uploader.Client, thumb File) (tg.InputFileClass, error) {
	return uploader.NewUploader(client).Upload(
		ctx,
		uploader.NewUpload(thumb.Name(), thumb, thumb.Size()),
	)
}
