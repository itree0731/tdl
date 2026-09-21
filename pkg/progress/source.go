package progress

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// Source assigns IDs to work items, independently of their display path.
type Source struct {
	mu        sync.Mutex
	ctx       context.Context
	direction Direction
	next      uint64
	items     map[any]Event
}

func NewSource(ctx context.Context, d Direction) *Source {
	return &Source{ctx: ctx, direction: d, items: make(map[any]Event)}
}
func (s *Source) Queue(key any, name string, size int64, expected int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[key]; ok {
		return
	}
	s.next++
	e := Event{Kind: KindQueued, Status: StatusQueued, TaskID: strconv.FormatUint(s.next, 10), Direction: s.direction, FileName: name, TotalBytes: size, TasksTotal: expected}
	s.send(key, e)
}
func (s *Source) send(key any, e Event) {
	e.Sequence++
	e.At = time.Now()
	s.items[key] = e
	Emit(s.ctx, e)
}
func (s *Source) Start(key any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[key]
	if !ok || terminal(e) {
		return
	}
	e.Kind = KindStarted
	e.Status = StatusRunning
	e.Phase = "transferring"
	s.send(key, e)
}
func (s *Source) Update(key any, completed, total int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[key]
	if !ok || terminal(e) {
		return
	}
	e.Kind = KindUpdated
	e.CompletedBytes = max(e.CompletedBytes, completed)
	e.TotalBytes = total
	s.send(key, e)
}
func (s *Source) Finish(key any, phase string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[key]
	if !ok || terminal(e) {
		return
	}
	e.Kind = KindFinished
	e.Phase = phase
	e.Status = StatusDone
	if err != nil {
		e.Err = err.Error()
		e.Status = StatusFailed
		if IsCancellation(s.ctx, err) {
			e.Status = StatusCanceled
		}
	} else if e.TotalBytes > 0 {
		e.CompletedBytes = e.TotalBytes
	}
	s.send(key, e)
}
func (s *Source) DiscoveryDone() { Emit(s.ctx, Event{Kind: KindDiscoveryDone, Direction: s.direction}) }
