package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	wideSidebarWidth = 30
	wideHeaderHeight = 3
	wideFooterHeight = 1
)

func (m model) currentPage() string {
	switch m.state() {
	case stateMenu:
		return m.viewMenu()
	case stateForm:
		return m.viewForm()
	case screenFilePicker:
		return m.viewFilePicker()
	case screenChatPicker:
		return m.viewChatPicker()
	case screenConfirm:
		return m.viewRetryConfirm()
	case screenErrorList:
		return m.viewErrorList()
	default:
		return m.viewRun()
	}
}

func (m model) wideFrame() Frame {
	sw := m.sidebarWidth()
	workspaceH := max(1, m.height-wideFooterHeight)
	rightW := max(1, m.width-sw)
	sidebar := m.viewWideSidebar(sw, workspaceH)
	header := m.viewWideHeader(rightW)
	page := lipgloss.NewStyle().Width(rightW).Height(m.mainHeight()).Render(m.currentPage())
	right := lipgloss.JoinVertical(lipgloss.Left, header, page)
	workspace := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, right)
	text := lipgloss.JoinVertical(lipgloss.Left, workspace, m.viewWideFooter())
	return Frame{Text: fitScreen(text, m.width, m.height), Regions: m.hitRegions()}
}

func (m model) compactFrame() Frame {
	header := stBrand.Render(brandName) + "  " + stHint.Render(m.lang.t("banner.title"))
	if len(m.namespaces) > 0 {
		header += "  " + stOK.Render("● "+m.currentNS())
	}
	header = ansi.Truncate(header, max(1, m.width), "…")
	nav := m.viewCompactNav()
	page := fitScreen(m.currentPage(), m.width, m.mainHeight())
	footer := activeTheme.status.Width(max(1, m.width)).Render(ansi.Truncate(m.viewShortcuts(), max(1, m.width), "…"))
	text := lipgloss.JoinVertical(lipgloss.Left, header, nav, page, footer)
	return Frame{Text: fitScreen(text, m.width, m.height), Regions: m.hitRegions()}
}

func (m model) viewWideSidebar(width, height int) string {
	rows := []string{
		stBrand.Render("  T D L"),
		stTitle.Render("  Telegram Media Transfer"),
		stHint.Render("  " + m.lang.t("sidebar.workbench")),
		stHint.Render("  " + strings.Repeat("─", max(1, width-4))),
		"",
	}
	for i, action := range m.actions {
		label := "    " + action.title(m.lang)
		if i == m.menuIx {
			label = " ▌  " + action.title(m.lang)
			rows = append(rows, stItemSelected.Width(max(1, width-2)).Render(ansi.Truncate(label, max(1, width-3), "…")))
		} else {
			rows = append(rows, activeTheme.text.Render(ansi.Truncate(label, max(1, width-2), "…")))
		}
	}
	footerY := height - 5
	for len(rows) < footerY {
		rows = append(rows, "")
	}
	rows = append(rows,
		stHint.Render("  "+strings.Repeat("─", max(1, width-4))),
		stHint.Render("  "+m.lang.t("banner.sub")),
		stFieldFlag.Render("  MEDIA  ANYWHERE"),
		stFieldFlag.Render("  WITH   TDL"),
	)
	return lipgloss.NewStyle().Width(width).Height(height).Background(colour(activeTheme.palette.Sidebar)).Render(fitScreen(strings.Join(rows, "\n"), width, height))
}

func (m model) viewWideHeader(width int) string {
	left := stHint.Render(m.lang.t("header.session")+": ") + stTitle.Render("Workstation") +
		stShortcutSep.Render("  │  ") + stHint.Render(m.lang.t("header.account")+": ")
	if len(m.namespaces) == 0 {
		left += stHint.Render(m.lang.t("hdr.nosession"))
	} else {
		for i, namespace := range m.namespaces {
			if i > 0 {
				left += "  "
			}
			if namespace == m.currentNS() {
				left += stFieldFocus.Render("● " + namespace)
			} else {
				left += stHint.Render("○ " + namespace)
			}
		}
	}
	left += stShortcutSep.Render("  │  ") + stHint.Render(m.lang.t("header.network")+": ") + stOK.Render("● "+m.lang.t("header.connected"))
	right := stHint.Render(time.Now().Format("2006-01-02 15:04:05"))
	line := joinEdges(left, right, max(1, width-8))
	return activeTheme.panel.Width(max(1, width-4)).Height(1).Render(line)
}

func (m model) viewWideFooter() string {
	left := stHint.Render("[TDL]  ") + activeTheme.text.Render("Telegram Media Transfer")
	right := m.viewShortcuts()
	if m.state() == stateForm {
		buttons := m.formButtons()
		if m.isSetting {
			buttons = m.settingsButtons()
		}
		labels := make([]string, 0, len(buttons))
		for i, button := range buttons {
			label := button.label
			if m.isSetting && m.settingsButton == i {
				label = stFieldFocus.Render(label)
			}
			labels = append(labels, label)
		}
		right = strings.Join(labels, " ")
	} else if m.state() == stateRun && !m.running {
		buttons := m.runButtons()
		labels := make([]string, 0, len(buttons))
		for _, button := range buttons {
			labels = append(labels, button.label)
		}
		right = strings.Join(labels, " ")
	}
	if m.state() == stateRun && m.running {
		current := fmt.Sprintf("%d/%d", m.progress.Succeeded+m.progress.Failed+m.progress.Canceled, max(m.progress.Expected, m.progress.Discovered))
		right = stSys.Render("↑ "+xBytesPerSecond(m.progress.Speed)) + stShortcutSep.Render("  │  ") + stHint.Render(m.lang.t("footer.task")+": ") + stSys.Render(current)
		if m.progress.ProcessInfo != "" {
			right += stShortcutSep.Render("  │  ") + stHint.Render(m.progress.ProcessInfo)
		}
	}
	return activeTheme.status.Width(max(1, m.width)).Render(joinEdges(left, right, m.width))
}

func joinEdges(left, right string, width int) string {
	space := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return ansi.Truncate(left+strings.Repeat(" ", space)+right, max(1, width), "")
}

func xBytesPerSecond(speed float64) string {
	return fmt.Sprintf("%s/s", formatBytes(int64(speed)))
}
