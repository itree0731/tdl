package tui

import (
	"fmt"
	"os"
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

type wideNavEntry struct {
	actionIndex int
	label       string
	icon        string
}

func (m model) widePrimaryNav() []wideNavEntry {
	return []wideNavEntry{
		{m.actionIndex("up"), m.actions[m.actionIndex("up")].title(m.lang), "↑"},
		{m.actionIndex("dl"), m.actions[m.actionIndex("dl")].title(m.lang), "↓"},
		{m.actionIndex("chatls"), m.lang.t("sidebar.chat"), "□"},
		{m.actionIndex("forward"), m.lang.t("sidebar.tasks"), "→"},
		{m.actionIndex("backup"), m.actions[m.actionIndex("backup")].title(m.lang), "◉"},
		{m.actionIndex("recover"), m.actions[m.actionIndex("recover")].title(m.lang), "↶"},
		{m.actionIndex("settings"), m.actions[m.actionIndex("settings")].title(m.lang), "⚙"},
	}
}

func (m model) wideSystemNav() []wideNavEntry {
	return []wideNavEntry{
		{m.actionIndex("login"), m.actions[m.actionIndex("login")].title(m.lang), ""},
		{m.actionIndex("update"), m.actions[m.actionIndex("update")].title(m.lang), ""},
		{m.actionIndex("version"), m.actions[m.actionIndex("version")].title(m.lang), ""},
		{m.actionIndex("quit"), m.actions[m.actionIndex("quit")].title(m.lang), ""},
	}
}

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
	for _, entry := range m.widePrimaryNav() {
		label := fmt.Sprintf("    %s  %s", entry.icon, entry.label)
		if entry.actionIndex == m.menuIx {
			label = fmt.Sprintf(" ▌  %s  %s", entry.icon, entry.label)
			rows = append(rows, stItemSelected.Width(max(1, width-2)).Render(ansi.Truncate(label, max(1, width-3), "…")))
		} else {
			rows = append(rows, activeTheme.text.Render(ansi.Truncate(label, max(1, width-2), "…")))
		}
	}
	footerY := height - 7
	for len(rows) < footerY {
		rows = append(rows, "")
	}
	systemLabels := make([]string, 0, 4)
	for _, entry := range m.wideSystemNav() {
		label := entry.label
		if entry.actionIndex == m.menuIx {
			label = "▌" + label
		}
		systemLabels = append(systemLabels, label)
	}
	rows = append(rows,
		stHint.Render("  "+strings.Repeat("─", max(1, width-4))),
		stHint.Render("  "+strings.Join(systemLabels, " · ")),
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

func (m model) viewChatHub() string {
	width := m.contentWidth()
	rows := []string{
		stHint.Render(strings.ToUpper(m.lang.t("sidebar.chat"))),
		stTitle.Render(m.lang.t("chat.hub.title")),
		stHint.Render(m.lang.t("chat.hub.desc")),
		"",
	}
	ids := []string{"chatls", "chatexport", "chatusers"}
	for i, id := range ids {
		action := m.actions[m.actionIndex(id)]
		rows = append(rows,
			stFieldFocus.Render(fmt.Sprintf("[ %d ]  %s", i+1, action.title(m.lang))),
			stHint.Render("       "+action.desc(m.lang)),
			"",
		)
	}
	panel := activeTheme.panelRaised.Width(max(1, width-8)).Render(strings.Join(rows, "\n"))
	return lipgloss.NewStyle().Width(width).Height(m.mainHeight()).Padding(1, 2).Render(panel)
}

func (m model) formPrimaryWidth() int {
	summaryW := min(32, max(26, m.contentWidth()/4))
	return max(42, m.contentWidth()-summaryW-2)
}

func (m model) viewFormWide() string {
	totalW, height := m.contentWidth(), m.mainHeight()
	primaryW := m.formPrimaryWidth()
	summaryW := max(24, totalW-primaryW-2)
	copyModel := m
	// viewFormClassic adds one horizontal padding cell on each side. Keep its
	// outer width inside the panel content box so right-aligned picker buttons
	// never wrap their closing bracket.
	copyModel.contentWidthOverride = primaryW - 6
	leftInner := fitScreen(copyModel.viewFormClassic(), max(1, primaryW-4), max(1, height-2))
	left := workbenchPanel(primaryW, height, colour(activeTheme.palette.Copper), leftInner)

	rows := []string{
		stTitle.Render(m.lang.t("form.summary")),
		stHint.Render(strings.Repeat("─", max(1, summaryW-6))),
		stHint.Render(m.lang.t("home.account")+": ") + stSys.Render(m.currentNS()),
		stHint.Render(m.lang.t("form.action")+": ") + stFieldFocus.Render(m.form.title(m.lang)),
	}
	if count, size := m.formPathSummary(); count > 0 {
		rows = append(rows,
			stHint.Render(m.lang.t("form.files")+": ")+stSys.Render(fmt.Sprintf("%d", count)),
			stHint.Render(m.lang.t("picker.size")+": ")+stSys.Render(formatBytes(size)),
		)
	}
	if chat := m.formValueByLabel("Chat"); chat != "" {
		rows = append(rows, stHint.Render(m.lang.t("field.up.chat")+": ")+stSys.Render(chat))
	}
	if cover := m.formValueByLabel("Cover mode"); cover != "" {
		rows = append(rows, stHint.Render(m.lang.t("form.cover")+": ")+stSys.Render(localizedChoice(m.lang, cover)))
	}
	rows = append(rows, "", stHint.Render(m.lang.t("form.submit.help")))
	right := workbenchPanel(summaryW, height, colour(activeTheme.palette.Border), strings.Join(rows, "\n"))
	return fitScreen(lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right), totalW, height)
}

func (m model) formPathSummary() (count int, size int64) {
	if m.form == nil {
		return 0, 0
	}
	for _, field := range m.form.fields {
		for _, path := range field.paths {
			count++
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				size += info.Size()
			}
		}
	}
	return count, size
}

func (m model) formValueByLabel(label string) string {
	if m.form == nil {
		return ""
	}
	for i := range m.form.fields {
		if m.form.fields[i].labelKey == label {
			return m.form.fields[i].value()
		}
	}
	return ""
}

func (m model) chatHubRegions() []HitRegion {
	if m.state() != stateMenu || chooseLayout(m.width, m.height) != layoutWide || m.actions[m.menuIx].id != "chatls" {
		return nil
	}
	ids := []string{"chatls", "chatexport", "chatusers"}
	regions := make([]HitRegion, 0, len(ids))
	for i, id := range ids {
		index := m.actionIndex(id)
		regions = append(regions, HitRegion{
			ID:      "hub:" + id,
			Rect:    Rect{X: m.sidebarWidth() + 3, Y: m.contentTop() + 6 + i*3, W: max(1, m.contentWidth()-8), H: 2},
			Enabled: true,
			Action:  UIAction{Kind: UIActionHub, Index: index, ID: id},
		})
	}
	return regions
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
