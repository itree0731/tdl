package tui

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
)

// Executor runs one tdl command with the given argv, writing combined
// output to out. It is injected from package cmd to avoid an import cycle.
type Executor func(ctx context.Context, argv []string, out io.Writer) error

type outputLineMsg struct{ text string } // complete line, for the scrollback
type liveLineMsg struct{ text string }   // \r-refreshed segment, status line only
type runDoneMsg struct {
	err     error
	elapsed time.Duration
}

// startRun launches argv in the background and streams its output into the
// program as messages. The returned func cancels the run.
func startRun(parent context.Context, p *tea.Program, exec Executor, argv []string) (cancel func()) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)

	go func() {
		start := time.Now()

		// nil program (unit tests): run, but drop the messages
		send := func(msg tea.Msg) {
			if p != nil {
				p.Send(msg)
			}
		}

		// os.Pipe (not io.Pipe): the write end is a real *os.File, so the
		// executor can swap os.Stdout/os.Stderr to it and capture output
		// from code that bypasses the cobra writer (e.g. go-pretty bars)
		pr, pw, err := os.Pipe()
		if err != nil {
			send(runDoneMsg{err: err, elapsed: time.Since(start)})
			return
		}
		defer pr.Close()

		result := make(chan error, 1)
		go func() {
			err := exec(ctx, argv, pw)
			_ = pw.Close() // unblocks the reader below
			result <- err
		}()

		buf := make([]byte, 0, 8192)
		tmp := make([]byte, 8192)
		for {
			n, err := pr.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
				lines, live, rest := splitStream(buf)
				buf = rest
				for _, l := range lines {
					send(outputLineMsg{text: l})
				}
				if live != "" {
					send(liveLineMsg{text: live})
				}
			}
			if err != nil {
				break
			}
		}
		if rest := strings.TrimRight(string(buf), "\r\n"); strings.TrimSpace(rest) != "" {
			send(outputLineMsg{text: rest})
		}

		runErr := <-result
		send(runDoneMsg{err: runErr, elapsed: time.Since(start)})
	}()

	return cancel
}

// splitStream splits a raw output chunk into complete lines and, if a
// progress-bar redraw (\r) is in flight, the latest live segment. The
// unconsumed remainder is returned for the next chunk.
func splitStream(buf []byte) (lines []string, live string, rest []byte) {
	for {
		if i := bytes.IndexByte(buf, '\n'); i >= 0 {
			line := string(buf[:i])
			buf = buf[i+1:]
			// progress bars redraw with \r inside one line:
			// only the final segment is the real content
			if j := strings.LastIndexByte(line, '\r'); j >= 0 {
				line = line[j+1:]
			}
			lines = append(lines, line)
			continue
		}
		if j := bytes.LastIndexByte(buf, '\r'); j >= 0 {
			live = string(buf[j+1:])
			buf = buf[j+1:]
		}
		return lines, live, buf
	}
}
