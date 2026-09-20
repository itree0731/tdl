package dl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/gabriel-vasile/mimetype"
	"github.com/go-faster/errors"
	pw "github.com/jedib0t/go-pretty/v6/progress"

	"github.com/iyear/tdl/core/downloader"
	"github.com/iyear/tdl/core/util/fsutil"
	"github.com/iyear/tdl/pkg/prog"
	xprogress "github.com/iyear/tdl/pkg/progress"
	"github.com/iyear/tdl/pkg/utils"
)

type progress struct {
	ctx        context.Context
	pw         pw.Writer
	trackers   *sync.Map // map[ID]*pw.Tracker
	opts       Options
	tasksTotal int

	it *iter
}

func newProgress(ctx context.Context, p pw.Writer, it *iter, opts Options) *progress {
	return &progress{
		ctx:        ctx,
		pw:         p,
		trackers:   &sync.Map{},
		opts:       opts,
		tasksTotal: it.Total(),
		it:         it,
	}
}

func (p *progress) OnAdd(elem downloader.Elem) {
	e := elem.(*iterElem)
	tracker := prog.AppendTracker(p.pw, utils.Byte.FormatBinaryBytes, p.processMessage(elem), elem.File().Size())
	p.trackers.Store(e.id, tracker)
	xprogress.Emit(p.ctx, xprogress.Event{
		Kind:       xprogress.KindStarted,
		Direction:  xprogress.DirectionDownload,
		Status:     xprogress.StatusRunning,
		TaskID:     fmt.Sprintf("%d", e.id),
		TasksTotal: p.tasksTotal,
		FileName:   strings.TrimSuffix(e.to.Name(), tempExt),
		TotalBytes: elem.File().Size(),
		At:         time.Now(),
	})
}

func (p *progress) OnDownload(elem downloader.Elem, state downloader.ProgressState) {
	tracker, ok := p.trackers.Load(elem.(*iterElem).id)
	if !ok {
		return
	}

	t := tracker.(*pw.Tracker)
	t.UpdateTotal(state.Total)
	t.SetValue(state.Downloaded)
	e := elem.(*iterElem)
	xprogress.Emit(p.ctx, xprogress.Event{
		Kind:           xprogress.KindUpdated,
		Direction:      xprogress.DirectionDownload,
		Status:         xprogress.StatusRunning,
		TaskID:         fmt.Sprintf("%d", e.id),
		TasksTotal:     p.tasksTotal,
		FileName:       strings.TrimSuffix(e.to.Name(), tempExt),
		CompletedBytes: state.Downloaded,
		TotalBytes:     state.Total,
		At:             time.Now(),
	})
}

func (p *progress) OnDone(elem downloader.Elem, err error) {
	e := elem.(*iterElem)

	tracker, ok := p.trackers.Load(e.id)
	if !ok {
		return
	}
	t := tracker.(*pw.Tracker)

	if err := e.to.Close(); err != nil {
		p.fail(t, elem, errors.Wrap(err, "close file"))
		return
	}

	if err != nil {
		status := xprogress.StatusFailed
		if errors.Is(err, context.Canceled) {
			status = xprogress.StatusCanceled
		}
		xprogress.Emit(p.ctx, xprogress.Event{
			Kind:           xprogress.KindFinished,
			Direction:      xprogress.DirectionDownload,
			Status:         status,
			TaskID:         fmt.Sprintf("%d", e.id),
			TasksTotal:     p.tasksTotal,
			FileName:       strings.TrimSuffix(e.to.Name(), tempExt),
			CompletedBytes: e.file.Size,
			TotalBytes:     e.file.Size,
			At:             time.Now(),
			Err:            err.Error(),
		})
		if !errors.Is(err, context.Canceled) { // don't report user cancel
			p.fail(t, elem, errors.Wrap(err, "progress"))
		}
		_ = os.Remove(e.to.Name()) // just try to remove temp file, ignore error
		return
	}

	p.it.Finish(e.logicalPos)
	xprogress.Emit(p.ctx, xprogress.Event{
		Kind:           xprogress.KindFinished,
		Direction:      xprogress.DirectionDownload,
		Status:         xprogress.StatusDone,
		TaskID:         fmt.Sprintf("%d", e.id),
		TasksTotal:     p.tasksTotal,
		FileName:       strings.TrimSuffix(e.to.Name(), tempExt),
		CompletedBytes: e.file.Size,
		TotalBytes:     e.file.Size,
		At:             time.Now(),
	})

	if err := p.donePost(e); err != nil {
		p.fail(t, elem, errors.Wrap(err, "post file"))
		return
	}

	// small files may finish too fast for the progress renderer to draw
	// their tracker, so give it a moment (relocated from the core write
	// path, which delayed every file's last part)
	if e.file.Size < downloader.MaxPartSize {
		time.Sleep(200 * time.Millisecond)
	}
}

func (p *progress) donePost(elem *iterElem) error {
	newfile := strings.TrimSuffix(filepath.Base(elem.to.Name()), tempExt)

	if p.opts.RewriteExt {
		mime, err := mimetype.DetectFile(elem.to.Name())
		if err != nil {
			return errors.Wrap(err, "detect mime")
		}
		ext := mime.Extension()
		if ext != "" && (filepath.Ext(newfile) != ext) {
			newfile = fsutil.GetNameWithoutExt(newfile) + ext
		}
	}

	newpath := filepath.Join(filepath.Dir(elem.to.Name()), newfile)
	if err := os.Rename(elem.to.Name(), newpath); err != nil {
		return errors.Wrap(err, "rename file")
	}

	// Set file modification time to message date if available
	if elem.file.Date > 0 {
		fileTime := time.Unix(elem.file.Date, 0)
		if err := os.Chtimes(newpath, fileTime, fileTime); err != nil {
			return errors.Wrap(err, "set file time")
		}
	}

	return nil
}

func (p *progress) fail(t *pw.Tracker, elem downloader.Elem, err error) {
	p.pw.Log(color.RedString("%s error: %s", p.elemString(elem), err.Error()))
	t.MarkAsErrored()
}

func (p *progress) processMessage(elem downloader.Elem) string {
	return p.elemString(elem)
}

func (p *progress) elemString(elem downloader.Elem) string {
	e := elem.(*iterElem)
	return fmt.Sprintf("%s(%d):%d -> %s",
		e.from.VisibleName(),
		e.from.ID(),
		e.fromMsg.ID,
		strings.TrimSuffix(e.to.Name(), tempExt))
}
