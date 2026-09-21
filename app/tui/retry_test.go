package tui

import (
	"context"
	"io"
	"reflect"
	"testing"
	"time"

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
