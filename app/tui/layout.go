package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	xprogress "github.com/iyear/tdl/pkg/progress"
)

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
		out = append(out, uiButton{id, labels[i], x, end, 1 + m.mainHeight()})
		x = end + 1
	}
	return out
}
func (m model) runButtons() []uiButton {
	if m.running {
		return m.buttons("run.details", "run.stop")
	}
	return m.buttons("run.details", "form.back")
}
func (m model) settingsButtons() []uiButton { return m.buttons("set.apply", "set.cancel") }
func (m model) activateButton(id string) (tea.Model, tea.Cmd) {
	switch id {
	case "run.stop":
		return m.stopRun()
	case "run.details":
		m.detailsOpen = !m.detailsOpen
	case "form.back":
		if !m.running {
			m.toMenu()
		}
	case "set.apply":
		m.applySettingsDraft()
	case "set.cancel":
		m.discardSettingsDraft()
	}
	return m, nil
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
	rows := m.runSummaryRows()
	if m.detailsOpen {
		rows = append(rows, m.vp.View())
	} else {
		rows = append(rows, m.lang.t("run.collapsed"))
		if !m.running && m.runErr != nil && len(m.progress.Errors) == 0 {
			rows = append(rows, m.runErr.Error())
		}
	}
	body := fitScreen(strings.Join(rows, "\n"), m.width, m.mainHeight())
	return lipgloss.NewStyle().Height(m.mainHeight()).Render(body)
}
func (m model) viewStatus() string {
	var buttons []uiButton
	prefix := ""
	if m.isSetting {
		buttons = m.settingsButtons()
	} else if m.state() == stateRun {
		buttons = m.runButtons()
		pct, known := m.progress.Percent()
		budget := max(0, buttons[0].x0-1)
		if budget >= 12 {
			n := max(1, budget-8)
			filled := 0
			label := "--"
			if known {
				filled = int(pct * float64(n) / 100)
				label = fmt.Sprintf("%.0f%%", pct)
			}
			prefix = strings.Repeat("█", filled) + strings.Repeat("░", n-filled) + " " + label
		}
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
