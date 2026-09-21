package progress

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

type Snapshot struct {
	Discovered, Expected, Pending, Running, Succeeded, Failed, Canceled, Skipped int
	CompletedBytes, TotalBytes                                                   int64
	Unknown                                                                      int
	DiscoveryDone, Final                                                         bool
	CurrentFile, CurrentSourcePath, Phase, ProcessInfo                           string
	Speed                                                                        float64
	Status                                                                       Status
	Errors                                                                       []string
}

func (c *Collector) Items() []Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	items := make([]Event, 0, len(c.items))
	for _, event := range c.items {
		items = append(items, event)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].TaskID < items[j].TaskID })
	return items
}

func (s Snapshot) Percent() (float64, bool) {
	if !s.DiscoveryDone || s.Unknown > 0 || s.TotalBytes <= 0 {
		return 0, false
	}
	return math.Min(100, float64(s.CompletedBytes)*100/float64(s.TotalBytes)), true
}
func (s Snapshot) ETA() string {
	if _, ok := s.Percent(); !ok || s.Speed <= 0 || s.Final {
		return "--"
	}
	return (time.Duration(float64(max(0, s.TotalBytes-s.CompletedBytes))/s.Speed) * time.Second).String()
}
func Bytes(n int64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}
func (s Snapshot) Summary(zh bool) string {
	suffix := ""
	if !s.DiscoveryDone && s.Discovered > 0 {
		suffix = " · total unknown"
		if zh {
			suffix = " · 总量未确定"
		}
	}
	if zh {
		return fmt.Sprintf("待处理 %d · 运行 %d · 成功 %d · 失败 %d · 取消 %d · 跳过 %d", s.Pending, s.Running, s.Succeeded, s.Failed, s.Canceled, s.Skipped) + suffix
	}
	return fmt.Sprintf("pending %d · running %d · succeeded %d · failed %d · canceled %d · skipped %d", s.Pending, s.Running, s.Succeeded, s.Failed, s.Canceled, s.Skipped) + suffix
}
func (s Snapshot) Metrics() string {
	pct := "--"
	if p, ok := s.Percent(); ok {
		pct = fmt.Sprintf("%.1f%%", p)
	}
	total := "--"
	if _, known := s.Percent(); known {
		total = Bytes(s.TotalBytes)
	}
	return fmt.Sprintf("%s · %s / %s · %s/s · ETA %s", pct, Bytes(s.CompletedBytes), total, Bytes(int64(s.Speed)), s.ETA())
}

// Collector coalesces chunk updates under a short lock. Rendering never runs in
// a transfer callback. Snapshot copies are independent of subsequent events.
type Collector struct {
	mu         sync.Mutex
	items      map[string]Event
	snapshot   Snapshot
	lastSample time.Time
	lastBytes  int64
}

func NewCollector() *Collector {
	return &Collector{items: make(map[string]Event), snapshot: Snapshot{Status: StatusRunning}, lastSample: time.Now()}
}
func terminal(e Event) bool { return e.Kind == KindFinished || e.Kind == KindSkipped }
func (c *Collector) adjust(e Event, delta int) {
	s := &c.snapshot
	s.Discovered += delta
	s.CompletedBytes += int64(delta) * e.CompletedBytes
	if e.TotalBytes > 0 {
		s.TotalBytes += int64(delta) * e.TotalBytes
	} else if e.Kind != KindSkipped {
		s.Unknown += delta
	}
	switch e.Status {
	case StatusQueued:
		s.Pending += delta
	case StatusDone:
		s.Succeeded += delta
	case StatusFailed:
		s.Failed += delta
	case StatusCanceled:
		s.Canceled += delta
	case StatusSkipped:
		s.Skipped += delta
	default:
		s.Running += delta
	}
}
func (c *Collector) Emit(e Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.snapshot.Final {
		return
	}
	if e.Kind == KindTelemetry {
		c.snapshot.ProcessInfo = e.Info
		return
	}
	if e.Kind == KindDiscoveryDone {
		c.snapshot.DiscoveryDone = true
		return
	}
	if e.TaskID == "" {
		return
	}
	if old, ok := c.items[e.TaskID]; ok {
		if terminal(old) {
			return
		}
		if !terminal(e) && (e.Sequence > 0 && old.Sequence > e.Sequence || !e.At.IsZero() && old.At.After(e.At)) {
			return
		}
		e.CompletedBytes = max(old.CompletedBytes, e.CompletedBytes)
		c.adjust(old, -1)
	}
	e.CompletedBytes = max(0, e.CompletedBytes)
	if e.TotalBytes > 0 {
		e.CompletedBytes = min(e.TotalBytes, e.CompletedBytes)
	}
	c.items[e.TaskID] = e
	c.adjust(e, 1)
	c.snapshot.Expected = max(c.snapshot.Expected, e.TasksTotal)
	if e.Kind != KindQueued && e.Kind != KindSkipped {
		c.snapshot.CurrentFile = e.FileName
		c.snapshot.CurrentSourcePath = e.SourcePath
		c.snapshot.Phase = e.Phase
	}
	if e.Err != "" {
		c.snapshot.Errors = append(c.snapshot.Errors, fmt.Sprintf("%s: %s", e.FileName, e.Err))
	}
}
func (c *Collector) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if !c.lastSample.IsZero() && now.Sub(c.lastSample) >= 100*time.Millisecond {
		speed := float64(max(0, c.snapshot.CompletedBytes-c.lastBytes)) / now.Sub(c.lastSample).Seconds()
		c.snapshot.Speed = speed
		c.lastSample = now
		c.lastBytes = c.snapshot.CompletedBytes
	} else if c.lastSample.IsZero() {
		c.lastSample = now
		c.lastBytes = c.snapshot.CompletedBytes
	}
	s := c.snapshot
	if !s.DiscoveryDone {
		s.Pending += max(0, s.Expected-s.Discovered)
	}
	s.Errors = append([]string(nil), s.Errors...)
	return s
}
func IsCancellation(ctx context.Context, err error) bool {
	return errors.Is(err, context.Canceled) || (errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil)
}

func (c *Collector) Finish(err error) Snapshot { return c.FinishContext(context.Background(), err) }
func (c *Collector) FinishContext(ctx context.Context, err error) Snapshot {
	c.mu.Lock()
	if !c.snapshot.Final {
		for id, e := range c.items {
			if !terminal(e) {
				c.adjust(e, -1)
				e.Kind = KindFinished
				if IsCancellation(ctx, err) {
					e.Status = StatusCanceled
				} else {
					e.Status = StatusFailed
				}
				c.items[id] = e
				c.adjust(e, 1)
			}
		}
		s := &c.snapshot
		s.Final = true
		switch {
		case IsCancellation(ctx, err) || s.Canceled > 0:
			s.Status = StatusCanceled
		case s.Failed > 0 && s.Succeeded > 0:
			s.Status = StatusPartial
		case s.Failed > 0:
			s.Status = StatusFailed
		case s.Canceled > 0:
			s.Status = StatusCanceled
		case errors.Is(err, context.Canceled):
			s.Status = StatusCanceled
		case err != nil:
			s.Status = StatusFailed
		default:
			s.Status = StatusDone
		}
		if err != nil {
			s.Errors = append(s.Errors, err.Error())
		}
	}
	c.mu.Unlock()
	return c.Snapshot()
}
