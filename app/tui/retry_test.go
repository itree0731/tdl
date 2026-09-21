package tui

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	xprogress "github.com/iyear/tdl/pkg/progress"
)

func TestBuildRunResultClassifiesUploadRisks(t *testing.T) {
	result := buildRunResult([]xprogress.Event{
		{TaskID: "1", Direction: xprogress.DirectionUpload, Status: xprogress.StatusFailed, FileName: "a.mp4", SourcePath: "a.mp4", Phase: "uploaded_cleanup", Err: "delete failed"},
		{TaskID: "2", Direction: xprogress.DirectionUpload, Status: xprogress.StatusFailed, FileName: "b.mp4", SourcePath: "b.mp4", Phase: "transferring", Err: "send message: timeout"},
		{TaskID: "3", Direction: xprogress.DirectionUpload, Status: xprogress.StatusFailed, FileName: "c.mp4", SourcePath: "c.mp4", Phase: "transferring", Err: "prepare video cover: ffmpeg"},
	})
	if len(result.Items) != 3 || result.Items[0].Retry != RetryCleanupOnly || result.Items[1].Retry != RetryUncertain || result.Items[2].Retry != RetrySafe {
		t.Fatalf("retry classes = %+v", result.Items)
	}
}

func TestRetryUploadQueuesOnlyFailedPaths(t *testing.T) {
	isolateSettings(t)
	argvCh := make(chan []string, 1)
	exec := func(_ context.Context, argv []string, _ io.Writer) error {
		argvCh <- append([]string(nil), argv...)
		return nil
	}
	m := newModel(exec, nil)
	m.currentRun = RunSpec{ActionID: "up", Args: []string{"up", "-p", "done.mp4", "-p", "failed.mp4", "--cover-mode", "video-cover"}, Display: RunDisplay{Title: "Upload"}}
	m.runResult = RunResult{Items: []ItemResult{{SourcePath: "failed.mp4", Retry: RetrySafe}}}
	r, _ := m.retryFailed(false)
	m = asModel(r)
	if !m.running {
		t.Fatal("retry did not start")
	}
	select {
	case got := <-argvCh:
		want := []string{"up", "--cover-mode", "video-cover", "-p", "failed.mp4"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("retry argv = %v, want %v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("retry executor not called")
	}
}

func TestUncertainRetryRequiresExplicitConfirmation(t *testing.T) {
	isolateSettings(t)
	m := newModel(stubExec, nil)
	m.currentRun = RunSpec{ActionID: "up", Args: []string{"up", "-p", "maybe.mp4"}}
	m.runResult = RunResult{Items: []ItemResult{{SourcePath: "maybe.mp4", Retry: RetryUncertain}}}
	r, _ := m.retryFailed(false)
	m = asModel(r)
	if !m.retryPrompt || m.running || m.state() != screenConfirm {
		t.Fatalf("uncertain retry state: prompt=%v running=%v state=%v", m.retryPrompt, m.running, m.state())
	}
}

func TestUncertainRetryConfirmationHasMouseControls(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, nil), 120, 30)
	m.showOutput = true
	m.currentRun = RunSpec{ActionID: "up", Args: []string{"up", "-p", "maybe.mp4"}}
	m.runResult = RunResult{Items: []ItemResult{{SourcePath: "maybe.mp4", Retry: RetryUncertain}}}
	r, _ := m.retryFailed(false)
	m = asModel(r)
	frame := m.frame()
	plain := renderPlain(m)
	lines := strings.Split(plain, "\n")
	var cancel *HitRegion
	for i := range frame.Regions {
		if frame.Regions[i].ID == "retry.cancel" {
			cancel = &frame.Regions[i]
			break
		}
	}
	if cancel == nil || cancel.Rect.Y >= len(lines) {
		t.Fatalf("missing retry cancel region: %+v", cancel)
	}
	visible := ansi.Cut(lines[cancel.Rect.Y], cancel.Rect.X, cancel.Rect.X+cancel.Rect.W)
	if !strings.Contains(visible, "Cancel") {
		t.Fatalf("cancel region does not cover visible button: %q rect=%+v\n%s", visible, cancel.Rect, plain)
	}
	r, _ = m.Update(tea.MouseMsg{X: cancel.Rect.X, Y: cancel.Rect.Y, Button: tea.MouseButtonLeft})
	if asModel(r).retryPrompt {
		t.Fatal("mouse cancel did not close retry confirmation")
	}
}

func TestCanceledRunContinuesOnlyRemainingUploadPaths(t *testing.T) {
	isolateSettings(t)
	argvCh := make(chan []string, 1)
	exec := func(_ context.Context, argv []string, _ io.Writer) error {
		argvCh <- append([]string(nil), argv...)
		return nil
	}
	m := sized(t, newModel(exec, nil), 120, 30)
	m.showOutput = true
	m.progress.Status = xprogress.StatusCanceled
	m.currentRun = RunSpec{ActionID: "up", Args: []string{"up", "-p", "done.mp4", "-p", "remaining.mp4"}, Display: RunDisplay{Title: "Upload"}}
	m.runResult = RunResult{Items: []ItemResult{{DisplayName: "remaining.mp4", SourcePath: "remaining.mp4", Status: string(xprogress.StatusCanceled), Retry: RetryRestartItem}}}
	r, _ := m.activateButton("run.continue")
	m = asModel(r)
	if !m.running {
		t.Fatal("continue remaining did not start")
	}
	select {
	case argv := <-argvCh:
		if containsStr(argv, "done.mp4") || !containsStr(argv, "remaining.mp4") {
			t.Fatalf("continue argv=%v", argv)
		}
	case <-time.After(time.Second):
		t.Fatal("continued executor not invoked")
	}
}

func TestCleanupOnlyRetryDeletesWithoutLaunchingUpload(t *testing.T) {
	isolateSettings(t)
	path := filepath.Join(t.TempDir(), "uploaded.mp4")
	if err := os.WriteFile(path, []byte("sent"), 0600); err != nil {
		t.Fatal(err)
	}
	called := false
	m := newModel(func(context.Context, []string, io.Writer) error { called = true; return nil }, nil)
	m.showOutput = true
	m.currentRun = RunSpec{ActionID: "up", Args: []string{"up", "-p", path}}
	m.progress = xprogress.Snapshot{Status: xprogress.StatusPartial, Failed: 1}
	m.runResult = RunResult{Items: []ItemResult{{SourcePath: path, Retry: RetryCleanupOnly}}}
	r, _ := m.retryFailed(false)
	m = asModel(r)
	if called || m.running || len(m.runResult.Items) != 0 {
		t.Fatalf("cleanup retry launched upload or retained failure: called=%v running=%v items=%d", called, m.running, len(m.runResult.Items))
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cleanup did not delete source: %v", err)
	}
}

func TestCommandFailureBeforeDiscoveryProducesFailureItem(t *testing.T) {
	isolateSettings(t)
	m := newModel(stubExec, nil)
	m.running, m.showOutput, m.runID, m.runLabel = true, true, 4, "Upload"
	r, _ := m.Update(runDoneMsg{runID: 4, err: errors.New("invalid source path")})
	m = asModel(r)
	if m.progress.Status != xprogress.StatusFailed || m.progress.Failed != 1 || len(m.runResult.Items) != 1 {
		t.Fatalf("status=%s failed=%d items=%+v", m.progress.Status, m.progress.Failed, m.runResult.Items)
	}
	if m.runResult.Items[0].Phase != "command" || m.runResult.Items[0].Retry != RetryNotAllowed {
		t.Fatalf("synthetic failure=%+v", m.runResult.Items[0])
	}
}
