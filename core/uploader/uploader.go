package uploader

import (
	"context"
	stderrors "errors"
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
	"github.com/iyear/tdl/core/videocover"
)

// MaxPartSize refer to https://core.telegram.org/api/files#uploading-files
const MaxPartSize = 512 * 1024

type Uploader struct {
	opts Options
}

type Options struct {
	Client     *tg.Client
	Threads    int
	Iter       Iter
	Progress   Progress
	VideoCover bool
	CoverAt    string
}

func New(o Options) *Uploader {
	return &Uploader{opts: o}
}

func (u *Uploader) Upload(ctx context.Context, limit int) error {
	return transfer.Run(ctx, limit, u.opts.Iter.Next, u.opts.Iter.Value, u.opts.Iter.Err,
		func(workCtx context.Context, elem Elem) error {
			u.opts.Progress.OnAdd(elem)
			err := u.upload(workCtx, elem)
			if finalizer, ok := u.opts.Progress.(Completion); ok {
				return finalizer.Finalize(elem, err)
			}
			u.opts.Progress.OnDone(elem, err)
			return err
		})
}

func (u *Uploader) upload(ctx context.Context, elem Elem) (rerr error) {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if _, err := elem.File().Seek(0, io.SeekStart); err != nil {
		return errors.Wrap(err, "seek file")
	}
	mime, err := mimetype.DetectReader(elem.File())
	if err != nil {
		return errors.Wrap(err, "detect mime")
	}
	if _, err = elem.File().Seek(0, io.SeekStart); err != nil {
		return errors.Wrap(err, "seek file")
	}
	var prepared videocover.Prepared
	var videoDuration time.Duration
	var videoWidth, videoHeight int
	if u.opts.VideoCover && mediautil.IsVideo(mime.String()) {
		pathFile, ok := elem.File().(interface{ Path() string })
		if !ok {
			return errors.New("video cover requires a local file path")
		}
		videoDuration, videoWidth, videoHeight, err = videocover.Probe(ctx, pathFile.Path())
		if err != nil {
			return errors.Wrap(err, "video cover metadata")
		}
		prepared, err = videocover.Prepare(ctx, pathFile.Path(), u.opts.CoverAt, videoDuration)
		if err != nil {
			return err
		}
		defer func() { rerr = stderrors.Join(rerr, prepared.Close()) }()
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
	// upload thumbnail TODO(iyear): maybe still unavailable
	if thumb, ok := elem.Thumb(); ok {
		if thumbFile, err := uploader.NewUploader(u.opts.Client).
			FromReader(ctx, thumb.Name(), thumb); err == nil {
			doc = doc.Thumb(thumbFile)
		}
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
		if prepared.Cover != "" {
			thumbFile, coverPhoto, coverErr := videocover.Upload(ctx, u.opts.Client, elem.To(), prepared)
			if coverErr != nil {
				return coverErr
			}
			media = message.Media(videocover.VideoDocument(f, thumbFile, coverPhoto, mime.String(), elem.File().Name(), int(videoDuration.Seconds()), videoWidth, videoHeight), caption)
			break
		}
		// reset reader
		if _, err = elem.File().Seek(0, io.SeekStart); err != nil {
			return errors.Wrap(err, "seek file")
		}
		if dur, w, h, err := mediautil.GetMP4Info(elem.File()); err == nil {
			// #132. There may be some errors, but we can still upload the file
			media = doc.Video().
				Duration(time.Duration(dur)*time.Second).
				Resolution(w, h).
				SupportsStreaming()
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
