package tui

import (
	"context"
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
	}, []string{"version"})

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
