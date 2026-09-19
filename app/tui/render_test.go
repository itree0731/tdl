package tui

import (
	"context"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"
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
	m := sized(t, newModel(stubExec), 90, 24)
	out := renderPlain(m)
	t.Log("\n" + out)

	for _, want := range []string{"tdl", "Download", "Login", "Settings", "Quit", "enter"} {
		if !strings.Contains(out, want) {
			t.Errorf("menu render missing %q", want)
		}
	}
}

func TestRenderFormSnapshot(t *testing.T) {
	m := sized(t, newModel(stubExec), 90, 24)
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
	m := sized(t, newModel(stubExec), 90, 24)
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
	mm, _ := m.Update(runDoneMsg{err: nil, elapsed: 1500 * time.Millisecond})
	m = mm.(model)

	out := renderPlain(m)
	t.Log("\n" + out)

	if !strings.Contains(out, "$ tdl dl -u https://t.me/a/1") {
		t.Errorf("scrollback missing command echo:\n%s", out)
	}
	if !strings.Contains(out, "Done in") {
		t.Errorf("scrollback missing completion banner:\n%s", out)
	}
}

func TestRenderSettingsZhSnapshot(t *testing.T) {
	m := sized(t, newModel(stubExec), 90, 24)
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
	m = key(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // language: en -> zh
	m = key(t, m, tea.KeyMsg{Type: tea.KeyTab})   // to ns field
	m = key(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // save

	out := renderPlain(m)
	t.Log("\n" + out)
	for _, want := range []string{"想做什么", "下载", "设置", "退出"} {
		if !strings.Contains(out, want) {
			t.Errorf("zh menu render missing %q", want)
		}
	}
	_ = saveSettings(settings{Language: string(LangEn)})
}
