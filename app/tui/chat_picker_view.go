package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m model) chatWindow() (first, count int) {
	if m.chatPicker == nil {
		return 0, 0
	}
	count = max(1, m.mainHeight()-8)
	if len(m.chatPicker.filtered) <= count {
		return 0, len(m.chatPicker.filtered)
	}
	first = m.chatPicker.cursor - count/2
	first = max(0, min(first, len(m.chatPicker.filtered)-count))
	return first, count
}

func (m model) viewChatPicker() string {
	if m.chatPicker == nil {
		return ""
	}
	p := m.chatPicker
	width := m.contentWidth()
	rows := []string{
		stTitle.Render(m.lang.t("chat.title")) + "  " + stHint.Render(m.lang.t("home.account")+": ") + stSys.Render(p.namespace),
		activeTheme.promptActive.Width(max(1, width-6)).Render(p.search.View()),
	}
	if width >= 76 {
		rows = append(rows, stHint.Render(padCell(m.lang.t("chat.type"), 10)+"  "+padCell(m.lang.t("chat.name"), max(12, width-46))+"  "+padCell(m.lang.t("chat.username"), 14)+"  ID"))
	} else {
		rows = append(rows, stHint.Render(padCell(m.lang.t("chat.type"), 9)+"  "+padCell(m.lang.t("chat.name"), max(12, width-28))+"  ID"))
	}
	first, count := m.chatWindow()
	for visible := first; visible < first+count; visible++ {
		index := p.filtered[visible]
		item := p.items[index]
		marker := "  "
		if item.Self {
			marker = "★ "
		}
		var row string
		if width >= 76 {
			nameW := max(12, width-46)
			row = marker + padCell(item.Type, 10) + "  " + padCell(ansi.Truncate(item.Title, nameW, "…"), nameW) + "  " + padCell(item.Username, 14) + "  " + strconv.FormatInt(item.ID, 10)
		} else {
			nameW := max(12, width-28)
			row = marker + padCell(item.Type, 9) + "  " + padCell(ansi.Truncate(item.Title, nameW, "…"), nameW) + "  " + strconv.FormatInt(item.ID, 10)
		}
		row = ansi.Truncate(row, max(1, width-4), "…")
		if visible == p.cursor {
			row = stItemSelected.Width(max(1, width-4)).Render("▌" + row)
		} else {
			row = activeTheme.text.Render("  " + row)
		}
		rows = append(rows, row)
	}
	for len(rows) < m.mainHeight()-5 {
		rows = append(rows, "")
	}
	topicRow := ""
	if current, ok := p.current(); ok && len(current.Topics) > 0 {
		topic := m.lang.t("chat.main")
		if p.topic >= 0 && p.topic < len(current.Topics) {
			topic = fmt.Sprintf("%d · %s", current.Topics[p.topic].ID, current.Topics[p.topic].Title)
		}
		topicRow = stHint.Render(m.lang.t("chat.topic")+": ") + stSys.Render(topic) + "  " + stFieldFocus.Render("[ "+m.lang.t("chat.topic.next")+" ]")
	}
	rows = append(rows, topicRow)
	if p.err != "" {
		rows = append(rows, stErr.Render(ansi.Truncate(p.err, max(1, width-2), "…")))
	} else if p.loading {
		rows = append(rows, stFieldFocus.Render("◐ "+m.lang.t("chat.loading")))
	} else {
		rows = append(rows, stHint.Render(fmt.Sprintf(m.lang.t("chat.loaded"), len(p.items))))
	}
	buttons := stFieldFocus.Render("[ "+m.lang.t("chat.confirm")+" ]") + "  " + stHint.Render("[ "+m.lang.t("chat.more")+" ]  [ "+m.lang.t("chat.manual")+" ]  [ "+m.lang.t("picker.cancel")+" ]")
	rows = append(rows, buttons)
	return lipgloss.NewStyle().Width(width).Height(m.mainHeight()).Padding(0, 1).Render(fitScreen(strings.Join(rows, "\n"), width, m.mainHeight()))
}

func (m model) chatRegions() []HitRegion {
	if m.chatPicker == nil {
		return nil
	}
	x := m.sidebarWidth()
	first, count := m.chatWindow()
	regions := make([]HitRegion, 0, count)
	for visible := first; visible < first+count; visible++ {
		regions = append(regions, HitRegion{
			ID: fmt.Sprintf("chat:row:%d", visible), Rect: Rect{X: x, Y: m.contentTop() + 3 + visible - first, W: m.contentWidth(), H: 1}, Enabled: true,
			Action: UIAction{Kind: UIActionPicker, ID: "chat", Index: visible},
		})
	}
	if item, ok := m.chatPicker.current(); ok && len(item.Topics) > 0 {
		regions = append(regions, HitRegion{ID: "chat.topic.next", Rect: Rect{X: x + 1, Y: m.contentTop() + m.mainHeight() - 3, W: m.contentWidth() - 2, H: 1}, Enabled: true, Action: UIAction{Kind: UIActionButton, ID: "chat.topic.next"}})
	}
	y := m.contentTop() + m.mainHeight() - 1
	xPos := x + 1
	buttons := []struct {
		id, label string
		enabled   bool
	}{
		{"chat.confirm", m.lang.t("chat.confirm"), true},
		{"chat.more", m.lang.t("chat.more"), m.chatPicker.next != nil && !m.chatPicker.loading},
		{"chat.manual", m.lang.t("chat.manual"), true},
		{"chat.cancel", m.lang.t("picker.cancel"), true},
	}
	for _, button := range buttons {
		label := "[ " + button.label + " ]"
		w := lipgloss.Width(label)
		regions = append(regions, HitRegion{ID: button.id, Rect: Rect{X: xPos, Y: y, W: w, H: 1}, Enabled: button.enabled, Action: UIAction{Kind: UIActionButton, ID: button.id}})
		xPos += w + 2
	}
	return regions
}
