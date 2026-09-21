package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	xprogress "github.com/iyear/tdl/pkg/progress"
)

func (m model) viewRunWide() string {
	width, height := m.contentWidth(), m.mainHeight()
	topH := min(18, max(12, height*38/100))
	if height < 24 {
		topH = min(12, height)
	}
	detailH := max(0, height-topH-1)
	previewW := min(32, max(24, width/5))
	statsW := min(28, max(24, width/6))
	taskW := max(36, width-previewW-statsW-4)
	if previewW+taskW+statsW+4 > width {
		previewW = 0
		taskW = max(36, width-statsW-2)
	}

	parts := make([]string, 0, 5)
	if previewW > 0 {
		parts = append(parts, m.viewMediaCard(previewW, topH))
		parts = append(parts, "  ")
	}
	parts = append(parts, m.viewCurrentTaskCard(taskW, topH))
	parts = append(parts, "  ", m.viewTaskStatsCard(statsW, topH))
	top := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	if detailH <= 3 {
		return fitScreen(top, width, height)
	}
	details := m.viewRunDetails(width, detailH)
	return fitScreen(lipgloss.JoinVertical(lipgloss.Left, top, "", details), width, height)
}

func workbenchPanel(width, height int, border lipgloss.TerminalColor, content string) string {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(max(1, width-4)).
		Height(max(1, height-2)).
		Render(fitScreen(content, max(1, width-4), max(1, height-2)))
}

func (m model) currentMediaPath() string {
	if m.progress.CurrentSourcePath != "" {
		return m.progress.CurrentSourcePath
	}
	for _, input := range m.currentRun.Inputs {
		if info, err := os.Stat(input.Path); err == nil && !info.IsDir() {
			return input.Path
		}
	}
	return ""
}

func (m model) viewMediaCard(width, height int) string {
	path := m.currentMediaPath()
	body := []string{stHint.Render("MEDIA")}
	previewH := max(3, height-5)
	if path != "" && m.mediaPreview != nil {
		if rendered, err := m.mediaPreview.Render(path, max(8, width-4), previewH, m.colorProfile); err == nil {
			body = append(body, rendered)
		}
	}
	if len(body) == 1 {
		body = append(body, stHint.Render(m.lang.t("preview.waiting")))
	}
	if path != "" {
		name := filepath.Base(path)
		body = append(body, stSys.Render(ansi.Truncate(name, max(8, width-4), "…")))
	}
	return workbenchPanel(width, height, colour(activeTheme.palette.Border), strings.Join(body, "\n"))
}

func (m model) viewCurrentTaskCard(width, height int) string {
	pct, known := m.progress.Percent()
	current := m.progress.CurrentFile
	if current == "" {
		current = m.runLabel
	}
	commandName := m.currentRun.Display.Title
	if commandName == "" {
		commandName = m.runLabel
	}
	barW := max(12, width-12)
	bytesTotal := "--"
	if m.progress.TotalBytes > 0 {
		bytesTotal = xprogress.Bytes(m.progress.TotalBytes)
	}
	body := []string{
		stFieldFocus.Render("↑  "+m.lang.t("run.current")) + stHint.Render("  ·  ") + stFieldFocus.Render(commandName),
		stHint.Render(strings.Repeat("─", max(1, width-6))),
		stTitle.Render(ansi.Truncate(current, max(8, width-4), "…")),
		stHint.Render(m.progress.Phase),
		stFieldFocus.Render(progressBar(barW, pct, known)) + "  " + stFieldFocus.Render(percentLabel(pct, known)),
		stHint.Render(m.lang.t("run.transferred")+"  ") + stSys.Render(xprogress.Bytes(m.progress.CompletedBytes)+" / "+bytesTotal),
		stHint.Render(m.lang.t("run.speed")+"  ") + stSys.Render(xBytesPerSecond(m.progress.Speed)) + stShortcutSep.Render("  │  ") + stHint.Render(m.lang.t("run.eta")+"  ") + stSys.Render(m.progress.ETA()),
	}
	for len(body) < height-4 {
		body = append(body, "")
	}
	body = append(body, stHint.Render("[d] "+m.lang.t("run.details"))+"  "+stStopKey.Render("[ctrl+c] "+m.lang.t("run.stop")))
	return workbenchPanel(width, height, colour(activeTheme.palette.Copper), strings.Join(body, "\n"))
}

func (m model) viewTaskStatsCard(width, height int) string {
	rows := []string{
		stTitle.Render(m.lang.t("run.stats")),
		stHint.Render(strings.Repeat("─", max(1, width-6))),
		statBadge("◴", m.lang.t("run.pending"), m.progress.Pending, activeTheme.warning),
		statBadge("▶", m.lang.t("run.running"), m.progress.Running, activeTheme.data),
		statBadge("●", m.lang.t("run.success"), m.progress.Succeeded, activeTheme.success),
		statBadge("×", m.lang.t("run.failed"), m.progress.Failed, activeTheme.failure),
		statBadge("■", m.lang.t("run.canceled"), m.progress.Canceled, activeTheme.warning),
		statBadge("–", m.lang.t("run.skipped"), m.progress.Skipped, activeTheme.hint),
	}
	return workbenchPanel(width, height, colour(activeTheme.palette.Border), strings.Join(rows, "\n"))
}

func statBadge(icon, label string, value int, style lipgloss.Style) string {
	return style.Render(icon) + "  " + stHint.Render(label) + strings.Repeat(" ", max(1, 12-lipgloss.Width(label))) + style.Bold(true).Render(fmt.Sprintf("%d", value))
}

func (m model) viewRunDetails(width, height int) string {
	headerIcon := "▼"
	if !m.detailsOpen {
		headerIcon = "▶"
	}
	header := stFieldFocus.Render(headerIcon+"  "+m.lang.t("run.details")) + stHint.Render("  ·  "+m.lang.t("run.logs"))
	auto := stHint.Render(m.lang.t("run.autoscroll")+"  ") + stSys.Render("●")
	rows := []string{joinEdges(header, auto, max(1, width-6)), stHint.Render(strings.Repeat("─", max(1, width-6)))}
	if !m.detailsOpen {
		rows = append(rows, stHint.Render(m.lang.t("run.collapsed")))
		return workbenchPanel(width, height, colour(activeTheme.palette.Border), strings.Join(rows, "\n"))
	}
	rows = append(rows, stHint.Render(padCell(m.lang.t("run.time"), 10)+padCell(m.lang.t("run.level"), 10)+m.lang.t("run.message")))
	rows = append(rows, stHint.Render(strings.Repeat("─", max(1, width-6))))
	available := max(1, height-6)
	start := max(0, len(m.scrollback)-available)
	for i := start; i < len(m.scrollback); i++ {
		line := stripANSI(strings.TrimSpace(m.scrollback[i]))
		if line == "" {
			continue
		}
		stamp := "--:--:--"
		if i < len(m.scrollTimes) {
			stamp = m.scrollTimes[i].Format("15:04:05")
		}
		level, style := "INFO", activeTheme.data
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "失败") {
			level, style = "ERROR", activeTheme.failure
		} else if strings.Contains(lower, "warn") || strings.Contains(lower, "取消") {
			level, style = "WARN", activeTheme.warning
		} else if strings.HasPrefix(line, "$") {
			level, style = "CMD", activeTheme.command
		}
		messageW := max(8, width-24)
		rows = append(rows, stHint.Render(padCell(stamp, 10))+style.Render(padCell(level, 10))+activeTheme.text.Render(ansi.Truncate(line, messageW, "…")))
	}
	return workbenchPanel(width, height, colour(activeTheme.palette.Border), strings.Join(rows, "\n"))
}

func stripANSI(value string) string {
	return ansi.Strip(value)
}
