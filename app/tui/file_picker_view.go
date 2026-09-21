package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m model) pickerWindow() (first, count int) {
	if m.picker == nil {
		return 0, 0
	}
	count = max(1, m.mainHeight()-7)
	if len(m.picker.entries) <= count {
		return 0, len(m.picker.entries)
	}
	first = m.picker.cursor - count/2
	if first < 0 {
		first = 0
	}
	if maxFirst := len(m.picker.entries) - count; first > maxFirst {
		first = maxFirst
	}
	return first, count
}

func (m model) viewFilePicker() string {
	if m.picker == nil {
		return ""
	}
	width := m.contentWidth()
	p := m.picker
	rows := []string{
		stTitle.Render(m.lang.t("picker.title")) + "  " + stHint.Render(m.lang.t("picker.purpose")+": ") + stFieldFocus.Render(p.request.Purpose),
		stSys.Render(ansi.Truncate(p.cwd, max(1, width-2), "…")),
	}
	if width >= 64 {
		nameW := max(16, width-39)
		rows = append(rows, stHint.Render(padCell(m.lang.t("picker.name"), nameW)+"  "+padCell(m.lang.t("picker.modified"), 16)+"  "+padCell(m.lang.t("picker.size"), 9)+"  "+m.lang.t("picker.type")))
	} else {
		rows = append(rows, stHint.Render(m.lang.t("picker.name")))
	}
	first, count := m.pickerWindow()
	for i := first; i < first+count; i++ {
		e := p.entries[i]
		mark := "  "
		if p.selected[e.Path] {
			mark = "● "
		}
		name := e.Name
		if e.Dir {
			name += string(filepathSeparator())
		}
		var row string
		if width >= 64 {
			nameW := max(16, width-39)
			size := formatBytes(e.Size)
			if e.Dir {
				size = "—"
			}
			row = mark + padCell(ansi.Truncate(name, nameW-2, "…"), nameW) + "  " + padCell(e.Modified.Format("2006-01-02 15:04"), 16) + "  " + padCell(size, 9) + "  " + e.Type
		} else {
			size := formatBytes(e.Size)
			if e.Dir {
				size = "—"
			}
			row = mark + ansi.Truncate(name, max(1, width-13), "…") + "  " + size
		}
		if i == p.cursor {
			row = stItemSelected.Width(max(1, width-2)).Render("▌" + row)
		} else if p.selected[e.Path] {
			row = stFieldFocus.Render(row)
		} else {
			row = stItem.Render(row)
		}
		rows = append(rows, row)
	}
	for len(rows) < m.mainHeight()-2 {
		rows = append(rows, "")
	}
	if p.err != "" {
		rows = append(rows, stErr.Render(ansi.Truncate(p.err, max(1, width-2), "…")))
	} else {
		rows = append(rows, stHint.Render(m.lang.t("picker.selected")+" ")+stSys.Render(p.summary()))
	}
	buttons := stFieldFocus.Render("[ "+m.lang.t("picker.confirm")+" ]") + "  " + stHint.Render("[ "+m.lang.t("picker.parent")+" ]  [ "+m.lang.t("picker.cancel")+" ]")
	rows = append(rows, buttons)
	return lipgloss.NewStyle().Width(width).Height(m.mainHeight()).Padding(0, 1).Render(fitScreen(strings.Join(rows, "\n"), width, m.mainHeight()))
}

func padCell(value string, width int) string {
	value = ansi.Truncate(value, max(0, width), "…")
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}

func filepathSeparator() rune {
	return '/'
}

func (m model) pickerRegions() []HitRegion {
	if m.picker == nil {
		return nil
	}
	x := m.sidebarWidth()
	first, count := m.pickerWindow()
	regions := make([]HitRegion, 0, count+3)
	for i := first; i < first+count; i++ {
		regions = append(regions, HitRegion{
			ID: fmt.Sprintf("picker:row:%d", i), Rect: Rect{X: x, Y: m.contentTop() + 3 + i - first, W: m.contentWidth(), H: 1}, Enabled: true,
			Action: UIAction{Kind: UIActionPicker, Index: i},
		})
	}
	y := m.contentTop() + m.mainHeight() - 1
	regions = append(regions,
		HitRegion{ID: "picker.confirm", Rect: Rect{X: x + 1, Y: y, W: 12, H: 1}, Enabled: true, Action: UIAction{Kind: UIActionButton, ID: "picker.confirm"}},
		HitRegion{ID: "picker.parent", Rect: Rect{X: x + 14, Y: y, W: 12, H: 1}, Enabled: true, Action: UIAction{Kind: UIActionButton, ID: "picker.parent"}},
		HitRegion{ID: "picker.cancel", Rect: Rect{X: x + 27, Y: y, W: 12, H: 1}, Enabled: true, Action: UIAction{Kind: UIActionButton, ID: "picker.cancel"}},
	)
	return regions
}
