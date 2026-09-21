package tui

import (
	"context"
	"errors"
	"fmt"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/iyear/tdl/pkg/consts"
	xp "github.com/iyear/tdl/pkg/progress"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFormReentryUsesFreshFields(t *testing.T) {
	isolateSettings(t)
	m := newModel(stubExec, nil)
	for _, id := range []string{"dl", "up", "forward"} {
		fresh := newActions()
		var expected []string
		for i := range fresh {
			if fresh[i].id == id {
				expected = fresh[i].argv(nil)
			}
		}
		r, _ := m.openMenuItem(indexOfAction(m, id))
		m = asModel(r)
		for i := range m.form.fields {
			f := &m.form.fields[i]
			switch f.kind {
			case kText, kExtra:
				f.ti.SetValue("old-value")
			case kBool:
				f.boolVal = true
			case kChoice:
				f.cycle(true)
			}
		}
		m.toMenu()
		r, _ = m.openMenuItem(indexOfAction(m, id))
		m = asModel(r)
		if !reflect.DeepEqual(m.form.argv(nil), expected) {
			t.Fatalf("%s retained input: %v", id, m.form.argv(nil))
		}
		m.toMenu()
	}
}
func TestSettingsFailureDoesNotCommitOrLoseDraft(t *testing.T) {
	isolateSettings(t)
	m := newModel(stubExec, nil)
	m.openSettings()
	before := m.set
	m.form.fields[3].ti.SetValue("0")
	if m.applySettingsDraft() || m.set != before || m.settingsError == "" {
		t.Fatal("invalid settings committed")
	}
	m.form.fields[3].ti.SetValue("8")
	broken := filepath.Join(t.TempDir(), "not-a-directory")
	os.WriteFile(broken, []byte("x"), 0600)
	consts.DataDir = broken
	if m.applySettingsDraft() || m.set != before || !m.settingsDirty || m.settingsError == "" {
		t.Fatalf("state=%+v", m.set)
	}
}
func TestSettingsModalFreezesAccountsAndSaveLeaves(t *testing.T) {
	isolateSettings(t)
	saveSettings(settings{Language: "en", NS: "default"})
	m := sized(t, newModel(stubExec, []string{"default", "work"}), 120, 30)
	m.openSettings()
	m.form.fields[2].ti.SetValue("socks5://127.0.0.1:1234")
	for _, chip := range m.nsChipLayout(m.width) {
		if chip.ns == "work" {
			m = click(t, m, chip.x0, 0)
		}
	}
	if m.currentNS() != "default" || loadSettings().NS != "default" {
		t.Fatal("account escaped draft")
	}
	m = key(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if !m.settingsPrompt {
		t.Fatal("missing dirty prompt")
	}
	m = key(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if m.state() != stateMenu || loadSettings().Proxy == "" {
		t.Fatal("save did not leave")
	}
}
func TestSettingsApplyCancelAndForcedExit(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, nil), 80, 24)
	m.openSettings()
	m.form.fields[3].ti.SetValue("4")
	if !m.applySettingsDraft() {
		t.Fatal(m.settingsError)
	}
	m.form.fields[3].ti.SetValue("8")
	m.discardSettingsDraft()
	if loadSettings().Threads != "4" {
		t.Fatal("cancel overwrote saved setting")
	}
	m.openSettings()
	m.form.fields[3].ti.SetValue("16")
	m = key(t, m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.settingsPrompt {
		t.Fatal("first quit lost draft prompt")
	}
	r, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	_ = r
	if cmd == nil {
		t.Fatal("modal swallowed ctrl+c")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected forced quit")
	}
}
func TestOldProgressAndCompletionCannotChangeNewRun(t *testing.T) {
	isolateSettings(t)
	m := newModel(stubExec, nil)
	m.runID = 2
	m.running = true
	r, _ := m.Update(progressMsg{runID: 1, event: xp.Event{TaskID: "old", Kind: xp.KindFinished, Status: xp.StatusDone}})
	m = asModel(r)
	r, _ = m.Update(progressSnapshotMsg{runID: 1, snapshot: xp.Snapshot{Succeeded: 99}})
	m = asModel(r)
	if m.progress.Discovered != 0 || m.progress.Succeeded != 0 {
		t.Fatal("old progress accepted")
	}
	m.applyProgress(xp.Event{TaskID: "failed", Kind: xp.KindFinished, Status: xp.StatusFailed, Err: "failed"})
	r, _ = m.Update(runDoneMsg{runID: 2})
	m = asModel(r)
	if m.progress.Status != xp.StatusFailed {
		t.Fatal("nil command error hid item failure")
	}
	m.toMenu()
	if len(m.scrollback) != 0 || m.form != nil || m.progress.Discovered != 0 || m.detailsOpen {
		t.Fatal("page not reset")
	}
}
func TestAllFieldsHaveLocalizedHelpAndPlaceholders(t *testing.T) {
	isolateSettings(t)
	m := newModel(stubExec, nil)
	m.openSettings()
	actions := append(newActions(), *m.form)
	for _, a := range actions {
		for _, f := range a.fields {
			pair, ok := fieldHelp[f.helpKey]
			if !ok || pair[0] == "" || pair[1] == "" {
				t.Errorf("missing help for %s/%s", a.id, f.labelKey)
			}
			if f.label(LangZh) == "" || f.label(LangEn) == "" {
				t.Error("empty label")
			}
		}
	}
	m = sized(t, newModel(stubExec, nil), 120, 30)
	m.lang = LangZh
	r, _ := m.openMenuItem(indexOfAction(m, "up"))
	m = asModel(r)
	view := renderPlain(m)
	if strings.Contains(view, "empty = Saved Messages") || strings.Contains(view, "dirs or files") || !strings.Contains(view, "收藏夹") {
		t.Fatal(view)
	}
	m.form.fields[1].ti.SetValue("empty = Saved Messages")
	if !strings.Contains(renderPlain(m), "empty = Saved Messages") {
		t.Fatal("translated user data")
	}
}
func TestFixedLayoutButtonsAndDetails(t *testing.T) {
	isolateSettings(t)
	for _, size := range [][2]int{{80, 24}, {120, 30}, {32, 12}} {
		m := sized(t, newModel(stubExec, nil), size[0], size[1])
		m.running = true
		m.showOutput = true
		m.runCommand = "tdl dl"
		m.progress = xp.Snapshot{CurrentFile: strings.Repeat("长路径文件名", 40), Discovered: 1, Running: 1}
		m.appendLine("DETAIL_MARKER")
		out := renderPlain(m)
		for _, line := range strings.Split(out, "\n") {
			if lipgloss.Width(line) > m.width {
				t.Fatalf("overwide: %q", line)
			}
		}
		if len(strings.Split(out, "\n")) > m.height {
			t.Fatal("too tall")
		}
		if strings.Contains(out, "DETAIL_MARKER") {
			t.Fatal("details not collapsed")
		}
		for _, b := range m.runButtons() {
			if b.id == "run.details" {
				m = click(t, m, b.x0, b.y)
			}
		}
		if !m.detailsOpen {
			t.Fatal("details button missed")
		}
		stopped := false
		m.cancelRun = func() { stopped = true }
		for _, b := range m.runButtons() {
			if b.id == "run.stop" {
				if !strings.Contains(m.viewStatus(), b.label) {
					t.Fatal("invisible stop")
				}
				m = click(t, m, b.x0, b.y)
			}
		}
		if !stopped {
			t.Fatal("stop missed")
		}
	}
}
func TestCanceledRunIsNotFailure(t *testing.T) {
	isolateSettings(t)
	m := newModel(stubExec, nil)
	m.running = true
	m.runID = 1
	r, _ := m.Update(runDoneMsg{runID: 1, err: errors.Join(errors.New("batch"), context.Canceled)})
	m = asModel(r)
	if m.progress.Status != xp.StatusCanceled {
		t.Fatal(m.progress.Status)
	}
}

func TestDetailsCanReachLastLogLine(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, nil), 80, 24)
	m.running = true
	m.showOutput = true
	m.detailsOpen = true
	m.runStart = time.Now().Add(-3 * time.Second)
	m.progress = xp.Snapshot{CurrentFile: "fixture", Discovered: 1}
	for i := 0; i < 100; i++ {
		m.appendLine(fmt.Sprintf("line %d", i))
	}
	m.appendLine("UNIQUE_LAST_LINE")
	if !strings.Contains(renderPlain(m), "UNIQUE_LAST_LINE") {
		t.Fatal("auto-follow hid the tail")
	}
	m.follow = false
	m.vp.GotoTop()
	for i := 0; i < 100; i++ {
		m = key(t, m, tea.KeyMsg{Type: tea.KeyPgDown})
	}
	if !strings.Contains(renderPlain(m), "UNIQUE_LAST_LINE") {
		t.Fatal("manual scroll hid the tail")
	}
	if strings.Contains(m.runSummaryRows()[1], "运行中 0s") {
		t.Fatal("elapsed did not advance")
	}
}
