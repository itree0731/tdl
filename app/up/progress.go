package up

import (
	"context"
	"errors"
	"fmt"
	"github.com/iyear/tdl/core/uploader"
	xprogress "github.com/iyear/tdl/pkg/progress"
	"os"
)

type progress struct {
	source     *xprogress.Source
	tasksTotal int
}

func newProgress(ctx context.Context, tasksTotal int) *progress {
	return &progress{source: xprogress.NewSource(ctx, xprogress.DirectionUpload), tasksTotal: tasksTotal}
}
func (p *progress) OnQueued(elem uploader.Elem) {
	sourcePath := ""
	if e, ok := elem.(*iterElem); ok && e.file != nil {
		sourcePath = e.file.File.Name()
	}
	p.source.QueuePath(elem, elem.File().Name(), sourcePath, elem.File().Size(), p.tasksTotal)
}
func (p *progress) OnAdd(elem uploader.Elem) { p.OnQueued(elem); p.source.Start(elem) }
func (p *progress) OnUpload(elem uploader.Elem, state uploader.ProgressState) {
	p.source.Update(elem, state.Uploaded, state.Total)
}
func (p *progress) OnDiscoveryDone()                     { p.source.DiscoveryDone() }
func (p *progress) OnDone(elem uploader.Elem, err error) { _ = p.Finalize(elem, err) }
func (p *progress) Finalize(elem uploader.Elem, err error) error {
	e := elem.(*iterElem)
	phase := "transferring"
	uploaded := err == nil
	if closeErr := p.closeFile(e); closeErr != nil {
		err = errors.Join(err, closeErr)
		phase = "closing"
	}
	if uploaded && err == nil && e.remove {
		phase = "uploaded_cleanup"
		if removeErr := os.Remove(e.file.File.Name()); removeErr != nil {
			err = fmt.Errorf("uploaded successfully; source deletion failed: %w", removeErr)
		}
	}
	if err == nil {
		phase = "done"
	}
	p.source.Finish(elem, phase, err)
	return err
}
func (p *progress) closeFile(e *iterElem) error {
	err := e.file.Close()
	if e.thumb != nil {
		err = errors.Join(err, e.thumb.Close())
	}
	if e.cover != nil {
		err = errors.Join(err, e.cover.Close())
	}
	for _, path := range e.temporaryFiles {
		err = errors.Join(err, os.Remove(path))
	}
	if err != nil {
		return fmt.Errorf("close upload file: %w", err)
	}
	return nil
}
