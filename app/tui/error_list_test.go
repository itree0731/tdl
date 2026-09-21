package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type fakeClipboard struct {
	value string
	err   error
}

func (c *fakeClipboard) Copy(value string) error {
	c.value = value
	return c.err
}

func TestFailureListSelectsAndCopiesExactError(t *testing.T) {
	isolateSettings(t)
	clipboard := &fakeClipboard{}
	m := sized(t, newModel(stubExec, nil), 120, 30)
	m.showOutput = true
	m.clipboard = clipboard
	m.runResult = RunResult{Items: []ItemResult{
		{DisplayName: "first.mp4", SourcePath: `C:\media\first.mp4`, Phase: "transferring", Err: "network timeout", Retry: RetryRestartItem},
		{DisplayName: "second.mp4", SourcePath: `C:\media\second.mp4`, Phase: "uploaded_cleanup", Err: "access denied", Retry: RetryCleanupOnly},
	}}
	m.openErrorList()
	frame := m.frame()
	var second, copyButton, backButton *HitRegion
	for i := range frame.Regions {
		region := &frame.Regions[i]
		switch region.ID {
		case "errors.row:1":
			second = region
		case "errors.copy":
			copyButton = region
		case "errors.back":
			backButton = region
		}
	}
	if second == nil || copyButton == nil || backButton == nil {
		t.Fatalf("missing failure list regions: second=%v copy=%v back=%v", second, copyButton, backButton)
	}
	r, _ := m.Update(tea.MouseMsg{X: second.Rect.X, Y: second.Rect.Y, Button: tea.MouseButtonLeft})
	m = asModel(r)
	if m.errorCursor != 1 {
		t.Fatalf("error cursor=%d", m.errorCursor)
	}
	frame = m.frame()
	plain := renderPlain(m)
	lines := strings.Split(plain, "\n")
	visibleY := -1
	for i, line := range lines {
		if strings.Contains(line, "Copy error") && strings.Contains(line, "Retry failed") {
			visibleY = i
			break
		}
	}
	if visibleY != copyButton.Rect.Y {
		t.Fatalf("error buttons rendered at y=%d but region uses y=%d\n%s", visibleY, copyButton.Rect.Y, plain)
	}
	visible := ansi.Cut(lines[copyButton.Rect.Y], copyButton.Rect.X, copyButton.Rect.X+copyButton.Rect.W)
	if !strings.Contains(visible, "Copy error") {
		t.Fatalf("copy region misses visible control: %q rect=%+v\n%s", visible, copyButton.Rect, plain)
	}
	r, _ = m.Update(tea.MouseMsg{X: copyButton.Rect.X, Y: copyButton.Rect.Y, Button: tea.MouseButtonLeft})
	m = asModel(r)
	if !strings.Contains(clipboard.value, "second.mp4") || !strings.Contains(clipboard.value, "access denied") || m.clipboardNotice == "" {
		t.Fatalf("clipboard=%q notice=%q", clipboard.value, m.clipboardNotice)
	}
	r, _ = m.Update(tea.MouseMsg{X: backButton.Rect.X, Y: backButton.Rect.Y, Button: tea.MouseButtonLeft})
	if asModel(r).errorListOpen {
		t.Fatal("Back to result did not close failure list")
	}
}
