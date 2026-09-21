package progress

import (
	"context"
	"time"
)

// Direction identifies the kind of transfer producing an event.
type Direction string

const (
	DirectionDownload Direction = "download"
	DirectionUpload   Direction = "upload"
)

// Kind identifies a task lifecycle event.
type Kind string

const (
	KindTelemetry     Kind = "telemetry"
	KindQueued        Kind = "queued"
	KindDiscoveryDone Kind = "discovery_done"
	KindSkipped       Kind = "skipped"
	KindStarted       Kind = "started"
	KindUpdated       Kind = "updated"
	KindFinished      Kind = "finished"
)

// Status is the task state represented by an event.
type Status string

const (
	StatusQueued   Status = "queued"
	StatusPartial  Status = "partial_failure"
	StatusSkipped  Status = "skipped"
	StatusRunning  Status = "running"
	StatusDone     Status = "done"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
)

// Event is the common progress contract shared by download and upload.
// TotalBytes <= 0 means that the item has no known total.
type Event struct {
	Kind      Kind
	Direction Direction
	Status    Status

	TaskID     string
	TasksTotal int
	FileName   string

	CompletedBytes int64
	TotalBytes     int64
	At             time.Time
	Err            string
	Phase          string
	Info           string
	Sequence       uint64
}

// Sink receives structured transfer events.
type Sink interface {
	Emit(Event)
}

type SinkFunc func(Event)

func (f SinkFunc) Emit(event Event) { f(event) }

type sinkKey struct{}

func WithSink(ctx context.Context, sink Sink) context.Context {
	return context.WithValue(ctx, sinkKey{}, sink)
}

func Emit(ctx context.Context, event Event) {
	if sink, ok := ctx.Value(sinkKey{}).(Sink); ok && sink != nil {
		sink.Emit(event)
	}
}

func HasSink(ctx context.Context) bool {
	sink, ok := ctx.Value(sinkKey{}).(Sink)
	return ok && sink != nil
}

// Skip records a filtered, missing, or previously completed input separately.
func Skip(ctx context.Context, id, name string) {
	Emit(ctx, Event{Kind: KindSkipped, Status: StatusSkipped, TaskID: "skip:" + id, FileName: name, At: time.Now()})
}
