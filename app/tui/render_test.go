package tui

import (
	"context"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/iyear/tdl/pkg/progress"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

func stubExec(ctx context.Context, argv []string, out io.Writer) error {
	_, _ = io.WriteString(out, "stub output\n")
	return nil
}

func renderPlain(m model) string {
	v := ansiRe.ReplaceAllString(m.View(), "")
	var lines []string
	for _, l := range strings.Split(v, "\n") {
		lines = append(lines, strings.TrimRight(l, " "))
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func sized(t *testing.T, m model, w, h int) model {
	t.Helper()
	mm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return mm.(model)
}

func key(t *testing.T, m model, k tea.KeyMsg) model {
	t.Helper()
	mm, _ := m.Update(k)
	return mm.(model)
}

func TestRenderMenuSnapshot(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, nil), 90, 24)
	out := renderPlain(m)
	t.Log("\n" + out)

	for _, want := range []string{"tdl", "Download", "Login", "Settings", "Quit", "enter"} {
		if !strings.Contains(out, want) {
			t.Errorf("menu render missing %q", want)
		}
	}
}

func TestRenderFormSnapshot(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, nil), 90, 24)
	m = key(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = key(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	out := renderPlain(m)
	t.Log("\n" + out)

	for _, want := range []string{"Message URLs", "Output dir", "Grouped media", "Extra args", "$ tdl dl"} {
		if !strings.Contains(out, want) {
			t.Errorf("form render missing %q", want)
		}
	}
}

func TestRenderRunSnapshot(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, nil), 90, 24)
	m = key(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = key(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = key(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("https://t.me/a/1")})
	m = key(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !m.running {
		time.Sleep(5 * time.Millisecond)
	}
	if !m.running {
		t.Fatal("command did not start")
	}

	m = key(t, m, tea.KeyMsg{Type: tea.KeyCtrlC}) // stop it
	mm, _ := m.Update(runDoneMsg{runID: m.runID, err: nil, elapsed: 1500 * time.Millisecond})
	m = mm.(model)

	out := renderPlain(m)
	t.Log("\n" + out)

	if !strings.Contains(out, "Transfer complete") {
		t.Errorf("result card missing completion state:\n%s", out)
	}
	if strings.Contains(out, "CPU:") || strings.Contains(out, "100%") {
		t.Errorf("completed result retained live telemetry:\n%s", out)
	}
}

func TestRenderSettingsZhSnapshot(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, nil), 90, 24)
	ix := 0
	for i, a := range m.actions {
		if a.id == "settings" {
			ix = i
		}
	}
	for i := 0; i < ix; i++ {
		m = key(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	m = key(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = key(t, m, tea.KeyMsg{Type: tea.KeySpace}) // language: en -> zh
	m = key(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // save from any field

	out := renderPlain(m)
	t.Log("\n" + out)
	for _, want := range []string{"设置", "语言", "命名空间", "代理", "线程数", "并发数"} {
		if !strings.Contains(out, want) {
			t.Errorf("zh settings render missing %q", want)
		}
	}
	_ = saveSettings(settings{Language: string(LangEn)})
}

func click(t *testing.T, m model, x, y int) model {
	t.Helper()
	mm, _ := m.Update(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft})
	return mm.(model)
}

func TestMouseClickSidebarMenu(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, nil), 90, 24)
	m = click(t, m, 5, 5) // Download
	if m.menuIx != 1 || m.state() != stateForm || m.form == nil || m.form.id != "dl" {
		t.Fatalf("single sidebar click did not open Download: index=%d state=%v form=%v", m.menuIx, m.state(), m.form)
	}
}

func TestMouseClickTogglesFormControlsAndFreezesAccount(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, []string{"default", "work"}), 120, 30)
	r, _ := m.openMenuItem(indexOfAction(m, "up"))
	m = asModel(r)
	boolIndex := 6 // Remove after upload.
	first := max(0, m.formIx-max(1, m.mainHeight()-3)+1)
	y := m.contentTop() + 2 + boolIndex - first
	m = click(t, m, m.sidebarWidth()+2, y)
	if !m.form.fields[boolIndex].boolVal {
		t.Fatal("mouse click did not toggle boolean field")
	}
	chips := m.nsChipLayout(m.width)
	if len(chips) < 2 {
		t.Fatal("missing account chips")
	}
	accountY := 1
	m = click(t, m, chips[1].x0, accountY)
	if m.currentNS() != "default" {
		t.Fatal("account changed while a form was open")
	}
}

func TestWordmarkReplacesLegacyLogo(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, nil), 90, 40)
	if m.menuShowsLogo() {
		t.Fatal("legacy logo must stay disabled")
	}
	out := renderPlain(m)
	if !strings.Contains(out, "TDL") || strings.Contains(out, "████") {
		t.Fatalf("compact wordmark missing or old logo remains:\n%s", out)
	}
}

func TestLegacyGradientLogoIsGone(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, nil), 90, 40)
	if out := renderPlain(m); strings.Contains(out, "████████╗") || strings.Contains(out, "grok-build") {
		t.Error("legacy visual identity still renders")
	}
}

func TestNamespacesChip(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, []string{"default", "work"}), 90, 24)
	out := renderPlain(m)
	if !strings.Contains(out, "● default  ○ work") {
		t.Errorf("namespace chips missing:\n%s", out)
	}
	// login desc gains a check hint
	if !strings.Contains(out, "✓ default, work") {
		t.Errorf("login ns hint missing:\n%s", out)
	}
}

func TestResponsiveFramesStayInsideTerminal(t *testing.T) {
	isolateSettings(t)
	for _, size := range []struct{ w, h int }{{120, 30}, {80, 24}, {32, 12}} {
		m := sized(t, newModel(stubExec, []string{"default"}), size.w, size.h)
		assertFrameFits(t, m, size.w, size.h)
		r, _ := m.openMenuItem(indexOfAction(m, "up"))
		m = asModel(r)
		m = sized(t, m, size.w, size.h)
		assertFrameFits(t, m, size.w, size.h)
	}
}

func assertFrameFits(t *testing.T, m model, width, height int) {
	t.Helper()
	out := renderPlain(m)
	lines := strings.Split(out, "\n")
	if len(lines) > height {
		t.Fatalf("%dx%d frame has %d rows:\n%s", width, height, len(lines), out)
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("%dx%d row %d has width %d:\n%s", width, height, i, got, out)
		}
	}
}

type fixturePreview struct{}

func (fixturePreview) Render(string, int, int, ColorProfile) (string, error) {
	return "▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀\n▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀\n▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀\n▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀", nil
}

func TestRenderWideWorkbenchSnapshot(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, []string{"default", "itree"}), 200, 60)
	m.lang = LangZh
	m.running = true
	m.showOutput = true
	m.detailsOpen = true
	m.runLabel = "上传"
	m.runStart = time.Now().Add(-time.Minute)
	m.currentRun = RunSpec{ActionID: "up", Display: RunDisplay{Title: "上传"}, Inputs: []InputRef{{Path: "fixture.mp4", Kind: "file"}}}
	m.mediaPreview = fixturePreview{}
	m.previewText, _ = m.mediaPreview.Render("fixture.mp4", 24, 6, ColorTrue)
	m.progress = progressFixture()
	for _, line := range []string{"$ tdl up -p fixture.mp4", "开始上传 fixture.mp4", "连接到 Telegram 服务器", "已传输 2.96 GiB / 4.32 GiB"} {
		m.appendLine(line)
	}
	out := renderPlain(m)
	t.Log("\n" + out)
	for _, want := range []string{"当前会话", "当前项目", "任务统计", "传输日志", "fixture.mp4", "69%"} {
		if !strings.Contains(out, want) {
			t.Errorf("wide workbench missing %q", want)
		}
	}
	if strings.Count(out, "ctrl+c") != 1 {
		t.Errorf("wide workbench has duplicate stop controls:\n%s", out)
	}
	assertFrameFits(t, m, 200, 60)
}

func TestWideFormFooterButtonsAreVisibleAndClickable(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, nil), 120, 30)
	r, _ := m.openMenuItem(indexOfAction(m, "up"))
	m = asModel(r)
	m.form.fields[0].paths = []string{"fixture.mp4"}
	frame := m.frame()
	plain := renderPlain(m)
	lines := strings.Split(plain, "\n")
	var run *HitRegion
	for i := range frame.Regions {
		if frame.Regions[i].ID == "button:form.run" {
			run = &frame.Regions[i]
			break
		}
	}
	if run == nil || run.Rect.Y >= len(lines) {
		t.Fatalf("missing form Run button region: %+v", run)
	}
	visible := ansi.Cut(lines[run.Rect.Y], run.Rect.X, run.Rect.X+run.Rect.W)
	if !strings.Contains(visible, "Run") {
		t.Fatalf("Run hit region does not cover visible button: %q rect=%+v\n%s", visible, run.Rect, plain)
	}
	r, _ = m.Update(tea.MouseMsg{X: run.Rect.X, Y: run.Rect.Y, Button: tea.MouseButtonLeft})
	if !asModel(r).running {
		t.Fatal("mouse Run button did not launch form")
	}
}

func progressFixture() progress.Snapshot {
	return progress.Snapshot{
		Discovered: 27, Expected: 27, Pending: 3, Running: 4, Succeeded: 20,
		CompletedBytes: 2960 * 1024 * 1024, TotalBytes: 4320 * 1024 * 1024,
		DiscoveryDone: true, CurrentFile: "fixture.mp4", CurrentSourcePath: "fixture.mp4",
		Phase: "transferring", Speed: 38.6 * 1024 * 1024,
	}
}

func TestEnterOnBoolFieldRuns(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, nil), 90, 24)
	m = key(t, m, tea.KeyMsg{Type: tea.KeyDown}) // Download
	m = key(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m.form.fields[0].ti.SetValue("https://t.me/a/1")
	// walk to "Rewrite ext" (field 5, bool)
	for i := 0; i < 5; i++ {
		m = key(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	m = key(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !m.running {
		time.Sleep(5 * time.Millisecond)
	}
	if !m.running || m.state() != stateRun {
		t.Fatalf("enter on bool field did not run: state=%v running=%v", m.state(), m.running)
	}
	// space toggles instead
	m2 := sized(t, newModel(stubExec, nil), 90, 24)
	m2 = key(t, m2, tea.KeyMsg{Type: tea.KeyDown})
	m2 = key(t, m2, tea.KeyMsg{Type: tea.KeyEnter})
	for i := 0; i < 5; i++ {
		m2 = key(t, m2, tea.KeyMsg{Type: tea.KeyDown})
	}
	m2 = key(t, m2, tea.KeyMsg{Type: tea.KeySpace})
	if m2.form.fields[5].boolVal != true || m2.state() != stateForm {
		t.Fatalf("space on bool should toggle without running: %+v", m2.form.fields[5])
	}
}
