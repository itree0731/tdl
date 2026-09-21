package progress

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/iyear/tdl/pkg/ps"
	"golang.org/x/term"
)

// StartCLI installs a collector only when a TUI sink is not already present.
func StartCLI(ctx context.Context, out io.Writer, processStats ...bool) (context.Context, func(error)) {
	if HasSink(ctx) {
		stopStats := startStats(ctx, processStats)
		return ctx, func(error) { stopStats() }
	}
	c := NewCollector()
	ctx = WithSink(ctx, c)
	stopStats := startStats(ctx, processStats)
	tty := false
	width := 100
	if f, ok := out.(*os.File); ok {
		tty = term.IsTerminal(int(f.Fd()))
		if w, _, err := term.GetSize(int(f.Fd())); err == nil {
			width = w
		}
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if tty {
					s := c.Snapshot()
					fmt.Fprint(out, "\r\x1b[2K", FormatLine(s, width))
				}
			}
		}
	}()
	return ctx, func(err error) {
		once.Do(func() {
			close(stop)
			<-done
			stopStats()
			s := c.FinishContext(ctx, err)
			if tty {
				fmt.Fprint(out, "\r\x1b[2K")
			}
			fmt.Fprintf(out, "%s: %s\n%s\n", s.Status, s.Summary(false), s.Metrics())
			if s.ProcessInfo != "" {
				fmt.Fprintln(out, s.ProcessInfo)
			}
			for _, e := range s.Errors {
				fmt.Fprintln(out, e)
			}
		})
	}
}

// FormatLine preserves progress metrics before spending the remaining columns on a filename.
func FormatLine(s Snapshot, width int) string {
	metrics := s.Metrics()
	if s.ProcessInfo != "" && ansi.StringWidth(metrics)+ansi.StringWidth(s.ProcessInfo)+3 < width {
		metrics += " · " + s.ProcessInfo
	}
	if ansi.StringWidth(metrics) > width {
		pct := "--"
		if p, known := s.Percent(); known {
			pct = fmt.Sprintf("%.0f%%", p)
		}
		metrics = fmt.Sprintf("%s %s/s ETA %s", pct, Bytes(int64(s.Speed)), s.ETA())
	}
	room := width - ansi.StringWidth(metrics) - 3
	if room > 3 && s.CurrentFile != "" {
		return ansi.Truncate(s.CurrentFile, room, "…") + " · " + metrics
	}
	return ansi.Truncate(strings.TrimSpace(metrics), max(1, width), "…")
}

func startStats(ctx context.Context, enabled []bool) func() {
	if len(enabled) == 0 || !enabled[0] {
		return func() {}
	}
	child, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			Emit(ctx, Event{Kind: KindTelemetry, Info: strings.Join(ps.Humanize(child), " · ")})
			select {
			case <-child.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func() { cancel(); <-done }
}
