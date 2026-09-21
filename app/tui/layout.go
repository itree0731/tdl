package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	xprogress "github.com/iyear/tdl/pkg/progress"
)

// Rect uses half-open terminal cell coordinates. It is the shared geometry
// primitive for rendering and mouse input.
type Rect struct {
	X, Y, W, H int
}

func (r Rect) Contains(x, y int) bool {
	return r.W > 0 && r.H > 0 && x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

type UIActionKind uint8

const (
	UIActionNone UIActionKind = iota
	UIActionMenu
	UIActionAccount
	UIActionField
	UIActionButton
	UIActionPicker
)

type UIAction struct {
	Kind  UIActionKind
	ID    string
	Index int
}

type HitRegion struct {
	ID      string
	Rect    Rect
	Enabled bool
	Action  UIAction
}

type Frame struct {
	Text    string
	Regions []HitRegion
}

func HitTest(frame Frame, x, y int) (UIAction, bool) {
	// Later regions are visually on top and therefore win.
	for i := len(frame.Regions) - 1; i >= 0; i-- {
		r := frame.Regions[i]
		if r.Enabled && r.Rect.Contains(x, y) {
			return r.Action, true
		}
	}
	return UIAction{}, false
}

type layoutMode uint8

const (
	layoutCompact layoutMode = iota
	layoutStandard
	layoutWide
)

func chooseLayout(w, h int) layoutMode {
	if w >= 100 && h >= 28 {
		return layoutWide
	}
	if w >= 70 && h >= 20 {
		return layoutStandard
	}
	return layoutCompact
}

type uiButton struct {
	id, label string
	x0, x1, y int
}

func (b uiButton) contains(x, y int) bool { return y == b.y && x >= b.x0 && x < b.x1 }
func fitScreen(text string, w, h int) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = ansi.Truncate(strings.TrimRight(lines[i], " "), max(1, w), "…")
	}
	if h > 0 && len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}
func (m model) buttons(ids ...string) []uiButton {
	labels := make([]string, len(ids))
	width := 0
	for i, id := range ids {
		labels[i] = "[" + m.lang.t(id) + "]"
		width += lipgloss.Width(labels[i])
		if i > 0 {
			width++
		}
	}
	x := max(0, m.width-width)
	out := make([]uiButton, 0, len(ids))
	for i, id := range ids {
		end := min(m.width, x+lipgloss.Width(labels[i]))
		out = append(out, uiButton{id, labels[i], x, end, m.contentTop() + m.mainHeight()})
		x = end + 1
	}
	return out
}
func (m model) runButtons() []uiButton {
	if chooseLayout(m.width, m.height) == layoutWide && m.running {
		topH := min(18, max(12, m.mainHeight()*38/100))
		if m.mainHeight() < 24 {
			topH = min(12, m.mainHeight())
		}
		previewW := min(32, max(24, m.contentWidth()/5))
		statsW := min(28, max(24, m.contentWidth()/6))
		taskW := max(36, m.contentWidth()-previewW-statsW-4)
		if previewW+taskW+statsW+4 > m.contentWidth() {
			previewW = 0
		}
		x := m.sidebarWidth() + previewW
		if previewW > 0 {
			x += 2
		}
		y := m.contentTop() + topH - 2
		details := "[d] " + m.lang.t("run.details")
		stop := "[ctrl+c] " + m.lang.t("run.stop")
		return []uiButton{
			{id: "run.details", label: details, x0: x + 2, x1: x + 2 + lipgloss.Width(details), y: y},
			{id: "run.stop", label: stop, x0: x + 4 + lipgloss.Width(details), x1: x + 4 + lipgloss.Width(details) + lipgloss.Width(stop), y: y},
		}
	}
	if m.running {
		return m.buttons("run.details", "run.stop")
	}
	if len(m.runResult.Items) > 0 {
		return m.buttons("run.details", "run.retry", "form.back")
	}
	return m.buttons("run.details", "form.back")
}
func (m model) settingsButtons() []uiButton { return m.buttons("set.apply", "set.cancel") }
func (m model) activateButton(id string) (tea.Model, tea.Cmd) {
	if raw, ok := strings.CutPrefix(id, "picker.open:"); ok {
		ix, err := strconv.Atoi(raw)
		if err == nil {
			return m.openFilePicker(ix)
		}
	}
	if raw, ok := strings.CutPrefix(id, "chat.open:"); ok {
		ix, err := strconv.Atoi(raw)
		if err == nil {
			return m.openChatSelector(ix)
		}
	}
	switch id {
	case "run.stop":
		return m.stopRun()
	case "run.details":
		m.detailsOpen = !m.detailsOpen
	case "run.retry":
		return m.retryFailed(false)
	case "form.back":
		if !m.running {
			m.toMenu()
		}
	case "set.apply":
		m.applySettingsDraft()
	case "set.cancel":
		m.discardSettingsDraft()
	case "picker.confirm":
		return m.closeFilePicker(true)
	case "picker.cancel":
		return m.closeFilePicker(false)
	case "picker.parent":
		if m.picker != nil {
			_ = m.picker.parent()
		}
	}
	return m, nil
}

func (m model) viewRetryConfirm() string {
	width := m.contentWidth()
	rows := []string{
		activeTheme.warning.Bold(true).Render(m.lang.t("retry.uncertain.title")),
		stHint.Render(m.lang.t("retry.uncertain.body")),
		"",
		stFieldFocus.Render("[ y / enter ] "+m.lang.t("retry.anyway")) + "  " + stHint.Render("[ n / esc ] "+m.lang.t("picker.cancel")),
	}
	card := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(colour(activeTheme.palette.Warning)).Padding(1, 2).Width(max(28, min(width-6, 70))).Render(strings.Join(rows, "\n"))
	return lipgloss.Place(width, m.mainHeight(), lipgloss.Center, lipgloss.Center, card)
}
func (m model) resultLabel() string {
	if m.running {
		if m.stopReq {
			return m.lang.t("status.stopping")
		}
		return m.lang.t("status.running")
	}
	switch m.progress.Status {
	case xprogress.StatusCanceled:
		return m.lang.t("status.canceled")
	case xprogress.StatusPartial:
		return m.lang.t("status.partial")
	case xprogress.StatusFailed:
		return m.lang.t("status.failed")
	}
	return m.lang.t("status.done")
}

func progressBar(width int, pct float64, known bool) string {
	if width < 1 {
		return ""
	}
	if !known {
		if width < 4 {
			return strings.Repeat("·", width)
		}
		return "◆" + strings.Repeat("·", width-1)
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(float64(width)*pct/100 + 0.5)
	return strings.Repeat("━", filled) + strings.Repeat("─", width-filled)
}

func statLine(label string, value int, style lipgloss.Style) string {
	return stHint.Render(label) + "  " + style.Render(fmt.Sprintf("%d", value))
}
func (m model) runSummaryRows() []string {
	elapsed := m.runLast
	if m.running {
		elapsed = time.Since(m.runStart)
	}
	rows := []string{m.runCommand, m.resultLabel() + " " + elapsed.Truncate(time.Millisecond).String(), m.progress.Summary(m.lang == LangZh)}
	if m.progress.CurrentFile != "" {
		rows = append(rows, m.progress.CurrentFile)
	}
	if m.progress.Discovered > 0 {
		metrics := m.progress.Metrics()
		if m.lang == LangZh {
			metrics = strings.Replace(metrics, "ETA", "剩余", 1)
		}
		rows = append(rows, metrics)
	}
	if m.progress.ProcessInfo != "" {
		info := m.progress.ProcessInfo
		if m.lang == LangZh {
			info = strings.ReplaceAll(strings.ReplaceAll(info, "Memory", "内存"), "Goroutines", "协程")
		}
		rows = append(rows, info)
	}
	if len(m.progress.Errors) > 0 {
		rows = append(rows, stErr.Render(strings.ReplaceAll(m.progress.Errors[0], "\n", " · ")))
	}

	return rows
}
func (m model) viewRun() string {
	if !m.running {
		return m.viewResult()
	}
	if chooseLayout(m.width, m.height) == layoutWide {
		return m.viewRunWide()
	}

	mainW := m.contentWidth()
	statsW := 25
	if chooseLayout(m.width, m.height) != layoutWide || mainW < 68 {
		statsW = 0
	}
	workW := mainW
	if statsW > 0 {
		workW -= statsW + 2
	}
	taskW := workW
	preview := ""
	if chooseLayout(m.width, m.height) == layoutWide && len(m.currentRun.Inputs) > 0 && m.mediaPreview != nil {
		if rendered, err := m.mediaPreview.Render(m.currentRun.Inputs[0].Path, 24, 6, m.colorProfile); err == nil && rendered != "" {
			preview = rendered
			taskW = max(24, workW-28)
		}
	}
	pct, known := m.progress.Percent()
	barW := max(8, taskW-12)
	current := m.progress.CurrentFile
	if current == "" {
		current = m.runLabel
	}
	rows := []string{
		stHint.Render(m.lang.t("run.current")),
		stTitle.Render(ansi.Truncate(current, max(8, taskW-2), "…")),
		stFieldFocus.Render(progressBar(barW, pct, known)) + "  " + stFieldFocus.Render(percentLabel(pct, known)),
	}
	if m.progress.Discovered > 0 {
		rows = append(rows, stSys.Render(m.progress.Metrics()))
	}
	rows = append(rows, stHint.Render(m.resultLabel()+" · "+time.Since(m.runStart).Truncate(time.Second).String()))
	left := activeTheme.panelRaised.Width(max(1, taskW-4)).Render(strings.Join(rows, "\n"))
	if preview != "" {
		previewPanel := activeTheme.panel.Width(24).Render(preview)
		left = lipgloss.JoinHorizontal(lipgloss.Top, previewPanel, "  ", left)
	}

	if statsW > 0 {
		stats := []string{
			stTitle.Render(m.lang.t("run.stats")),
			statLine(m.lang.t("run.pending"), m.progress.Pending, activeTheme.warning),
			statLine(m.lang.t("run.running"), m.progress.Running, activeTheme.data),
			statLine(m.lang.t("run.success"), m.progress.Succeeded, activeTheme.success),
			statLine(m.lang.t("run.failed"), m.progress.Failed, activeTheme.failure),
			statLine(m.lang.t("run.canceled"), m.progress.Canceled, activeTheme.warning),
			statLine(m.lang.t("run.skipped"), m.progress.Skipped, activeTheme.hint),
		}
		right := activeTheme.panel.Width(max(1, statsW-4)).Render(strings.Join(stats, "\n"))
		left = lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	}

	details := m.lang.t("run.collapsed")
	if m.detailsOpen {
		available := max(3, m.mainHeight()-10)
		if m.follow {
			details = tailText(strings.Join(m.scrollback, "\n"), available)
		} else {
			details = fitScreen(m.vp.View(), workW, available)
		}
	}
	if len(m.progress.Errors) > 0 {
		details = stErr.Render(strings.ReplaceAll(m.progress.Errors[0], "\n", " · ")) + "\n" + details
	}
	if m.mainHeight() >= 12 {
		left = lipgloss.JoinVertical(lipgloss.Left, left, activeTheme.panel.Width(max(1, mainW-4)).Render(details))
	}
	return lipgloss.NewStyle().Height(m.mainHeight()).Render(fitScreen(left, mainW, m.mainHeight()))
}

func tailText(text string, height int) string {
	lines := strings.Split(text, "\n")
	if height > 0 && len(lines) > height {
		lines = lines[len(lines)-height:]
	}
	return strings.Join(lines, "\n")
}

func percentLabel(pct float64, known bool) string {
	if !known {
		return "--"
	}
	return fmt.Sprintf("%.0f%%", pct)
}

func (m model) viewResult() string {
	mainW := m.contentWidth()
	title := "✓ " + m.lang.t("run.complete")
	style := activeTheme.success
	border := colour(activeTheme.palette.Success)
	switch m.progress.Status {
	case xprogress.StatusCanceled:
		title = "! " + m.lang.t("status.canceled")
		style = activeTheme.warning
		border = colour(activeTheme.palette.Warning)
	case xprogress.StatusPartial, xprogress.StatusFailed:
		title = "× " + m.resultLabel()
		style = activeTheme.failure
		border = colour(activeTheme.palette.Error)
	}
	rows := []string{
		style.Bold(true).Render(title),
		stHint.Render(m.runLabel + " · " + m.runLast.Truncate(time.Millisecond).String()),
		m.progress.Summary(m.lang == LangZh),
	}
	if m.progress.TotalBytes > 0 {
		rows = append(rows, stSys.Render(m.progress.Metrics()))
	}
	if len(m.progress.Errors) > 0 {
		rows = append(rows, stErr.Render(strings.ReplaceAll(m.progress.Errors[0], "\n", " · ")))
	} else if m.runErr != nil {
		rows = append(rows, stErr.Render(m.runErr.Error()))
	}
	for i, item := range m.runResult.Items {
		if i == 3 {
			rows = append(rows, stHint.Render(fmt.Sprintf("+%d more", len(m.runResult.Items)-i)))
			break
		}
		rows = append(rows, stErr.Render("× "+item.DisplayName)+stHint.Render(" · "+item.Phase))
	}
	cardW := min(max(34, mainW/2), max(34, mainW-4))
	card := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(border).Padding(1, 2).Width(max(1, cardW-6)).Render(strings.Join(rows, "\n"))
	if m.detailsOpen && m.mainHeight() >= 14 {
		card = lipgloss.JoinVertical(lipgloss.Left, card, activeTheme.panel.Width(max(1, mainW-4)).Render(m.vp.View()))
	}
	return lipgloss.Place(mainW, m.mainHeight(), lipgloss.Center, lipgloss.Center, card)
}
func (m model) viewStatus() string {
	var buttons []uiButton
	prefix := ""
	if m.isSetting {
		buttons = m.settingsButtons()
	} else if m.state() == stateRun {
		buttons = m.runButtons()
	} else {
		if m.settingsError != "" {
			return fitScreen(m.settingsError, m.width, 1)
		}
		return ""
	}
	var b strings.Builder
	b.WriteString(prefix)
	for i, button := range buttons {
		padding := max(0, button.x0-lipgloss.Width(b.String()))
		b.WriteString(strings.Repeat(" ", padding))
		label := button.label
		if m.isSetting && m.settingsButton == i {
			label = stFieldFocus.Render(label)
		}
		b.WriteString(label)
	}
	return stStatusBg.Render(ansi.Truncate(b.String(), m.width, ""))
}
func (m model) currentFieldHelp() string {
	if m.form == nil || len(m.form.fields) == 0 {
		return ""
	}
	if m.settingsButton >= 0 && m.isSetting {
		return m.lang.t("set.buttons.help")
	}
	return m.form.fields[m.formIx].help(m.lang)
}
