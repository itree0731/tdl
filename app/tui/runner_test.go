package tui

import (
	"context"
	"errors"
	xp "github.com/iyear/tdl/pkg/progress"
	"io"
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"
)

type runnerTestModel struct {
	done chan runDoneMsg
}

func (m runnerTestModel) Init() tea.Cmd { return nil }

func (m runnerTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if done, ok := msg.(runDoneMsg); ok {
		m.done <- done
		return m, tea.Quit
	}
	return m, nil
}

func (m runnerTestModel) View() string { return "" }

func TestStartRunDeliversCompletionToProgram(t *testing.T) {
	done := make(chan runDoneMsg, 1)
	p := tea.NewProgram(runnerTestModel{done: done},
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
		tea.WithoutSignals(),
	)
	programDone := make(chan error, 1)
	go func() {
		_, err := p.Run()
		programDone <- err
	}()

	startRun(context.Background(), p, func(ctx context.Context, argv []string, out io.Writer) error {
		_, _ = io.WriteString(out, "completed\n")
		return nil
	}, []string{"version"}, 1)

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("run error = %v", result.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("program did not receive runDoneMsg")
	}

	select {
	case err := <-programDone:
		if err != nil {
			t.Fatalf("program error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("program did not exit after runDoneMsg")
	}
}

func TestCanceledExecutorReturningNilStillReportsCanceled(t *testing.T) {
	done := make(chan runDoneMsg, 1)
	p := tea.NewProgram(runnerTestModel{done: done}, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutSignals())
	exited := make(chan error, 1)
	go func() { _, err := p.Run(); exited <- err }()
	started := make(chan struct{})
	cancel := startRun(context.Background(), p, func(ctx context.Context, _ []string, _ io.Writer) error { close(started); <-ctx.Done(); return nil }, nil, 7)
	<-started
	cancel()
	select {
	case result := <-done:
		if !errors.Is(result.err, context.Canceled) || result.snapshot.Status != xp.StatusCanceled {
			t.Fatalf("%+v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not complete")
	}
	select {
	case err := <-exited:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("program did not exit")
	}
}
