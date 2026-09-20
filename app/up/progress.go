package up

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/go-faster/errors"
	pw "github.com/jedib0t/go-pretty/v6/progress"

	"github.com/iyear/tdl/core/uploader"
	"github.com/iyear/tdl/pkg/prog"
	xprogress "github.com/iyear/tdl/pkg/progress"
	"github.com/iyear/tdl/pkg/utils"
)

type progress struct {
	ctx        context.Context
	pw         pw.Writer
	trackers   *sync.Map // map[tuple]*pw.Tracker
	tasksTotal int
}

type tuple struct {
	name string
	to   int64
}

func newProgress(ctx context.Context, p pw.Writer, tasksTotal int) *progress {
	return &progress{
		ctx:        ctx,
		pw:         p,
		trackers:   &sync.Map{},
		tasksTotal: tasksTotal,
	}
}

func (p *progress) OnAdd(elem uploader.Elem) {
	tracker := prog.AppendTracker(p.pw, utils.Byte.FormatBinaryBytes, p.processMessage(elem), elem.File().Size())
	p.trackers.Store(p.tuple(elem), tracker)
	e := elem.(*iterElem)
	xprogress.Emit(p.ctx, xprogress.Event{
		Kind:       xprogress.KindStarted,
		Direction:  xprogress.DirectionUpload,
		Status:     xprogress.StatusRunning,
		TaskID:     e.file.File.Name(),
		TasksTotal: p.tasksTotal,
		FileName:   e.file.File.Name(),
		TotalBytes: elem.File().Size(),
		At:         time.Now(),
	})
}

func (p *progress) OnUpload(elem uploader.Elem, state uploader.ProgressState) {
	tracker, ok := p.trackers.Load(p.tuple(elem))
	if !ok {
		return
	}

	t := tracker.(*pw.Tracker)
	t.UpdateTotal(state.Total)
	t.SetValue(state.Uploaded)
	e := elem.(*iterElem)
	xprogress.Emit(p.ctx, xprogress.Event{
		Kind:           xprogress.KindUpdated,
		Direction:      xprogress.DirectionUpload,
		Status:         xprogress.StatusRunning,
		TaskID:         e.file.File.Name(),
		TasksTotal:     p.tasksTotal,
		FileName:       e.file.File.Name(),
		CompletedBytes: state.Uploaded,
		TotalBytes:     state.Total,
		At:             time.Now(),
	})
}

func (p *progress) OnDone(elem uploader.Elem, err error) {
	tracker, ok := p.trackers.Load(p.tuple(elem))
	if !ok {
		return
	}
	t := tracker.(*pw.Tracker)
	e := elem.(*iterElem)

	if err := p.closeFile(e); err != nil {
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
			Direction:      xprogress.DirectionUpload,
			Status:         status,
			TaskID:         e.file.File.Name(),
			TasksTotal:     p.tasksTotal,
			FileName:       e.file.File.Name(),
			CompletedBytes: e.file.Size(),
			TotalBytes:     e.file.Size(),
			At:             time.Now(),
			Err:            err.Error(),
		})
		p.fail(t, elem, errors.Wrap(err, "progress"))
		return
	}

	xprogress.Emit(p.ctx, xprogress.Event{
		Kind:           xprogress.KindFinished,
		Direction:      xprogress.DirectionUpload,
		Status:         xprogress.StatusDone,
		TaskID:         e.file.File.Name(),
		TasksTotal:     p.tasksTotal,
		FileName:       e.file.File.Name(),
		CompletedBytes: e.file.Size(),
		TotalBytes:     e.file.Size(),
		At:             time.Now(),
	})

	if e.remove {
		if err := os.Remove(e.file.File.Name()); err != nil {
			p.fail(t, elem, errors.Wrap(err, "remove file"))
			return
		}
	}
}

func (p *progress) closeFile(e *iterElem) error {
	if err := e.file.Close(); err != nil {
		return errors.Wrap(err, "close file")
	}

	if e.thumb != nil {
		if err := e.thumb.Close(); err != nil {
			return errors.Wrap(err, "close thumb")
		}
	}

	return nil
}

func (p *progress) fail(t *pw.Tracker, elem uploader.Elem, err error) {
	p.pw.Log(color.RedString("%s error: %s", p.elemString(elem), err.Error()))
	t.MarkAsErrored()
}

func (p *progress) tuple(elem uploader.Elem) tuple {
	return tuple{elem.(*iterElem).file.File.Name(), elem.(*iterElem).to.ID()}
}

func (p *progress) processMessage(elem uploader.Elem) string {
	return p.elemString(elem)
}

func (p *progress) elemString(elem uploader.Elem) string {
	e := elem.(*iterElem)
	return fmt.Sprintf("%s -> %s(%d)", e.file.File.Name(), e.to.VisibleName(), e.to.ID())
}
