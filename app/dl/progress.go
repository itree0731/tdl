package dl

import (
	"context"
	"errors"
	"github.com/gabriel-vasile/mimetype"
	xerrors "github.com/go-faster/errors"
	"github.com/iyear/tdl/core/downloader"
	"github.com/iyear/tdl/core/util/fsutil"
	xprogress "github.com/iyear/tdl/pkg/progress"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type progress struct {
	source *xprogress.Source
	opts   Options
	it     *iter
}

func newProgress(ctx context.Context, it *iter, opts Options) *progress {
	return &progress{source: xprogress.NewSource(ctx, xprogress.DirectionDownload), it: it, opts: opts}
}
func (p *progress) OnQueued(elem downloader.Elem) {
	e := elem.(*iterElem)
	p.source.QueuePath(elem, strings.TrimSuffix(e.to.Name(), tempExt), e.to.Name(), elem.File().Size(), 0)
}
func (p *progress) OnAdd(elem downloader.Elem) { p.OnQueued(elem); p.source.Start(elem) }
func (p *progress) OnDownload(elem downloader.Elem, state downloader.ProgressState) {
	p.source.Update(elem, state.Downloaded, state.Total)
}
func (p *progress) OnDiscoveryDone()                       { p.source.DiscoveryDone() }
func (p *progress) OnDone(elem downloader.Elem, err error) { _ = p.Finalize(elem, err) }
func (p *progress) Finalize(elem downloader.Elem, err error) error {
	e := elem.(*iterElem)
	phase := "transferring"
	if e.skipTransfer {
		phase = "finalizing_existing"
	}
	closeErr := e.to.Close()
	if closeErr != nil {
		err = errors.Join(err, xerrors.Wrap(closeErr, "close file"))
		phase = "closing"
	}
	if err == nil {
		phase = "finalizing"
		err = p.donePost(e)
		if err == nil {
			p.it.Finish(e.logicalPos)
			phase = "done"
		}
	}
	// Keep failed partial files for inspection. Byte-level resume is not claimed.
	p.source.Finish(elem, phase, err)
	return err
}
func (p *progress) donePost(elem *iterElem) error {
	newfile := strings.TrimSuffix(filepath.Base(elem.to.Name()), tempExt)

	if p.opts.RewriteExt {
		mime, err := mimetype.DetectFile(elem.to.Name())
		if err != nil {
			return xerrors.Wrap(err, "detect mime")
		}
		ext := mime.Extension()
		if ext != "" && (filepath.Ext(newfile) != ext) {
			newfile = fsutil.GetNameWithoutExt(newfile) + ext
		}
	}

	newpath := filepath.Join(filepath.Dir(elem.to.Name()), newfile)
	if err := os.Rename(elem.to.Name(), newpath); err != nil {
		return xerrors.Wrap(err, "rename file")
	}

	// Set file modification time to message date if available
	if elem.file.Date > 0 {
		fileTime := time.Unix(elem.file.Date, 0)
		if err := os.Chtimes(newpath, fileTime, fileTime); err != nil {
			return xerrors.Wrap(err, "set file time")
		}
	}

	return nil
}
