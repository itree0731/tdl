package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type memoryChatSource struct {
	pages map[string][]ChatPage
	calls map[string]int
}

func TestChatPickerMouseLoadMoreUsesVisibleButton(t *testing.T) {
	isolateSettings(t)
	source := &memoryChatSource{pages: map[string][]ChatPage{
		"default": {
			{Items: []ChatRef{{ID: 1, Title: "First"}}, Next: &ChatCursor{OffsetID: 1}},
			{Items: []ChatRef{{ID: 2, Title: "Second"}}},
		},
	}}
	m := sized(t, newModel(stubExec, []string{"default"}, WithChatSource(source)), 120, 30)
	r, _ := m.openMenuItem(indexOfAction(m, "up"))
	m = asModel(r)
	r, cmd := m.openChatSelector(1)
	m = asModel(r)
	r, _ = m.Update(cmd())
	m = asModel(r)
	frame := m.frame()
	var more *HitRegion
	for i := range frame.Regions {
		if frame.Regions[i].ID == "chat.more" {
			more = &frame.Regions[i]
			break
		}
	}
	if more == nil || !more.Enabled {
		t.Fatal("visible Load more button has no enabled hit region")
	}
	plain := renderPlain(m)
	lines := strings.Split(plain, "\n")
	visibleY := -1
	for i, line := range lines {
		if strings.Contains(line, "Load more") {
			visibleY = i
			break
		}
	}
	if visibleY != more.Rect.Y {
		t.Fatalf("Load more rendered at y=%d but hit region uses y=%d\n%s", visibleY, more.Rect.Y, plain)
	}
	segment := ansi.Cut(lines[more.Rect.Y], more.Rect.X, more.Rect.X+more.Rect.W)
	if !strings.Contains(segment, "Load more") {
		t.Fatalf("hit region does not cover Load more: %q rect=%+v\n%s", segment, more.Rect, plain)
	}
	r, cmd = m.Update(tea.MouseMsg{X: more.Rect.X, Y: more.Rect.Y, Button: tea.MouseButtonLeft})
	m = asModel(r)
	if cmd == nil || !m.chatPicker.loading {
		t.Fatal("mouse click did not start next page")
	}
	r, _ = m.Update(cmd())
	m = asModel(r)
	if len(m.chatPicker.items) != 3 || m.chatPicker.items[2].ID != 2 {
		t.Fatalf("next page was not appended: %+v", m.chatPicker.items)
	}
}

func (s *memoryChatSource) Page(_ context.Context, namespace string, cursor *ChatCursor, _ int) (ChatPage, error) {
	if s.calls == nil {
		s.calls = make(map[string]int)
	}
	index := s.calls[namespace]
	s.calls[namespace]++
	if cursor != nil {
		index = cursor.OffsetID
	}
	if index >= len(s.pages[namespace]) {
		return ChatPage{}, nil
	}
	return s.pages[namespace][index], nil
}

func TestChatSelectorSavedSearchTopicAndStableID(t *testing.T) {
	isolateSettings(t)
	source := &memoryChatSource{pages: map[string][]ChatPage{
		"default": {{Items: []ChatRef{
			{ID: 100, Title: "Family", Username: "family", Type: "group"},
			{ID: 200, Title: "Project Forum", Username: "project", Type: "group", Topics: []TopicRef{{ID: 7, Title: "Media"}}},
		}}},
	}}
	m := sized(t, newModel(stubExec, []string{"default"}, WithChatSource(source)), 120, 30)
	r, _ := m.openMenuItem(indexOfAction(m, "up"))
	m = asModel(r)
	r, cmd := m.openChatSelector(1)
	m = asModel(r)
	if cmd == nil || m.state() != screenChatPicker {
		t.Fatal("chat selector did not start loading")
	}
	r, _ = m.Update(cmd())
	m = asModel(r)
	if len(m.chatPicker.items) != 3 || !m.chatPicker.items[0].Self {
		t.Fatalf("Saved Messages is not pinned: %+v", m.chatPicker.items)
	}
	m.chatPicker.search.SetValue("forum")
	m.chatPicker.filter("forum")
	if len(m.chatPicker.filtered) != 1 {
		t.Fatalf("search result = %+v", m.chatPicker.filtered)
	}
	m.chatPicker.topic = 0
	r, _ = m.closeChatSelector(true)
	m = asModel(r)
	if got := m.form.fields[1].ti.Value(); got != "200" {
		t.Fatalf("chat command value = %q", got)
	}
	if got := m.form.fields[2].ti.Value(); got != "7" {
		t.Fatalf("topic command value = %q", got)
	}
	if got := loadSettings().RecentChats["default"]; len(got) != 1 || got[0] != 200 {
		t.Fatalf("recent chats = %v", got)
	}
}

func TestChatSelectorDropsOldScreenResults(t *testing.T) {
	isolateSettings(t)
	m := newModel(stubExec, nil)
	m.chatPicker = newChatPicker("default", 2)
	m.screenID = 2
	r, _ := m.Update(chatPageMsg{screenID: 1, page: ChatPage{Items: []ChatRef{{ID: 99, Title: "old"}}}})
	m = asModel(r)
	if len(m.chatPicker.items) != 1 {
		t.Fatalf("old page polluted selector: %+v", m.chatPicker.items)
	}
}

func TestRecentChatsBoundedAndMovedToFront(t *testing.T) {
	existing := make([]int64, 20)
	for i := range existing {
		existing[i] = int64(i + 1)
	}
	got := updateRecentChats(existing, 10)
	if len(got) != 20 || got[0] != 10 || got[1] != 1 {
		t.Fatalf("recent reorder = %v", got)
	}
	got = updateRecentChats(existing, 99)
	if len(got) != 20 || got[0] != 99 || got[19] != 19 {
		t.Fatalf("recent bound = %v", got)
	}
}

func TestChatSearchInputUpdatesFilter(t *testing.T) {
	p := newChatPicker("default", 1)
	p.loading = false
	p.items = append(p.items, ChatRef{ID: 42, Title: "Alpha Team", Username: "alpha", Type: "group"})
	p.filter("")
	m := newModel(stubExec, nil)
	m.chatPicker = p
	m.screenID = 1
	r, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("alpha")})
	m = asModel(r)
	if len(m.chatPicker.filtered) != 1 || m.chatPicker.items[m.chatPicker.filtered[0]].ID != 42 {
		t.Fatalf("filtered chats = %+v", m.chatPicker.filtered)
	}
}
