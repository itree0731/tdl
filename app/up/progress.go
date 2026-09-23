package up

import (
	stderrors "errors"
	"fmt"
	"os"
	"sync"

	"github.com/fatih/color"
	"github.com/go-faster/errors"
	pw "github.com/jedib0t/go-pretty/v6/progress"

	"github.com/iyear/tdl/core/uploader"
	"github.com/iyear/tdl/pkg/prog"
	"github.com/iyear/tdl/pkg/utils"
)

type progress struct {
	pw       pw.Writer
	trackers *sync.Map // map[tuple]*pw.Tracker
}

var removeUploadedSource = os.Remove

type tuple struct {
	name string
	to   int64
}

func newProgress(p pw.Writer) *progress {
	return &progress{
		pw:       p,
		trackers: &sync.Map{},
	}
}

func (p *progress) OnAdd(elem uploader.Elem) {
	tracker := prog.AppendTracker(p.pw, utils.Byte.FormatBinaryBytes, p.processMessage(elem), elem.File().Size())
	p.trackers.Store(p.tuple(elem), tracker)
}

func (p *progress) OnUpload(elem uploader.Elem, state uploader.ProgressState) {
	tracker, ok := p.trackers.Load(p.tuple(elem))
	if !ok {
		return
	}

	t := tracker.(*pw.Tracker)
	t.UpdateTotal(state.Total)
	t.SetValue(state.Uploaded)
}

func (p *progress) OnDone(elem uploader.Elem, err error) {
	_ = p.Finalize(elem, err)
}

func (p *progress) Finalize(elem uploader.Elem, transferErr error) error {
	tracker, ok := p.trackers.Load(p.tuple(elem))
	if !ok {
		return fmt.Errorf("missing upload progress tracker for %s", elem.File().Name())
	}
	t := tracker.(*pw.Tracker)
	e := elem.(*iterElem)
	err := finalizeUploadFile(e, transferErr)
	if err != nil {
		p.fail(t, elem, err)
		return err
	}
	t.MarkAsDone()
	return nil
}

func finalizeUploadFile(e *iterElem, transferErr error) error {
	err := transferErr
	if closeErr := closeUploadFile(e); closeErr != nil {
		err = stderrors.Join(err, errors.Wrap(closeErr, "close upload file"))
	}
	if transferErr == nil && err == nil && e.remove {
		if removeErr := removeUploadedSource(e.file.File.Name()); removeErr != nil {
			err = fmt.Errorf("uploaded successfully; source deletion failed: %w", removeErr)
		}
	}
	return err
}

func closeUploadFile(e *iterElem) error {
	err := e.file.Close()
	if e.thumb != nil {
		err = stderrors.Join(err, e.thumb.Close())
	}
	return err
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
