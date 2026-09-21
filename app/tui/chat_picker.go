package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
)

type TopicRef struct {
	ID    int
	Title string
}

type ChatRef struct {
	ID       int64
	Username string
	Title    string
	Type     string
	Topics   []TopicRef
	Self     bool
}

type PeerCursor struct {
	ID   int64
	Kind string
}

type ChatCursor struct {
	OffsetDate int
	OffsetID   int
	OffsetPeer PeerCursor
}

type ChatPage struct {
	Items   []ChatRef
	Next    *ChatCursor
	Skipped int
}

type ChatSource interface {
	Page(context.Context, string, *ChatCursor, int) (ChatPage, error)
}

type tuiOption func(*model)

func WithChatSource(source ChatSource) tuiOption {
	return func(m *model) { m.chatSource = source }
}

type chatPicker struct {
	items     []ChatRef
	filtered  []int
	cursor    int
	selected  int
	topic     int
	search    textinput.Model
	next      *ChatCursor
	loading   bool
	err       string
	namespace string
	screenID  uint64
}

type chatPageMsg struct {
	screenID uint64
	page     ChatPage
	err      error
}

func newChatPicker(namespace string, screenID uint64) *chatPicker {
	search := textinput.New()
	search.Placeholder = "title, username or ID"
	search.Focus()
	p := &chatPicker{
		items:  []ChatRef{{Title: "Saved Messages", Type: "self", Self: true}},
		search: search, selected: -1, topic: -1, namespace: namespace, screenID: screenID, loading: true,
	}
	p.filter("")
	return p
}

func loadChatPageCmd(ctx context.Context, source ChatSource, namespace string, cursor *ChatCursor, screenID uint64) tea.Cmd {
	return func() tea.Msg {
		if source == nil {
			return chatPageMsg{screenID: screenID, err: fmt.Errorf("chat source is unavailable; use manual input")}
		}
		page, err := source.Page(ctx, namespace, cursor, 100)
		return chatPageMsg{screenID: screenID, page: page, err: err}
	}
}

func (p *chatPicker) apply(page ChatPage, err error, recent []int64) {
	p.loading = false
	if err != nil {
		p.err = err.Error()
		return
	}
	p.err = ""
	seen := make(map[int64]bool, len(p.items))
	for _, item := range p.items {
		seen[item.ID] = !item.Self
	}
	for _, item := range page.Items {
		if seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		p.items = append(p.items, item)
	}
	p.next = page.Next
	if len(recent) > 0 {
		rank := make(map[int64]int, len(recent))
		for i, id := range recent {
			rank[id] = i
		}
		sort.SliceStable(p.items[1:], func(i, j int) bool {
			a, aok := rank[p.items[i+1].ID]
			b, bok := rank[p.items[j+1].ID]
			if aok != bok {
				return aok
			}
			return aok && a < b
		})
	}
	p.filter(p.search.Value())
}

func (p *chatPicker) filter(query string) {
	query = strings.ToLower(strings.TrimSpace(query))
	p.filtered = p.filtered[:0]
	for i, item := range p.items {
		haystack := strings.ToLower(strings.Join([]string{item.Title, item.Username, item.Type, strconv.FormatInt(item.ID, 10)}, " "))
		matched := query == "" || strings.Contains(haystack, query)
		if !matched {
			for _, topic := range item.Topics {
				if strings.Contains(strings.ToLower(topic.Title), query) {
					matched = true
					break
				}
			}
		}
		if matched {
			p.filtered = append(p.filtered, i)
		}
	}
	p.cursor = min(p.cursor, max(0, len(p.filtered)-1))
}

func (p *chatPicker) current() (ChatRef, bool) {
	if len(p.filtered) == 0 || p.cursor < 0 || p.cursor >= len(p.filtered) {
		return ChatRef{}, false
	}
	return p.items[p.filtered[p.cursor]], true
}

func updateRecentChats(existing []int64, id int64) []int64 {
	if id == 0 {
		return existing
	}
	out := []int64{id}
	for _, value := range existing {
		if value != id {
			out = append(out, value)
		}
		if len(out) == 20 {
			break
		}
	}
	return out
}
