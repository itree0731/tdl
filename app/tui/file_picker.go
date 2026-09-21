package tui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type pickerSort uint8

const (
	sortName pickerSort = iota
	sortModified
	sortSize
	sortType
)

type pickerEntry struct {
	Name, Path, Type string
	Size             int64
	Modified         time.Time
	Dir, Link        bool
}

type filePicker struct {
	request       PickerRequest
	cwd           string
	entries       []pickerEntry
	selected      map[string]bool
	cursor        int
	sortBy        pickerSort
	desc          bool
	err           string
	lastClickPath string
	lastClickAt   time.Time
}

func newFilePicker(req PickerRequest) (*filePicker, error) {
	cwd := req.InitialDir
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	p := &filePicker{request: req, cwd: abs, selected: make(map[string]bool)}
	if err := p.refresh(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *filePicker) refresh() error {
	entries, err := os.ReadDir(p.cwd)
	if err != nil {
		p.err = err.Error()
		return err
	}
	p.err = ""
	p.entries = p.entries[:0]
	for _, entry := range entries {
		path := filepath.Join(p.cwd, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		if !p.request.ShowHidden && isHiddenPath(path, info) {
			continue
		}
		link := info.Mode()&os.ModeSymlink != 0
		isDir := info.IsDir() && !link
		typ := strings.TrimPrefix(strings.ToLower(filepath.Ext(entry.Name())), ".")
		if isDir {
			typ = "dir"
		} else if link {
			typ = "link"
		} else if typ == "" {
			typ = "file"
		}
		p.entries = append(p.entries, pickerEntry{
			Name: entry.Name(), Path: path, Type: typ, Size: info.Size(), Modified: info.ModTime(), Dir: isDir, Link: link,
		})
	}
	p.sortEntries()
	if p.cursor >= len(p.entries) {
		p.cursor = max(0, len(p.entries)-1)
	}
	return nil
}

func (p *filePicker) sortEntries() {
	sort.SliceStable(p.entries, func(i, j int) bool {
		a, b := p.entries[i], p.entries[j]
		if a.Dir != b.Dir {
			return a.Dir
		}
		cmp := 0
		switch p.sortBy {
		case sortModified:
			cmp = a.Modified.Compare(b.Modified)
		case sortSize:
			cmp = compareInt64(a.Size, b.Size)
		case sortType:
			cmp = strings.Compare(strings.ToLower(a.Type), strings.ToLower(b.Type))
		default:
			cmp = naturalCompare(a.Name, b.Name)
		}
		if cmp == 0 {
			cmp = strings.Compare(a.Name, b.Name)
		}
		if p.desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareInt64(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func naturalCompare(a, b string) int {
	ra, rb := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	for ia, ib := 0, 0; ia < len(ra) || ib < len(rb); {
		if ia >= len(ra) {
			return -1
		}
		if ib >= len(rb) {
			return 1
		}
		if ra[ia] >= '0' && ra[ia] <= '9' && rb[ib] >= '0' && rb[ib] <= '9' {
			ja, jb := ia, ib
			for ja < len(ra) && ra[ja] >= '0' && ra[ja] <= '9' {
				ja++
			}
			for jb < len(rb) && rb[jb] >= '0' && rb[jb] <= '9' {
				jb++
			}
			na, _ := strconv.ParseUint(string(ra[ia:ja]), 10, 64)
			nb, _ := strconv.ParseUint(string(rb[ib:jb]), 10, 64)
			if na < nb {
				return -1
			}
			if na > nb {
				return 1
			}
			ia, ib = ja, jb
			continue
		}
		if ra[ia] < rb[ib] {
			return -1
		}
		if ra[ia] > rb[ib] {
			return 1
		}
		ia++
		ib++
	}
	return strings.Compare(a, b)
}

func (p *filePicker) allows(entry pickerEntry) bool {
	if entry.Dir {
		return p.request.Mode == PickDirectory || p.request.Mode == PickFilesAndDirectories
	}
	if entry.Link {
		return false
	}
	switch p.request.Mode {
	case PickDirectory:
		return false
	case PickSaveFile:
		return false
	}
	if len(p.request.AllowedExt) == 0 {
		return true
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(entry.Name)), ".")
	for _, allowed := range p.request.AllowedExt {
		if ext == strings.TrimPrefix(strings.ToLower(allowed), ".") {
			return true
		}
	}
	return false
}

func (p *filePicker) toggleCurrent() {
	if len(p.entries) == 0 {
		return
	}
	e := p.entries[p.cursor]
	if !p.allows(e) {
		return
	}
	if !p.request.Multi {
		clear(p.selected)
	}
	p.selected[e.Path] = !p.selected[e.Path]
	if !p.selected[e.Path] {
		delete(p.selected, e.Path)
	}
}

func (p *filePicker) enterCurrent() error {
	if len(p.entries) == 0 || !p.entries[p.cursor].Dir {
		return nil
	}
	p.cwd = p.entries[p.cursor].Path
	p.cursor = 0
	return p.refresh()
}

func (p *filePicker) parent() error {
	parent := filepath.Dir(p.cwd)
	if parent == p.cwd {
		return nil
	}
	p.cwd = parent
	p.cursor = 0
	return p.refresh()
}

func (p *filePicker) selectedPaths() []string {
	paths := make([]string, 0, len(p.selected)+1)
	for path := range p.selected {
		paths = append(paths, path)
	}
	if p.request.Mode == PickDirectory && len(paths) == 0 {
		paths = append(paths, p.cwd)
	}
	sort.Slice(paths, func(i, j int) bool { return naturalCompare(paths[i], paths[j]) < 0 })
	return paths
}

func buildSelectionPlan(req PickerRequest, paths []string) SelectionPlan {
	plan := SelectionPlan{Paths: append([]string(nil), paths...)}
	seen := make(map[string]struct{})
	for _, path := range paths {
		appendSelection(req, path, &plan, seen)
	}
	sort.Slice(plan.Files, func(i, j int) bool { return naturalCompare(plan.Files[i].Path, plan.Files[j].Path) < 0 })
	return plan
}

func appendSelection(req PickerRequest, path string, plan *SelectionPlan, seen map[string]struct{}) {
	abs, err := filepath.Abs(path)
	if err != nil {
		plan.Problems = append(plan.Problems, PathProblem{Path: path, Err: err.Error()})
		return
	}
	info, err := os.Lstat(abs)
	if err != nil {
		plan.Problems = append(plan.Problems, PathProblem{Path: abs, Err: err.Error()})
		return
	}
	if info.Mode()&os.ModeSymlink != 0 {
		plan.Problems = append(plan.Problems, PathProblem{Path: abs, Err: "symbolic links and junctions are not followed"})
		return
	}
	if !info.IsDir() {
		appendFile(req, abs, info, plan, seen)
		return
	}
	if !req.Recursive {
		return
	}
	err = filepath.WalkDir(abs, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			plan.Problems = append(plan.Problems, PathProblem{Path: current, Err: walkErr.Error()})
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if current == abs {
			return nil
		}
		entryInfo, err := entry.Info()
		if err != nil {
			plan.Problems = append(plan.Problems, PathProblem{Path: current, Err: err.Error()})
			return nil
		}
		if entryInfo.Mode()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !req.ShowHidden && isHiddenPath(current, entryInfo) {
			if entry.IsDir() {
				plan.ExcludedHidden++
				return fs.SkipDir
			}
			plan.ExcludedHidden++
			return nil
		}
		if !entry.IsDir() {
			appendFile(req, current, entryInfo, plan, seen)
		}
		return nil
	})
	if err != nil {
		plan.Problems = append(plan.Problems, PathProblem{Path: abs, Err: err.Error()})
	}
}

func appendFile(req PickerRequest, path string, info fs.FileInfo, plan *SelectionPlan, seen map[string]struct{}) {
	if len(req.AllowedExt) > 0 {
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
		allowed := false
		for _, candidate := range req.AllowedExt {
			if ext == strings.TrimPrefix(strings.ToLower(candidate), ".") {
				allowed = true
				break
			}
		}
		if !allowed {
			return
		}
	}
	key := canonicalPath(path)
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	plan.Files = append(plan.Files, SelectedFile{Path: path, Size: info.Size(), ModUnix: info.ModTime().Unix()})
	plan.TotalBytes += info.Size()
}

func canonicalPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}
	abs = filepath.Clean(abs)
	if filepath.Separator == '\\' {
		return strings.ToLower(abs)
	}
	return abs
}

func (p *filePicker) summary() string {
	plan := buildSelectionPlan(p.request, p.selectedPaths())
	return fmt.Sprintf("%d · %s", len(plan.Files), formatBytes(plan.TotalBytes))
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for value := n / unit; value >= unit && exp < 3; value /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGT"[exp])
}
