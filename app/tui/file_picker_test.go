package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestNaturalCompare(t *testing.T) {
	items := []pickerEntry{{Name: "file10.mp4"}, {Name: "File2.mp4"}, {Name: "file1.mp4"}}
	p := filePicker{entries: items}
	p.sortEntries()
	got := []string{p.entries[0].Name, p.entries[1].Name, p.entries[2].Name}
	want := []string{"file1.mp4", "File2.mp4", "file10.mp4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("natural order = %v, want %v", got, want)
	}
}

func TestPickerKeepsDirectoriesFirstInDescendingOrder(t *testing.T) {
	p := filePicker{desc: true, sortBy: sortSize, entries: []pickerEntry{
		{Name: "small", Size: 1}, {Name: "dir-a", Dir: true}, {Name: "large", Size: 10}, {Name: "dir-b", Dir: true},
	}}
	p.sortEntries()
	if !p.entries[0].Dir || !p.entries[1].Dir || p.entries[2].Name != "large" {
		t.Fatalf("descending order lost directory priority: %+v", p.entries)
	}
}

func TestSelectionPlanRecursesFiltersAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "media")
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(path string, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	video := filepath.Join(dir, "clip 2.mp4")
	write(video, "video")
	write(filepath.Join(dir, "nested", "clip10.mp4"), "longer-video")
	write(filepath.Join(dir, "note.txt"), "ignored")
	write(filepath.Join(dir, ".hidden.mp4"), "hidden")

	req := PickerRequest{Mode: PickFilesAndDirectories, Recursive: true, AllowedExt: []string{".mp4"}}
	plan := buildSelectionPlan(req, []string{dir, video})
	if len(plan.Problems) != 0 {
		t.Fatalf("unexpected scan problems: %+v", plan.Problems)
	}
	if len(plan.Files) != 2 {
		t.Fatalf("files = %+v", plan.Files)
	}
	if plan.ExcludedHidden != 1 {
		t.Fatalf("excluded hidden = %d", plan.ExcludedHidden)
	}
	if plan.TotalBytes != int64(len("video")+len("longer-video")) {
		t.Fatalf("total bytes = %d", plan.TotalBytes)
	}
}

func TestPickerDirectoryAndFileMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "folder2"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "file10.txt")
	if err := os.WriteFile(path, []byte("ten"), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}
	p, err := newFilePicker(PickerRequest{Mode: PickFilesAndDirectories, InitialDir: root, Multi: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.entries) != 2 || !p.entries[0].Dir || p.entries[1].Size != 3 {
		t.Fatalf("entries = %+v", p.entries)
	}
}

func TestFrameHitTestUsesTopmostEnabledRegion(t *testing.T) {
	frame := Frame{Regions: []HitRegion{
		{ID: "base", Rect: Rect{X: 0, Y: 0, W: 10, H: 2}, Enabled: true, Action: UIAction{Kind: UIActionMenu, Index: 1}},
		{ID: "disabled", Rect: Rect{X: 1, Y: 0, W: 3, H: 1}, Enabled: false, Action: UIAction{Kind: UIActionButton, ID: "bad"}},
		{ID: "top", Rect: Rect{X: 2, Y: 0, W: 3, H: 1}, Enabled: true, Action: UIAction{Kind: UIActionButton, ID: "ok"}},
	}}
	if got, ok := HitTest(frame, 2, 0); !ok || got.ID != "ok" {
		t.Fatalf("top hit = %+v, %v", got, ok)
	}
	if got, ok := HitTest(frame, 1, 0); !ok || got.Kind != UIActionMenu {
		t.Fatalf("disabled region blocked base: %+v, %v", got, ok)
	}
}

func TestUploadPickerProducesExactRepeatedPathArgs(t *testing.T) {
	isolateSettings(t)
	root := t.TempDir()
	dir := filepath.Join(root, "folder, with comma")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"clip2.mp4", "clip10.mp4"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	argvCh := make(chan []string, 1)
	exec := func(_ context.Context, argv []string, _ io.Writer) error {
		argvCh <- append([]string(nil), argv...)
		return nil
	}
	m := sized(t, newModel(exec, nil), 120, 30)
	r, _ := m.openMenuItem(indexOfAction(m, "up"))
	m = asModel(r)
	m.form.fields[0].picker.InitialDir = root
	r, _ = m.openFilePicker(0)
	m = asModel(r)
	if m.state() != screenFilePicker {
		t.Fatalf("state = %v", m.state())
	}
	for i, entry := range m.picker.entries {
		if entry.Path == dir {
			m.picker.cursor = i
			m.picker.toggleCurrent()
		}
	}
	var scanCmd tea.Cmd
	r, scanCmd = m.closeFilePicker(true)
	m = asModel(r)
	if m.picker == nil || m.picker.scanner == nil || scanCmd == nil {
		t.Fatal("recursive confirmation did not start background scan")
	}
	deliverSelectionScan(t, &m, scanCmd)
	if m.state() != stateForm || len(m.form.fields[0].paths) != 2 {
		t.Fatalf("selection was not expanded: %+v", m.form.fields[0].paths)
	}
	r, _ = m.launchForm()
	m = asModel(r)
	if !m.running {
		t.Fatal("upload did not start")
	}
	var executed []string
	select {
	case executed = <-argvCh:
	case <-time.After(time.Second):
		t.Fatal("executor did not receive arguments")
	}
	var paths []string
	for i := 0; i+1 < len(executed); i++ {
		if executed[i] == "-p" {
			paths = append(paths, executed[i+1])
		}
	}
	if !reflect.DeepEqual(paths, m.form.fields[0].paths) {
		t.Fatalf("-p args = %v, want %v; argv=%v", paths, m.form.fields[0].paths, executed)
	}
}

func deliverSelectionScan(t *testing.T, m *model, cmd tea.Cmd) {
	t.Helper()
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("scan command returned %T, want tea.BatchMsg", msg)
	}
	for _, sub := range batch {
		if sub == nil {
			continue
		}
		candidate := sub()
		if _, ok := candidate.(selectionScanMsg); !ok {
			continue
		}
		r, _ := m.Update(candidate)
		*m = asModel(r)
		return
	}
	t.Fatal("background scan produced no completion message")
}

func TestRecursiveScanCanBeCanceledWithoutApplyingSelection(t *testing.T) {
	isolateSettings(t)
	root := t.TempDir()
	dir := filepath.Join(root, "many")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%02d.bin", i)), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m := sized(t, newModel(stubExec, nil), 120, 30)
	r, _ := m.openMenuItem(indexOfAction(m, "up"))
	m = asModel(r)
	m.form.fields[0].picker.InitialDir = root
	r, _ = m.openFilePicker(0)
	m = asModel(r)
	for i, entry := range m.picker.entries {
		if entry.Path == dir {
			m.picker.cursor = i
			m.picker.toggleCurrent()
		}
	}
	r, scanCmd := m.closeFilePicker(true)
	m = asModel(r)
	if m.picker.scanner == nil || scanCmd == nil {
		t.Fatal("scan did not start")
	}
	r, _ = m.closeFilePicker(false)
	m = asModel(r)
	if m.picker == nil || m.picker.scanner != nil || m.form.fields[0].value() != "" {
		t.Fatalf("cancel applied or closed selection: picker=%v value=%q", m.picker != nil, m.form.fields[0].value())
	}
	deliverSelectionScanIgnored(t, &m, scanCmd)
	if m.picker == nil || m.form.fields[0].value() != "" {
		t.Fatal("late canceled scan result polluted form")
	}
}

func TestScanProblemsRequireExplicitSkipBeforeApplying(t *testing.T) {
	isolateSettings(t)
	file := filepath.Join(t.TempDir(), "ok.mp4")
	if err := os.WriteFile(file, []byte("ok"), 0600); err != nil {
		t.Fatal(err)
	}
	m := sized(t, newModel(stubExec, nil), 120, 30)
	r, _ := m.openMenuItem(indexOfAction(m, "up"))
	m = asModel(r)
	m.picker = &filePicker{request: PickerRequest{Mode: PickFilesAndDirectories, Purpose: "upload_paths"}, cwd: filepath.Dir(file), selected: map[string]bool{}, pendingPaths: []string{file}}
	m.pickerField = 0
	plan := SelectionPlan{Paths: []string{file}, Files: []SelectedFile{{Path: file, Size: 2}}, TotalBytes: 2, Problems: []PathProblem{{Path: "denied", Err: "access denied"}}}
	r, _ = m.applyFilePickerPlan(plan)
	m = asModel(r)
	if m.picker == nil || m.picker.problemPlan == nil || len(m.form.fields[0].paths) != 0 {
		t.Fatal("problem plan was applied without confirmation")
	}
	frame := m.frame()
	var skip *HitRegion
	for i := range frame.Regions {
		if frame.Regions[i].ID == "picker.problem.skip" {
			skip = &frame.Regions[i]
			break
		}
	}
	if skip == nil {
		t.Fatal("problem page has no Skip unreadable control")
	}
	r, _ = m.Update(tea.MouseMsg{X: skip.Rect.X, Y: skip.Rect.Y, Button: tea.MouseButtonLeft})
	m = asModel(r)
	if m.picker != nil || len(m.form.fields[0].paths) != 1 || m.form.fields[0].paths[0] != file {
		t.Fatalf("confirmed skip did not apply readable files: %+v", m.form.fields[0].paths)
	}
}

func TestSavePickerEditsAndValidatesFileName(t *testing.T) {
	isolateSettings(t)
	m := sized(t, newModel(stubExec, nil), 120, 30)
	r, _ := m.openMenuItem(indexOfAction(m, "backup"))
	m = asModel(r)
	m.form.fields[0].picker.InitialDir = t.TempDir()
	r, _ = m.openFilePicker(0)
	m = asModel(r)
	m.picker.saveName.SetValue("backup.json")
	r, _ = m.closeFilePicker(true)
	m = asModel(r)
	if m.picker == nil || !strings.Contains(m.picker.err, ".tdl") {
		t.Fatalf("wrong extension accepted: picker=%v err=%q", m.picker != nil, m.picker.err)
	}
	m.picker.saveName.SetValue("nightly")
	r, _ = m.closeFilePicker(true)
	m = asModel(r)
	if m.picker != nil || !strings.HasSuffix(m.form.fields[0].value(), "nightly.tdl") {
		t.Fatalf("save name was not applied with extension: %q", m.form.fields[0].value())
	}
}

func deliverSelectionScanIgnored(t *testing.T, m *model, cmd tea.Cmd) {
	t.Helper()
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return
	}
	for _, sub := range batch {
		if sub == nil {
			continue
		}
		candidate := sub()
		if _, ok := candidate.(selectionScanMsg); ok {
			r, _ := m.Update(candidate)
			*m = asModel(r)
			return
		}
	}
}

func TestFilePickerCanOpenWithEnterAndVisibleMouseButton(t *testing.T) {
	isolateSettings(t)
	for _, size := range []struct{ w, h int }{{120, 30}, {80, 24}} {
		m := sized(t, newModel(stubExec, nil), size.w, size.h)
		r, _ := m.openMenuItem(indexOfAction(m, "up"))
		m = asModel(r)

		r, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		opened := asModel(r)
		if opened.state() != screenFilePicker || opened.running {
			t.Fatalf("%dx%d enter state=%v running=%v", size.w, size.h, opened.state(), opened.running)
		}

		m.picker = nil
		frame := m.frame()
		var region *HitRegion
		for i := range frame.Regions {
			if frame.Regions[i].ID == "picker.open:0" {
				region = &frame.Regions[i]
				break
			}
		}
		if region == nil {
			t.Fatalf("%dx%d has no picker button region", size.w, size.h)
		}
		lines := strings.Split(renderPlain(m), "\n")
		if region.Rect.Y >= len(lines) || !strings.Contains(ansi.Cut(lines[region.Rect.Y], region.Rect.X, region.Rect.X+region.Rect.W), "Select") {
			t.Fatalf("%dx%d hit region does not cover the rendered Select button: rect=%+v", size.w, size.h, region.Rect)
		}
		action, ok := HitTest(frame, region.Rect.X, region.Rect.Y)
		if !ok || action.ID != "picker.open:0" {
			t.Fatalf("%dx%d visible picker button is not hittable: %+v %v", size.w, size.h, action, ok)
		}
		r, _ = m.Update(tea.MouseMsg{X: region.Rect.X, Y: region.Rect.Y, Button: tea.MouseButtonLeft})
		if asModel(r).state() != screenFilePicker {
			t.Fatalf("%dx%d mouse click did not open picker", size.w, size.h)
		}
	}
}

func TestFilePickerHeaderControlsAreClickable(t *testing.T) {
	isolateSettings(t)
	root := t.TempDir()
	for _, name := range []string{"file2.txt", "file10.txt", ".hidden.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m := sized(t, newModel(stubExec, nil), 120, 30)
	r, _ := m.openMenuItem(indexOfAction(m, "up"))
	m = asModel(r)
	m.form.fields[0].picker.InitialDir = root
	r, _ = m.openFilePicker(0)
	m = asModel(r)
	if len(m.picker.entries) != 2 {
		t.Fatalf("hidden file visible by default: %+v", m.picker.entries)
	}
	clickRegion := func(id string) {
		t.Helper()
		frame := m.frame()
		for _, region := range frame.Regions {
			if region.ID == id {
				r, _ := m.Update(tea.MouseMsg{X: region.Rect.X, Y: region.Rect.Y, Button: tea.MouseButtonLeft})
				m = asModel(r)
				return
			}
		}
		t.Fatalf("missing region %q", id)
	}
	clickRegion("picker.hidden")
	if !m.picker.request.ShowHidden || len(m.picker.entries) != 3 {
		t.Fatalf("hidden toggle failed: show=%v entries=%d", m.picker.request.ShowHidden, len(m.picker.entries))
	}
	clickRegion("picker.sort.name")
	if !m.picker.desc || m.picker.entries[0].Name != "file10.txt" {
		t.Fatalf("name sort toggle failed: desc=%v entries=%+v", m.picker.desc, m.picker.entries)
	}
}
