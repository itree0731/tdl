package downloader

import (
	"context"

	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram/downloader"
	"github.com/iyear/tdl/core/transfer"
	"go.uber.org/zap"

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
	return transfer.Run(ctx, limit, d.opts.Iter.Next, func() Elem {
		elem := d.opts.Iter.Value()
		if q, ok := d.opts.Progress.(interface{ OnQueued(Elem) }); ok {
			q.OnQueued(elem)
		}
		return elem
	}, d.opts.Iter.Err,
		func(workCtx context.Context, elem Elem) error {
			d.opts.Progress.OnAdd(elem)
			err := d.download(workCtx, elem)
			if finalizer, ok := d.opts.Progress.(Completion); ok {
				return finalizer.Finalize(elem, err)
			}
			d.opts.Progress.OnDone(elem, err)
			return err
		}, func() {
			if observer, ok := d.opts.Progress.(interface{ OnDiscoveryDone() }); ok {
				observer.OnDiscoveryDone()
			}
		})
}

func (d *Downloader) download(ctx context.Context, elem Elem) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if prepared, ok := elem.(PreparedElem); ok && prepared.SkipTransfer() {
		return nil
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
