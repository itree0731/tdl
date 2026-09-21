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
	m.menuIx = 2 // keep the first click off the pre-selected item

	m = click(t, m, 5, 4) // Login
	if m.menuIx != 0 {
		t.Errorf("click on row 4 selected %d, want 0 (Login)", m.menuIx)
	}
	m = click(t, m, 5, 7) // Chat: List
	if m.menuIx != 3 {
		t.Errorf("click on row 7 selected %d, want 3 (Chat: List)", m.menuIx)
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

func TestEnterOnBoolFieldRuns(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, nil), 90, 24)
	m = key(t, m, tea.KeyMsg{Type: tea.KeyDown}) // Download
	m = key(t, m, tea.KeyMsg{Type: tea.KeyEnter})
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
