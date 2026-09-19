package downloader

import (
	"context"

	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram/downloader"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/iyear/tdl/core/dcpool"
	"github.com/iyear/tdl/core/logctx"
	"github.com/iyear/tdl/core/util/tutil"
)

// MaxPartSize refer to https://core.telegram.org/api/files#downloading-files
const MaxPartSize = 1024 * 1024

type Downloader struct {
	opts Options
}

type Options struct {
	Pool     dcpool.Pool
	Threads  int
	Iter     Iter
	Progress Progress
}

func New(opts Options) *Downloader {
	return &Downloader{
		opts: opts,
	}
}

func (d *Downloader) Download(ctx context.Context, limit int) error {
	wg, wgctx := errgroup.WithContext(ctx)
	wg.SetLimit(limit)

	for d.opts.Iter.Next(wgctx) {
		elem := d.opts.Iter.Value()

		wg.Go(func() (rerr error) {
			d.opts.Progress.OnAdd(elem)

			var derr error
			defer func() { d.opts.Progress.OnDone(elem, derr) }()

			derr = d.download(wgctx, elem)
			if derr != nil {
				// canceled by user, so we directly return error to stop all
				if errors.Is(derr, context.Canceled) {
					return errors.Wrap(derr, "download")
				}

				// don't fail the whole group, just log it,
				// but the error must reach OnDone so progress
				// doesn't finalize a partial file as complete
				logctx.
					From(ctx).
					Error("Download error",
						zap.Any("element", elem),
						zap.Error(derr),
					)
			}

			return nil
		})
	}

	if err := d.opts.Iter.Err(); err != nil {
		wg.Wait() // let in-flight goroutines settle before the caller tears down connections
		return errors.Wrap(err, "iter")
	}

	return wg.Wait()
}

func (d *Downloader) download(ctx context.Context, elem Elem) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	logctx.From(ctx).Debug("Start download elem",
		zap.Any("elem", elem))

	client := d.opts.Pool.Client(ctx, elem.File().DC())
	if elem.AsTakeout() {
		client = d.opts.Pool.Takeout(ctx, elem.File().DC())
	}

	_, err := downloader.NewDownloader().WithPartSize(MaxPartSize).
		Download(client, elem.File().Location()).
		WithThreads(tutil.BestThreads(elem.File().Size(), d.opts.Threads)).
		Parallel(ctx, newWriteAt(elem, d.opts.Progress))
	if err != nil {
		return errors.Wrap(err, "download")
	}

	return nil
}
