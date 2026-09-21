package progress

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWeightedUnknownAndDiscovery(t *testing.T) {
	c := NewCollector()
	c.Emit(Event{TaskID: "a", Kind: KindStarted, Status: StatusRunning, TotalBytes: 100, CompletedBytes: 50})
	if _, known := c.Snapshot().Percent(); known {
		t.Fatal("unsealed collection has no reliable total")
	}
	c.Emit(Event{TaskID: "b", Kind: KindStarted, Status: StatusRunning, TotalBytes: 300, CompletedBytes: 50})
	c.Emit(Event{Kind: KindDiscoveryDone})
	if pct, known := c.Snapshot().Percent(); !known || pct != 25 {
		t.Fatalf("%v %v", pct, known)
	}
	c.Emit(Event{TaskID: "unknown", Kind: KindStarted, Status: StatusRunning, CompletedBytes: 10})
	if _, known := c.Snapshot().Percent(); known {
		t.Fatal("mixed unknown sizes")
	}
	if c.Snapshot().CompletedBytes != 110 {
		t.Fatal("unknown bytes were discarded")
	}
}
func TestSourceFailureCancelDuplicateAndOrdering(t *testing.T) {
	c := NewCollector()
	s := NewSource(WithSink(context.Background(), c), DirectionUpload)
	a, b := new(int), new(int)
	s.Queue(a, "same-path", 100, 2)
	s.Queue(b, "same-path", 100, 2)
	s.Start(a)
	s.Update(a, 10, 100)
	s.Finish(a, "transferring", errors.New("failed"))
	s.Update(a, 100, 100)
	s.Finish(a, "done", nil)
	s.Start(b)
	s.Update(b, 10, 100)
	s.Finish(b, "transferring", context.Canceled)
	s.DiscoveryDone()
	got := c.Finish(errors.New("batch failed"))
	if got.Discovered != 2 || got.Failed != 1 || got.Canceled != 1 || got.Succeeded != 0 || got.CompletedBytes != 20 {
		t.Fatalf("%+v", got)
	}
	c = NewCollector()
	c.Emit(Event{TaskID: "a", Kind: KindUpdated, Sequence: 2, Status: StatusRunning, TotalBytes: 100, CompletedBytes: 50})
	c.Emit(Event{TaskID: "a", Kind: KindUpdated, Sequence: 1, Status: StatusRunning, TotalBytes: 100, CompletedBytes: 10})
	if c.Snapshot().CompletedBytes != 50 {
		t.Fatal("late update regressed progress")
	}
}
func TestFinishCannotEraseKnownFailure(t *testing.T) {
	c := NewCollector()
	c.Emit(Event{TaskID: "a", Kind: KindFinished, Status: StatusFailed, Err: "failed"})
	c.Emit(Event{TaskID: "b", Kind: KindFinished, Status: StatusDone})
	c.Emit(Event{TaskID: "skip:x", Kind: KindSkipped, Status: StatusSkipped})
	s := c.Finish(nil)
	if s.Status != StatusPartial || s.Skipped != 1 {
		t.Fatalf("%+v", s)
	}
	c.Emit(Event{TaskID: "c", Kind: KindFinished, Status: StatusDone})
	if c.Snapshot().Discovered != 3 {
		t.Fatal("late event changed final snapshot")
	}
}
func TestConcurrentSources(t *testing.T) {
	c := NewCollector()
	s := NewSource(WithSink(context.Background(), c), DirectionDownload)
	var wg sync.WaitGroup
	for n := 0; n < 100; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.Queue(n, fmt.Sprint(n), 100, 100)
			s.Start(n)
			for i := 0; i <= 100; i++ {
				s.Update(n, int64(i), 100)
			}
			s.Finish(n, "done", nil)
		}(n)
	}
	wg.Wait()
	s.DiscoveryDone()
	got := c.Finish(nil)
	if got.Succeeded != 100 || got.CompletedBytes != 10000 {
		t.Fatalf("%+v", got)
	}
}
func TestCLIRedirectionAndExistingSink(t *testing.T) {
	var out bytes.Buffer
	ctx, finish := StartCLI(context.Background(), &out)
	s := NewSource(ctx, DirectionUpload)
	s.Queue(1, "file", 100, 1)
	s.Start(1)
	s.Update(1, 10, 100)
	s.Finish(1, "transferring", errors.New("failed"))
	s.DiscoveryDone()
	finish(errors.New("batch"))
	finish(nil)
	if strings.ContainsAny(out.String(), "\r\x1b") || !strings.Contains(out.String(), "failed 1") {
		t.Fatal(out.String())
	}
	c := NewCollector()
	out.Reset()
	_, finish = StartCLI(WithSink(context.Background(), c), &out)
	finish(nil)
	if out.Len() != 0 {
		t.Fatal("TUI sink also rendered CLI text")
	}
}

func TestInternalTimeoutIsFailureAndBatchDeadlineIsCancellation(t *testing.T) {
	c := NewCollector()
	s := NewSource(WithSink(context.Background(), c), DirectionDownload)
	s.Queue(1, "a", 100, 2)
	s.Start(1)
	s.Finish(1, "transferring", context.DeadlineExceeded)
	s.Queue(2, "b", 100, 2)
	s.Start(2)
	s.Finish(2, "done", nil)
	s.DiscoveryDone()
	got := c.Finish(context.DeadlineExceeded)
	if got.Status != StatusPartial || got.Failed != 1 || got.Canceled != 0 {
		t.Fatalf("%+v", got)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	got = NewCollector().FinishContext(ctx, context.DeadlineExceeded)
	if got.Status != StatusCanceled {
		t.Fatalf("%+v", got)
	}
}
func TestLongCLIPathPreservesMetrics(t *testing.T) {
	s := Snapshot{CurrentFile: strings.Repeat("长路径", 100), DiscoveryDone: true, TotalBytes: 1000, CompletedBytes: 500, Speed: 100}
	for _, w := range []int{32, 80, 120} {
		line := FormatLine(s, w)
		if ansi.StringWidth(line) > w || !strings.Contains(line, "50") || !strings.Contains(line, "ETA") {
			t.Fatalf("width=%d line=%s", w, line)
		}
	}
}
