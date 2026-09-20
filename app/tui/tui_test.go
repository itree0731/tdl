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
	// exec runs on a goroutine started by startRun: the argv must cross
	// goroutines through a channel, not a shared variable
	argvCh := make(chan []string, 1)
	exec := func(ctx context.Context, argv []string, out io.Writer) error {
		select {
		case argvCh <- argv:
		default:
		}
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
	// exec runs in a goroutine: wait for it to hand over the argv
	var gotArgv []string
	select {
	case gotArgv = <-argvCh:
	case <-time.After(2 * time.Second):
		t.Fatal("exec was not invoked within 2s")
	}
	if gotArgv[0] != "dl" {
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

func asModel(tm tea.Model) model {
	switch v := tm.(type) {
	case model:
		return v
	case *model:
		return *v
	}
	panic("unexpected model type")
}

func indexOfAction(m model, id string) int {
	for i := range m.actions {
		if m.actions[i].id == id {
			return i
		}
	}
	panic("action not found: " + id)
}

// An exec that ignores cancellation must never trap the user: the first
// ctrl+c stops the run, the second force quits the program.
func TestCtrlCForceQuitWhileRunning(t *testing.T) {
	cancelled := make(chan struct{})
	exec := func(ctx context.Context, argv []string, out io.Writer) error {
		<-ctx.Done()
		close(cancelled)
		return ctx.Err()
	}

	m := newModel(exec, nil)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m2 := asModel(mm)

	ix := indexOfAction(m2, "dl")
	m2.actions[ix].fields[0].ti.SetValue("https://t.me/a/1")
	mm2, _ := m2.launchAction(&m2.actions[ix])
	m3 := asModel(mm2)
	if !m3.running {
		t.Fatal("run did not start")
	}

	r1, cmd1 := m3.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd1 != nil {
		t.Fatal("first ctrl+c should stop the run, not quit")
	}
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("first ctrl+c did not cancel the exec context")
	}

	// running is still true: runDoneMsg for a cancelled exec may never
	// arrive. The second press must quit regardless.
	_, cmd2 := r1.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd2 == nil {
		t.Fatal("second ctrl+c must force quit")
	}
}

// login prompts on the console the TUI owns; it must not launch in-process.
func TestLoginActionGivesGuidance(t *testing.T) {
	exec := func(ctx context.Context, argv []string, out io.Writer) error {
		t.Error("login must not be executed in-process")
		return nil
	}

	m := newModel(exec, nil)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m2 := asModel(mm)

	r, _ := m2.openMenuItem(indexOfAction(m2, "login"))
	m3 := asModel(r)

	if m3.running {
		t.Fatal("login launched in-process")
	}
	if !m3.showOutput {
		t.Fatal("guidance should show in the output pane")
	}
	last := m3.scrollback[len(m3.scrollback)-1]
	if !strings.Contains(last, "tdl login") {
		t.Fatalf("guidance line = %q", last)
	}
}
func TestExtraFieldFocusDoesNotPanic(t *testing.T) {
	m := newModel(stubExec, nil)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = asModel(mm)
	m.menuIx = indexOfAction(m, "dl")
	mm, _ = m.openMenuItem(m.menuIx)
	m = asModel(mm)

	for i := 0; i < len(m.form.fields)-1; i++ {
		mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = asModel(mm)
	}
	if m.formIx != len(m.form.fields)-1 {
		t.Fatalf("formIx = %d, want %d", m.formIx, len(m.form.fields)-1)
	}
	if !m.form.fields[m.formIx].ti.Focused() {
		t.Fatal("extra args field is not focused")
	}
}

// Header account chips: click one to switch the namespace every command
// runs under; inert while a command is executing.
func TestAccountChipSwitch(t *testing.T) {
	exec := func(ctx context.Context, argv []string, out io.Writer) error { return nil }
	m := newModel(exec, []string{"default", "work"})
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m2 := asModel(mm)

	if m2.currentNS() != "default" {
		t.Fatalf("currentNS = %q", m2.currentNS())
	}
	chips := m2.nsChipLayout(m2.width)
	if len(chips) != 2 {
		t.Fatalf("chips = %+v", chips)
	}
	if chips[0].x0 >= chips[0].x1 || chips[1].x0 != chips[0].x1+2 || chips[1].x1 > m2.width {
		t.Fatalf("chip layout wrong: %+v", chips)
	}

	// click the second chip: switch, apply to argv, persist
	x := (chips[1].x0 + chips[1].x1) / 2
	r, _ := m2.Update(tea.MouseMsg{X: x, Y: 0, Button: tea.MouseButtonLeft})
	m3 := asModel(r)
	if m3.currentNS() != "work" {
		t.Fatalf("after click currentNS = %q", m3.currentNS())
	}
	if got := m3.set.globalArgs(); !containsStr(got, "--ns") || !containsStr(got, "work") {
		t.Fatalf("globalArgs = %v", got)
	}
	if s := loadSettings(); s.NS != "work" {
		t.Fatalf("persisted NS = %q", s.NS)
	}
	_ = saveSettings(settings{Language: "en"}) // restore for other tests

	// clicking the chip of an account again keeps it
	r2, _ := m3.Update(tea.MouseMsg{X: x, Y: 0, Button: tea.MouseButtonLeft})
	m4 := asModel(r2)
	if m4.currentNS() != "work" {
		t.Fatalf("re-click changed NS to %q", m4.currentNS())
	}

	// while running, chips must not switch accounts mid-run
	m4.running = true
	r3, _ := m4.Update(tea.MouseMsg{X: chips[0].x0, Y: 0, Button: tea.MouseButtonLeft})
	if asModel(r3).currentNS() != "work" {
		t.Fatal("switched namespace while running")
	}
}
