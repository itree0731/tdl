package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m model) errorListWindow() (first, count int) {
	count = max(1, m.mainHeight()-8)
	if len(m.runResult.Items) <= count {
		return 0, len(m.runResult.Items)
	}
	first = max(0, min(m.errorCursor-count/2, len(m.runResult.Items)-count))
	return first, count
}

func (m model) viewErrorList() string {
	width := m.contentWidth()
	rows := []string{
		stTitle.Render(m.lang.t("errors.title")) + "  " + stHint.Render(fmt.Sprintf(m.lang.t("errors.count"), len(m.runResult.Items))),
		stHint.Render(strings.Repeat("─", max(1, width-4))),
		stHint.Render(padCell(m.lang.t("errors.file"), max(16, width-42)) + "  " + padCell(m.lang.t("errors.phase"), 18) + "  " + m.lang.t("errors.retry")),
	}
	first, count := m.errorListWindow()
	for i := first; i < first+count; i++ {
		item := m.runResult.Items[i]
		nameW := max(16, width-42)
		row := padCell(ansi.Truncate(item.DisplayName, nameW, "…"), nameW) + "  " + padCell(item.Phase, 18) + "  " + retryClassLabel(m.lang, item.Retry)
		if i == m.errorCursor {
			row = stItemSelected.Width(max(1, width-4)).Render("▌ " + row)
		} else {
			row = activeTheme.text.Render("  " + row)
		}
		rows = append(rows, row)
	}
	for len(rows) < m.mainHeight()-5 {
		rows = append(rows, "")
	}
	if len(m.runResult.Items) > 0 {
		item := m.runResult.Items[m.errorCursor]
		rows = append(rows, stErr.Render(ansi.Truncate(item.Err, max(1, width-4), "…")))
	}
	rows = append(rows, stHint.Render(m.clipboardNotice))
	buttons := stFieldFocus.Render("[ "+m.lang.t("errors.copy")+" ]") + "  " + stHint.Render("[ "+m.lang.t("errors.retry.all")+" ]  [ "+m.lang.t("errors.back")+" ]")
	rows = append(rows, buttons)
	return lipgloss.NewStyle().Width(width).Height(m.mainHeight()).Padding(0, 1).Render(fitScreen(strings.Join(rows, "\n"), width, m.mainHeight()))
}

func retryClassLabel(lang Lang, class RetryClass) string {
	key := "errors.retry.restart"
	switch class {
	case RetrySafe:
		key = "errors.retry.safe"
	case RetryCleanupOnly:
		key = "errors.retry.cleanup"
	case RetryUncertain:
		key = "errors.retry.uncertain"
	case RetryNotAllowed:
		key = "errors.retry.blocked"
	}
	return lang.t(key)
}

func (m model) errorListRegions() []HitRegion {
	if !m.errorListOpen {
		return nil
	}
	x := m.sidebarWidth()
	first, count := m.errorListWindow()
	regions := make([]HitRegion, 0, count+3)
	for i := first; i < first+count; i++ {
		regions = append(regions, HitRegion{ID: fmt.Sprintf("errors.row:%d", i), Rect: Rect{X: x, Y: m.contentTop() + 3 + i - first, W: m.contentWidth(), H: 1}, Enabled: true, Action: UIAction{Kind: UIActionError, Index: i}})
	}
	y := m.contentTop() + m.mainHeight() - 3
	xPos := x + 1
	buttons := []struct{ id, label string }{
		{"errors.copy", m.lang.t("errors.copy")},
		{"errors.retry", m.lang.t("errors.retry.all")},
		{"errors.back", m.lang.t("errors.back")},
	}
	for _, button := range buttons {
		label := "[ " + button.label + " ]"
		w := lipgloss.Width(label)
		regions = append(regions, HitRegion{ID: button.id, Rect: Rect{X: xPos, Y: y, W: w, H: 1}, Enabled: true, Action: UIAction{Kind: UIActionButton, ID: button.id}})
		xPos += w + 2
	}
	return regions
}
