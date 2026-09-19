package tui

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"
)

func TestSplitStreamProgressBars(t *testing.T) {
	// go-pretty style redraws: multiple \r segments inside one line
	lines, live, rest := splitStream([]byte("PROG 10%\rPROG 50%\rPROG 100%\ndone\n"))
	if !reflect.DeepEqual(lines, []string{"PROG 100%", "done"}) {
		t.Errorf("lines = %v", lines)
	}
	if live != "" {
		t.Errorf("live = %q", live)
	}
	if len(rest) != 0 {
		t.Errorf("rest = %q", rest)
	}
}

func TestSplitStreamLiveWithoutNewline(t *testing.T) {
	_, live, rest := splitStream([]byte("head\nPROG 10%\rPROG 5"))
	if live != "PROG 5" {
		t.Errorf("live = %q", live)
	}
	if string(rest) != "PROG 5" {
		t.Errorf("rest = %q", rest)
	}
}

func TestSplitStreamPlainLines(t *testing.T) {
	lines, _, rest := splitStream([]byte("a\nb\n\npartial"))
	if !reflect.DeepEqual(lines, []string{"a", "b", ""}) {
		t.Errorf("lines = %v", lines)
	}
	if string(rest) != "partial" {
		t.Errorf("rest = %q", rest)
	}
}

func TestActionArgv(t *testing.T) {
	acts := newActions()
	var dl *action
	for i := range acts {
		if acts[i].id == "dl" {
			dl = &acts[i]
		}
	}
	if dl == nil {
		t.Fatal("dl action not found")
	}

	// simulate: -u two links, dir != default, --group on, rest untouched
	dl.fields[0].ti.SetValue("https://t.me/a/1,https://t.me/b/2")
	dl.fields[2].ti.SetValue("D:\\videos")
	dl.fields[8].boolVal = true // grouped media
	dl.fields[11].ti.SetValue("--takeout")

	got := dl.argv([]string{"--ns", "test"})
	want := []string{
		"--ns", "test", "dl",
		"-u", "https://t.me/a/1,https://t.me/b/2",
		"-d", "D:\\videos",
		"--group",
		"--takeout",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("argv =\n%v\nwant\n%v", got, want)
	}
}

func TestActionArgvDefaultsSkipped(t *testing.T) {
	acts := newActions()
	var ver *action
	for i := range acts {
		if acts[i].id == "version" {
			ver = &acts[i]
		}
	}
	got := ver.argv(nil)
	if !reflect.DeepEqual(got, []string{"version"}) {
		t.Errorf("argv = %v", got)
	}
}

func TestSettingsGlobalArgs(t *testing.T) {
	s := settings{Language: "zh", NS: "work", Proxy: "", Threads: "8", Limit: ""}
	got := s.globalArgs()
	want := []string{"--ns", "work", "--threads", "8"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("globalArgs = %v", got)
	}
}

func TestQuoteJoin(t *testing.T) {
	got := quoteJoin([]string{"chat", "export", "-c", "my chat", "--all"})
	if !strings.Contains(got, `"my chat"`) {
		t.Errorf("spaced arg not quoted: %s", got)
	}
	if strings.HasPrefix(got, " ") {
		t.Errorf("unexpected leading space: %q", got)
	}
}

func TestModelFlow(t *testing.T) {
	var gotArgv []string
	exec := func(ctx context.Context, argv []string, out io.Writer) error {
		gotArgv = argv
		fmt.Fprintln(out, "hello from stub")
		fmt.Fprint(out, "10%\r50%\r99%\r")
		return nil
	}

	m := newModel(exec, nil)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = mm.(model)

	if m.state() != stateMenu {
		t.Fatalf("initial state = %v", m.state())
	}

	// navigate to Download (index 1) and open its form
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(model)
	if m.state() != stateForm {
		t.Fatalf("after enter state = %v", m.state())
	}

	// type a URL into the first field, then Enter on a text field runs it
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("https://t.me/a/1")})
	mm, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(model)
	if m.state() != stateRun || !m.running {
		t.Fatalf("after run state = %v running = %v", m.state(), m.running)
	}
	// exec runs in a goroutine: wait for it to observe the argv
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(gotArgv) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if len(gotArgv) == 0 || gotArgv[0] != "dl" {
		t.Fatalf("exec argv = %v", gotArgv)
	}
	if !containsStr(gotArgv, "https://t.me/a/1") {
		t.Errorf("typed URL missing from argv: %v", gotArgv)
	}

	// stream output + completion the way the runner would
	mm, _ = m.Update(outputLineMsg{text: "hello from stub"})
	mm, _ = mm.Update(liveLineMsg{text: "99%"})
	mm, _ = mm.Update(runDoneMsg{err: nil, elapsed: time.Second})
	m = mm.(model)
	if m.running {
		t.Fatal("still running after runDoneMsg")
	}

	// back to menu
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(model)
	if m.state() != stateMenu {
		t.Fatalf("after done+enter state = %v", m.state())
	}

	// settings: open, switch language to zh, save
	var ix int
	for i, a := range m.actions {
		if a.id == "settings" {
			ix = i
		}
	}
	steps := (ix - m.menuIx + len(m.actions)) % len(m.actions)
	for i := 0; i < steps; i++ {
		mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = mm.(model)
	}
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(model)
	if !m.isSetting {
		t.Fatal("settings form not opened")
	}
	// language is a choice field: space cycles to zh; enter saves from any
	// field (enter is reserved for run/save)
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mm.(model)
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // save
	m = mm.(model)
	m = mm.(model)
	if m.lang != LangZh {
		t.Errorf("lang = %v, want zh", m.lang)
	}
	if m.state() != stateMenu {
		t.Fatalf("after settings save state = %v", m.state())
	}

	// persisted?
	if s := loadSettings(); Lang(s.Language) != LangZh {
		t.Errorf("persisted language = %v", s.Language)
	}
	_ = saveSettings(settings{Language: "en"}) // restore default for other tests
}

func containsStr(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
