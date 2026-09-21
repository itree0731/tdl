package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	xprogress "github.com/iyear/tdl/pkg/progress"
)

type screenKind uint8

const (
	screenMenu screenKind = iota
	screenForm
	screenFilePicker
	screenChatPicker
	screenConfirm
	screenRun
	screenResult
	screenErrorList
	screenSettings
)

type state = screenKind

const (
	stateMenu = screenMenu
	stateForm = screenForm
	stateRun  = screenRun
)

const maxScrollbackLines = 5000

type model struct {
	ctx     context.Context
	program *tea.Program
	exec    Executor

	lang       Lang
	set        settings
	namespaces []string // logged-in namespaces, for the header chip
	hasSession bool     // true once any namespace was seen at startup

	actions []action
	menuIx  int

	// form state (also used for settings)
	form              *action
	formIx            int
	isSetting         bool
	settingsBase      settings
	settingsDirty     bool
	settingsPrompt    bool
	settingsQuit      bool
	settingsError     string
	formError         string
	settingsButton    int
	running           bool
	showOutput        bool
	runErr            error
	runLabel          string
	runCommand        string
	currentRun        RunSpec
	runResult         RunResult
	picker            *filePicker
	pickerField       int
	pickerScanID      uint64
	chatSource        ChatSource
	chatPicker        *chatPicker
	chatField         int
	retryPrompt       bool
	errorListOpen     bool
	errorCursor       int
	clipboard         Clipboard
	clipboardNotice   string
	colorProfile      ColorProfile
	mediaPreview      MediaPreview
	previewPath       string
	previewText       string
	previewErr        string
	previewLoading    bool
	runStart          time.Time
	runLast           time.Duration
	progress          xprogress.Snapshot
	progressCollector *xprogress.Collector
	runID             uint64
	detailsOpen       bool
	live              string
	stopReq           bool // ctrl+c pressed once already: next one force quits
	cancelRun         func()

	vp     viewport.Model
	follow bool

	scrollback     []string
	scrollTimes    []time.Time
	lastLoggedFile string
	lastLoggedPct  int

	width, height        int
	screenID             uint64
	contentWidthOverride int
}

func newModel(exec Executor, namespaces []string, options ...tuiOption) model {
	set := loadSettings()
	if len(namespaces) > 0 {
		selected := false
		for _, ns := range namespaces {
			if ns == set.NS || (set.NS == "" && ns == "default") {
				selected = true
				break
			}
		}
		if !selected {
			set.NS = namespaces[0]
		}
	} else {
		set.NS = ""
	}

	m := model{
		ctx:           context.Background(),
		exec:          exec,
		lang:          Lang(set.Language),
		set:           set,
		namespaces:    namespaces,
		hasSession:    len(namespaces) > 0,
		actions:       newActions(),
		vp:            viewport.New(0, 0),
		follow:        true,
		colorProfile:  detectColorProfile(),
		mediaPreview:  newLocalMediaPreview(32),
		clipboard:     defaultClipboard(),
		lastLoggedPct: -1,
	}
	for _, option := range options {
		option(&m)
	}
	return m
}

// Run starts the TUI program. exec is injected from package cmd;
// namespaces are the logged-in namespaces shown in the header.
func Run(ctx context.Context, exec Executor, namespaces []string, options ...tuiOption) error {
	if ctx == nil {
		ctx = context.Background()
	}
	m := newModel(exec, namespaces, options...)
	m.ctx = ctx
	p := tea.NewProgram(&m, tea.WithContext(ctx), tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.program = p
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd { return nil }

// ---------------------------------------------------------------------------
// settings form

func (m *model) openSettings() {
	langIx := 0
	if m.lang == LangZh {
		langIx = 1
	}
	m.settingsBase = m.set
	m.settingsError = ""
	m.settingsQuit = false
	m.settingsButton = -1
	m.settingsDirty = false
	m.settingsPrompt = false
	m.form = &action{
		id:       "settings",
		titleKey: "menu.settings",
		fields: []field{
			{kind: kChoice, labelKey: "set.language", helpKey: "set.language", choices: []string{"en", "zh"}, choiceIx: langIx},
			textField("set.ns", "--ns", m.set.NS, "default"),
			textField("set.proxy", "--proxy", m.set.Proxy, "protocol://host:port"),
			textField("set.threads", "--threads", m.set.Threads, "4"),
			textField("set.limit", "--limit", m.set.Limit, "2"),
		},
	}
	m.isSetting = true
	m.formIx = 0
	m.focusFormField(0)
}

func (m *model) applySettingsDraft() bool {
	f := m.form.fields
	candidate := m.settingsBase
	candidate.SchemaVersion = settingsSchemaVersion
	candidate.Language = f[0].choices[f[0].choiceIx]
	candidate.NS = f[1].value()
	candidate.Proxy = f[2].value()
	candidate.Threads = f[3].value()
	candidate.Limit = f[4].value()
	if err := validateSettings(candidate); err != nil {
		m.settingsError = err.Error()
		m.updateSettingsDirty()
		return false
	}
	if err := saveSettings(candidate); err != nil {
		m.settingsError = err.Error()
		m.updateSettingsDirty()
		return false
	}
	m.set = candidate
	m.lang = Lang(candidate.Language)
	m.settingsBase = candidate
	m.settingsDirty = false
	m.settingsError = ""
	return true
}

func (m *model) discardSettingsDraft() {
	m.set = m.settingsBase
	m.lang = Lang(m.set.Language)
	m.toMenu()
	m.settingsDirty = false
	m.settingsPrompt = false
}

func (m *model) updateSettingsDirty() {
	if !m.isSetting || m.form == nil {
		return
	}
	f := m.form.fields
	m.settingsDirty = string(Lang(f[0].choices[f[0].choiceIx])) != m.settingsBase.Language ||
		f[1].value() != m.settingsBase.NS ||
		f[2].value() != m.settingsBase.Proxy ||
		f[3].value() != m.settingsBase.Threads ||
		f[4].value() != m.settingsBase.Limit
}

// globalArgs returns persistent flags applied to every executed command.
func (s settings) globalArgs() []string {
	var g []string
	if s.NS != "" {
		g = append(g, "--ns", s.NS)
	}
	if s.Proxy != "" {
		g = append(g, "--proxy", s.Proxy)
	}
	if s.Threads != "" {
		g = append(g, "--threads", s.Threads)
	}
	if s.Limit != "" {
		g = append(g, "--limit", s.Limit)
	}
	return g
}

// ---------------------------------------------------------------------------
// update

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.normalizeWideMenu()
		w := m.width - 4 // prompt box padding+border
		m.vp.Width = m.width
		m.vp.Height = m.mainHeight()
		if m.state() == stateRun {
			m.renderViewport()
		}
		if m.form != nil {
			for i := range m.form.fields {
				m.form.fields[i].ti.Width = w - 24
				if m.form.fields[i].ti.Width < 10 {
					m.form.fields[i].ti.Width = 10
				}
			}
		}
		return m, nil

	case progressSnapshotMsg:
		var previewCmd tea.Cmd
		if msg.runID == m.runID && m.running {
			m.progress = msg.snapshot
			m.appendProgressLog()
			m.renderViewport()
			if msg.snapshot.CurrentSourcePath != "" && msg.snapshot.CurrentSourcePath != m.previewPath {
				previewCmd = m.startMediaPreview(msg.snapshot.CurrentSourcePath)
			}
		}
		return m, previewCmd
	case progressMsg:
		if msg.runID != m.runID || !m.running {
			return m, nil
		}
		m.applyProgress(msg.event)
		return m, nil

	case outputLineMsg:
		if msg.runID != m.runID {
			return m, nil
		}
		m.appendLine(msg.text)
		return m, nil

	case liveLineMsg:
		if msg.runID != m.runID {
			return m, nil
		}
		m.live = msg.text
		return m, nil

	case runDoneMsg:
		if msg.runID != m.runID {
			return m, nil
		}
		m.running = false
		m.runErr = msg.err
		m.runLast = msg.elapsed
		m.live = ""
		m.stopReq = false
		if msg.snapshot != nil {
			m.progress = *msg.snapshot
		} else {
			if m.progressCollector == nil {
				m.progressCollector = xprogress.NewCollector()
			}
			m.progress = m.progressCollector.Finish(msg.err)
		}
		m.runResult = buildRunResult(msg.items)
		m.appendLine("")
		m.appendLine(m.resultLabel() + " " + msg.elapsed.Truncate(time.Millisecond).String())
		if msg.err != nil {
			m.appendLine(msg.err.Error())
		}

		return m, nil

	case chatPageMsg:
		if m.chatPicker == nil || msg.screenID != m.screenID || msg.screenID != m.chatPicker.screenID {
			return m, nil
		}
		m.chatPicker.apply(msg.page, msg.err, m.set.RecentChats[m.currentNS()])
		return m, nil

	case selectionScanTickMsg:
		if m.picker == nil || m.picker.scanner == nil || msg.screenID != m.screenID || msg.scanID != m.picker.scanner.id {
			return m, nil
		}
		return m, tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
			return selectionScanTickMsg{screenID: msg.screenID, scanID: msg.scanID}
		})

	case selectionScanMsg:
		if m.picker == nil || m.picker.scanner == nil || msg.screenID != m.screenID || msg.scanID != m.picker.scanner.id {
			return m, nil
		}
		m.picker.scanner = nil
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) {
				m.picker.err = m.lang.t("picker.scan.canceled")
			} else {
				m.picker.err = msg.err.Error()
			}
			return m, nil
		}
		return m.applyFilePickerPlan(msg.plan)

	case mediaPreviewMsg:
		if msg.screenID != m.screenID || msg.runID != m.runID || msg.path != m.previewPath {
			return m, nil
		}
		m.previewLoading = false
		m.previewText = msg.text
		m.previewErr = ""
		if msg.err != nil {
			m.previewErr = msg.err.Error()
		}
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		if m.settingsPrompt {
			return m.handleSettingsPrompt(msg)
		}
		return m.handleKey(msg)
	}
	return m, nil
}

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.state() == stateRun {
			m.follow = false
			m.vp.LineUp(3)
		}
	case tea.MouseButtonWheelDown:
		if m.state() == stateRun {
			m.vp.LineDown(3)
			if m.atBottom() {
				m.follow = true
			}
		}
	case tea.MouseButtonLeft:
		if action, ok := HitTest(m.frame(), int(msg.X), int(msg.Y)); ok {
			switch action.Kind {
			case UIActionAccount:
				if !m.running && !m.isSetting && !m.settingsPrompt {
					return m.switchNS(action.ID)
				}
			case UIActionMenu:
				if action.Index >= 0 && action.Index < len(m.actions) {
					m.menuIx = action.Index
					if m.state() == stateMenu {
						if chooseLayout(m.width, m.height) == layoutWide && m.actions[action.Index].id == "chatls" {
							return m, nil
						}
						return m.openMenuItem(action.Index)
					}
				}
			case UIActionField:
				if m.form != nil && action.Index >= 0 && action.Index < len(m.form.fields) {
					m.settingsButton = -1
					m.focusFormField(action.Index)
					field := &m.form.fields[action.Index]
					switch field.kind {
					case kBool:
						field.boolVal = !field.boolVal
					case kChoice:
						field.cycle(true)
					}
				}
			case UIActionButton:
				return m.activateButton(action.ID)
			case UIActionPicker:
				if action.ID == "chat" && m.chatPicker != nil && action.Index >= 0 && action.Index < len(m.chatPicker.filtered) {
					m.chatPicker.cursor = action.Index
					return m, nil
				}
				if m.picker != nil && action.Index >= 0 && action.Index < len(m.picker.entries) {
					m.picker.cursor = action.Index
					now := time.Now()
					entry := m.picker.entries[action.Index]
					if entry.Dir && m.picker.lastClickPath == entry.Path && now.Sub(m.picker.lastClickAt) <= 400*time.Millisecond {
						_ = m.picker.enterCurrent()
						m.picker.lastClickPath = ""
					} else {
						m.picker.toggleCurrent()
						m.picker.lastClickPath = entry.Path
						m.picker.lastClickAt = now
					}
				}
			case UIActionError:
				if action.Index >= 0 && action.Index < len(m.runResult.Items) {
					m.errorCursor = action.Index
				}
			case UIActionHub:
				if action.Index >= 0 && action.Index < len(m.actions) {
					return m.openMenuItem(action.Index)
				}
			}
		}
	}
	return m, nil
}

func (m model) handleSettingsPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "s", "y":
		quit := m.settingsQuit
		if m.applySettingsDraft() {
			m.toMenu()
			if quit {
				return m, tea.Quit
			}
		}
	case "d", "n":
		quit := m.settingsQuit
		m.discardSettingsDraft()
		if quit {
			return m, tea.Quit
		}
	case "esc", "c":
		m.settingsPrompt = false
		m.settingsQuit = false
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ctrl+C: stop a running command first, quit on the next press — an
	// exec that ignores cancellation must never trap the user in the TUI
	if msg.Type == tea.KeyCtrlC {
		if m.state() == screenFilePicker && m.picker != nil && m.picker.scanner != nil {
			return m.closeFilePicker(false)
		}
		if m.isSetting {
			m.updateSettingsDirty()
			if m.settingsDirty {
				m.settingsPrompt = true
				m.settingsQuit = true
				return m, nil
			}
		}
		if m.running && m.state() == stateRun && !m.stopReq {
			return m.stopRun()
		}
		return m, tea.Quit
	}

	switch m.state() {
	case stateMenu:
		switch msg.String() {
		case "up", "k":
			m.moveMenu(-1)
		case "down", "j":
			m.moveMenu(1)
		case "left":
			m.moveMenu(-1)
		case "right":
			m.moveMenu(1)
		case "1", "2", "3":
			if chooseLayout(m.width, m.height) == layoutWide && m.actions[m.menuIx].id == "chatls" {
				ids := []string{"chatls", "chatexport", "chatusers"}
				choice := int(msg.String()[0] - '1')
				return m.openMenuItem(m.actionIndex(ids[choice]))
			}
		case "enter", " ":
			return m.openMenuItem(m.menuIx)
		case "esc", "q":
			return m, tea.Quit
		}

	case stateForm:
		if m.isSetting && m.settingsPrompt {
			return m, nil
		}
		if m.isSetting {
			m.updateSettingsDirty()
		}
		if m.isSetting && m.settingsButton >= 0 {
			switch msg.String() {
			case "tab", "right":
				m.settingsButton = (m.settingsButton + 1) % 2
			case "shift+tab", "left":
				m.settingsButton = (m.settingsButton + 1) % 2
			case "up":
				m.settingsButton = -1
				m.focusFormField(len(m.form.fields) - 1)
			case "enter", " ":
				if m.settingsButton == 0 {
					m.applySettingsDraft()
				} else {
					m.discardSettingsDraft()
				}
			case "esc":
				m.updateSettingsDirty()
				if m.settingsDirty {
					m.settingsPrompt = true
				} else {
					m.toMenu()
				}
			}
			return m, nil
		}
		f := &m.form.fields[m.formIx]
		switch msg.String() {
		case "p":
			if f.picker != nil {
				return m.openFilePicker(m.formIx)
			}
			if f.kind == kChat {
				return m.openChatSelector(m.formIx)
			}
			if f.kind.editable() {
				var cmd tea.Cmd
				m.form.fields[m.formIx].ti, cmd = f.ti.Update(msg)
				return m, cmd
			}
		case "up":
			m.focusFormField(m.formIx - 1)
		case "down", "tab":
			if m.isSetting && msg.String() == "tab" && m.formIx == len(m.form.fields)-1 {
				m.settingsButton = 0
				f.ti.Blur()
				return m, nil
			}
			m.focusFormField((m.formIx + 1) % len(m.form.fields))
		case "shift+tab":
			m.focusFormField(m.formIx - 1)
		case " ":
			// space is the only toggle key; enter is reserved for run/save
			if f.kind == kBool {
				f.boolVal = !f.boolVal
			} else if f.kind == kChoice {
				f.cycle(true)
			} else {
				var cmd tea.Cmd
				m.form.fields[m.formIx].ti, cmd = f.ti.Update(msg)
				return m, cmd
			}
		case "left", "right":
			if f.kind == kChoice {
				f.cycle(msg.String() == "right")
			} else if f.kind.editable() {
				var cmd tea.Cmd
				m.form.fields[m.formIx].ti, cmd = f.ti.Update(msg)
				return m, cmd
			}
		case "enter":
			if f.picker != nil {
				return m.openFilePicker(m.formIx)
			}
			if f.kind == kChat {
				return m.openChatSelector(m.formIx)
			}
			return m.launchForm()
		case "ctrl+enter":
			return m.launchForm()
		case "esc":
			if m.isSetting && m.settingsDirty {
				m.settingsPrompt = true
				return m, nil
			}
			m.toMenu()
		default:
			if f.kind.editable() {
				var cmd tea.Cmd
				m.form.fields[m.formIx].ti, cmd = f.ti.Update(msg)
				return m, cmd
			}
		}

	case screenFilePicker:
		if m.picker == nil {
			return m, nil
		}
		if m.picker.scanner != nil {
			switch msg.String() {
			case "esc", "q", "ctrl+c":
				return m.closeFilePicker(false)
			}
			return m, nil
		}
		if m.picker.request.Mode == PickSaveFile {
			switch msg.Type {
			case tea.KeyRunes, tea.KeySpace, tea.KeyBackspace, tea.KeyDelete, tea.KeyLeft, tea.KeyRight, tea.KeyHome, tea.KeyEnd:
				var cmd tea.Cmd
				m.picker.saveName, cmd = m.picker.saveName.Update(msg)
				return m, cmd
			}
		}
		switch msg.String() {
		case "up", "k":
			m.picker.cursor = max(0, m.picker.cursor-1)
		case "down", "j":
			m.picker.cursor = min(max(0, len(m.picker.entries)-1), m.picker.cursor+1)
		case "enter":
			if len(m.picker.entries) > 0 && m.picker.entries[m.picker.cursor].Dir {
				_ = m.picker.enterCurrent()
			} else {
				m.picker.toggleCurrent()
			}
		case " ":
			m.picker.toggleCurrent()
		case "backspace", "left", "h":
			_ = m.picker.parent()
		case "ctrl+h":
			m.picker.request.ShowHidden = !m.picker.request.ShowHidden
			_ = m.picker.refresh()
		case "r":
			_ = m.picker.refresh()
		case "s":
			m.picker.sortBy = (m.picker.sortBy + 1) % 4
			m.picker.sortEntries()
		case "c", "ctrl+enter":
			return m.closeFilePicker(true)
		case "esc", "q":
			return m.closeFilePicker(false)
		}

	case screenChatPicker:
		if m.chatPicker == nil {
			return m, nil
		}
		p := m.chatPicker
		switch msg.String() {
		case "up":
			p.cursor = max(0, p.cursor-1)
		case "down":
			p.cursor = min(max(0, len(p.filtered)-1), p.cursor+1)
		case "left":
			p.topic = max(-1, p.topic-1)
		case "right":
			if item, ok := p.current(); ok {
				p.topic = min(len(item.Topics)-1, p.topic+1)
			}
		case "enter", "c":
			return m.closeChatSelector(true)
		case "l":
			return m.loadMoreChats()
		case "r":
			if !p.loading {
				p.loading = true
				return m, loadChatPageCmd(m.ctx, m.chatSource, p.namespace, p.next, m.screenID)
			}
		case "m":
			return m.closeChatSelector(false)
		case "esc", "q":
			return m.closeChatSelector(false)
		default:
			before := p.search.Value()
			var cmd tea.Cmd
			p.search, cmd = p.search.Update(msg)
			if p.search.Value() != before {
				p.filter(p.search.Value())
			}
			return m, cmd
		}

	case screenConfirm:
		switch msg.String() {
		case "y", "enter":
			return m.retryFailed(true)
		case "n", "esc":
			m.retryPrompt = false
		}

	case screenErrorList:
		switch msg.String() {
		case "up", "k":
			m.errorCursor = max(0, m.errorCursor-1)
		case "down", "j":
			m.errorCursor = min(max(0, len(m.runResult.Items)-1), m.errorCursor+1)
		case "c":
			m.copySelectedError()
		case "r":
			m.errorListOpen = false
			return m.retryFailed(false)
		case "enter", "esc", "q":
			m.errorListOpen = false
		}

	case stateRun:
		if !m.running {
			switch msg.String() {
			case "enter", "esc":
				m.toMenu()
			case "r":
				return m.retryFailed(false)
			case "e":
				m.openErrorList()
			case "c":
				if m.progress.Status == xprogress.StatusCanceled {
					return m.retryFailed(false)
				}
			}
		}
		switch msg.String() {
		case "d":
			m.detailsOpen = !m.detailsOpen
		case "up", "pgup":
			m.follow = false
			m.vp.LineUp(m.vp.Height / 2)
		case "down", "pgdown":
			m.vp.LineDown(m.vp.Height / 2)
			if m.atBottom() {
				m.follow = true
			}
		}
	}
	return m, nil
}

func (f *field) cycle(fwd bool) {
	if len(f.choices) == 0 {
		return
	}
	if fwd {
		f.choiceIx = (f.choiceIx + 1) % len(f.choices)
	} else if f.choiceIx > 0 {
		f.choiceIx--
	} else {
		f.choiceIx = len(f.choices) - 1
	}
}

func (m *model) focusFormField(ix int) {
	if m.form == nil || len(m.form.fields) == 0 {
		return
	}
	if ix < 0 {
		ix = len(m.form.fields) - 1
	}
	ix %= len(m.form.fields)
	// only text inputs hold a usable cursor; zero-value inputs panic on Focus
	if m.formIx < len(m.form.fields) {
		if k := m.form.fields[m.formIx].kind; k.editable() {
			m.form.fields[m.formIx].ti.Blur()
		}
	}
	m.formIx = ix
	if k := m.form.fields[ix].kind; k.editable() {
		m.form.fields[ix].ti.Focus()
		m.form.fields[ix].ti.CursorStart()
	}
}

func (m model) openMenuItem(ix int) (tea.Model, tea.Cmd) {
	definitions := newActions()
	a := &definitions[ix]
	switch a.id {
	case "settings":
		m.openSettings()
		return m, nil
	case "login":
		// login prompts for phone/code/password on the console, which the
		// TUI owns: it cannot run in-process. Show guidance instead.
		m.showOutput = true
		m.appendLine(stErr.Render(m.lang.t("status.loginext")))
		return m, nil
	case "quit":
		return m, tea.Quit
	case "version":
		// no options: run directly
		return m.launchAction(a)
	default:
		m.form = a
		m.isSetting = false
		m.formIx = 0
		m.focusFormField(0)
		return m, nil
	}
}

func (m model) launchForm() (tea.Model, tea.Cmd) {
	if m.isSetting {
		m.applySettingsDraft()
		return m, nil
	}
	return m.launchAction(m.form)
}

func (m model) openFilePicker(fieldIndex int) (tea.Model, tea.Cmd) {
	if m.form == nil || fieldIndex < 0 || fieldIndex >= len(m.form.fields) {
		return m, nil
	}
	f := &m.form.fields[fieldIndex]
	if f.picker == nil {
		return m, nil
	}
	req := *f.picker
	if recent := m.set.RecentDirs[req.Purpose]; recent != "" {
		req.InitialDir = recent
	} else if value := strings.TrimSpace(f.ti.Value()); value != "" {
		candidate := value
		if req.Mode == PickSaveFile || req.Mode == PickOpenFile {
			candidate = filepath.Dir(candidate)
		}
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			req.InitialDir = candidate
		}
	}
	picker, err := newFilePicker(req)
	if err != nil {
		m.settingsError = err.Error()
		return m, nil
	}
	if req.Mode == PickSaveFile {
		name := filepath.Base(strings.TrimSpace(f.ti.Value()))
		if name == "." || name == "" {
			name = filepath.Base(f.def)
		}
		if name == "." || name == "" {
			if req.Purpose == "backup_destination" {
				name = time.Now().Format("20060102") + ".backup.tdl"
			} else {
				name = "output"
			}
		}
		picker.saveName.SetValue(name)
	}
	m.picker = picker
	m.pickerField = fieldIndex
	m.screenID++
	return m, nil
}

func (m model) openChatSelector(fieldIndex int) (tea.Model, tea.Cmd) {
	if m.form == nil || fieldIndex < 0 || fieldIndex >= len(m.form.fields) || m.form.fields[fieldIndex].kind != kChat {
		return m, nil
	}
	m.screenID++
	m.chatField = fieldIndex
	m.chatPicker = newChatPicker(m.currentNS(), m.screenID)
	return m, loadChatPageCmd(m.ctx, m.chatSource, m.currentNS(), nil, m.screenID)
}

func (m model) closeChatSelector(apply bool) (tea.Model, tea.Cmd) {
	if m.chatPicker == nil || m.form == nil || m.chatField >= len(m.form.fields) {
		m.chatPicker = nil
		return m, nil
	}
	if apply {
		item, ok := m.chatPicker.current()
		if !ok {
			m.chatPicker.err = m.lang.t("chat.empty")
			return m, nil
		}
		value := ""
		if !item.Self {
			value = strconv.FormatInt(item.ID, 10)
		}
		m.form.fields[m.chatField].ti.SetValue(value)
		for i := range m.form.fields {
			if m.form.fields[i].labelKey == "Topic id" {
				topic := ""
				if m.chatPicker.topic >= 0 && m.chatPicker.topic < len(item.Topics) {
					topic = strconv.Itoa(item.Topics[m.chatPicker.topic].ID)
				}
				m.form.fields[i].ti.SetValue(topic)
			}
		}
		if m.set.RecentChats == nil {
			m.set.RecentChats = make(map[string][]int64)
		}
		m.set.RecentChats[m.currentNS()] = updateRecentChats(m.set.RecentChats[m.currentNS()], item.ID)
		if err := saveSettings(m.set); err != nil {
			m.chatPicker.err = err.Error()
			return m, nil
		}
	}
	m.chatPicker = nil
	m.screenID++
	m.focusFormField(m.chatField)
	return m, nil
}

func (m model) loadMoreChats() (tea.Model, tea.Cmd) {
	if m.chatPicker == nil || m.chatPicker.next == nil || m.chatPicker.loading {
		return m, nil
	}
	m.chatPicker.loading = true
	return m, loadChatPageCmd(m.ctx, m.chatSource, m.chatPicker.namespace, m.chatPicker.next, m.screenID)
}

func (m model) closeFilePicker(apply bool) (tea.Model, tea.Cmd) {
	if m.picker == nil || m.form == nil || m.pickerField >= len(m.form.fields) {
		m.picker = nil
		return m, nil
	}
	if !apply {
		if m.picker.scanner != nil {
			m.picker.scanner.cancel()
			m.picker.scanner = nil
			m.picker.err = m.lang.t("picker.scan.canceled")
			return m, nil
		}
		m.picker = nil
		m.screenID++
		return m, nil
	}
	f := &m.form.fields[m.pickerField]
	paths := m.picker.selectedPaths()
	switch m.picker.request.Mode {
	case PickSaveFile:
		name := filepath.Base(strings.TrimSpace(m.picker.saveName.Value()))
		if name == "." || name == "" {
			name = filepath.Base(f.def)
		}
		if name == "." || name == "" {
			name = "output"
		}
		if len(m.picker.request.AllowedExt) > 0 {
			ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
			allowed := strings.TrimPrefix(strings.ToLower(m.picker.request.AllowedExt[0]), ".")
			if ext == "" {
				name += "." + allowed
			} else if ext != allowed {
				m.picker.err = fmt.Sprintf(m.lang.t("picker.save.extension"), allowed)
				return m, nil
			}
		}
		paths = []string{filepath.Join(m.picker.cwd, name)}
	case PickDirectory:
		paths = []string{m.picker.cwd}
	}
	if len(paths) == 0 {
		m.picker.err = m.lang.t("picker.empty")
		return m, nil
	}
	m.picker.pendingPaths = append([]string(nil), paths...)
	if m.picker.request.Mode == PickSaveFile || m.picker.request.Mode == PickDirectory {
		return m.applyFilePickerPlan(SelectionPlan{Paths: append([]string(nil), paths...)})
	}
	if m.picker.scanner != nil {
		return m, nil
	}
	m.pickerScanID++
	ctx, cancel := context.WithCancel(m.ctx)
	scanner := &selectionScanner{id: m.pickerScanID, cancel: cancel}
	m.picker.scanner = scanner
	m.picker.err = ""
	screenID := m.screenID
	req := m.picker.request
	selected := append([]string(nil), paths...)
	scanCmd := func() tea.Msg {
		plan, err := scanSelectionPlan(ctx, req, selected, scanner)
		return selectionScanMsg{screenID: screenID, scanID: scanner.id, plan: plan, err: err}
	}
	tickCmd := tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return selectionScanTickMsg{screenID: screenID, scanID: scanner.id}
	})
	return m, tea.Batch(scanCmd, tickCmd)
}

func (m model) applyFilePickerPlan(plan SelectionPlan) (tea.Model, tea.Cmd) {
	return m.applyFilePickerPlanMode(plan, false)
}

func (m model) applyFilePickerPlanMode(plan SelectionPlan, skipProblems bool) (tea.Model, tea.Cmd) {
	if m.picker == nil || m.form == nil || m.pickerField >= len(m.form.fields) {
		return m, nil
	}
	f := &m.form.fields[m.pickerField]
	paths := append([]string(nil), m.picker.pendingPaths...)
	if len(paths) == 0 {
		paths = append(paths, plan.Paths...)
	}
	if !skipProblems && m.picker.request.Mode != PickSaveFile && len(plan.Problems) > 0 {
		copyPlan := plan
		m.picker.problemPlan = &copyPlan
		m.picker.err = plan.Problems[0].Path + ": " + plan.Problems[0].Err
		return m, nil
	}
	if m.picker.request.Mode == PickFilesAndDirectories {
		paths = paths[:0]
		for _, selected := range plan.Files {
			paths = append(paths, selected.Path)
		}
	}
	f.paths = append([]string(nil), paths...)
	if len(paths) == 1 {
		f.ti.SetValue(paths[0])
	} else {
		f.ti.SetValue(fmt.Sprintf("%d items · %s", len(paths), formatBytes(plan.TotalBytes)))
	}
	if m.set.RecentDirs == nil {
		m.set.RecentDirs = make(map[string]string)
	}
	m.set.RecentDirs[m.picker.request.Purpose] = m.picker.cwd
	if err := saveSettings(m.set); err != nil {
		m.picker.err = err.Error()
		return m, nil
	}
	m.picker = nil
	m.screenID++
	m.focusFormField(m.pickerField)
	return m, nil
}

func (m model) launchAction(a *action) (tea.Model, tea.Cmd) {
	spec := a.formSpec()
	run, err := spec.build(a.formValues(), m.set)
	if err != nil {
		m.formError = err.Error()
		return m, nil
	}
	m.formError = ""
	run.Display = RunDisplay{Title: a.title(m.lang), Summary: a.desc(m.lang)}
	return m.launchRunSpec(run)
}

func (m model) launchRunSpec(run RunSpec) (tea.Model, tea.Cmd) {
	argv := append([]string(nil), run.Args...)
	m.currentRun = run

	m.vp = viewport.New(m.width, m.mainHeight())
	m.vp.SetContent("")
	m.follow = true
	m.appendLine(stCmdEcho.Render("$ tdl " + quoteJoin(argv)))

	m.running = true
	m.showOutput = true
	m.runErr = nil
	m.runLabel = run.Display.Title
	m.runCommand = "$ tdl " + quoteJoin(argv)
	m.runStart = time.Now()
	m.live = ""
	m.stopReq = false
	m.lastLoggedFile = ""
	m.lastLoggedPct = -1
	m.previewPath = ""
	m.previewText = ""
	m.previewErr = ""
	m.previewLoading = false

	m.runID++
	m.progressCollector = xprogress.NewCollector()
	m.progress = m.progressCollector.Snapshot()
	m.detailsOpen = chooseLayout(m.width, m.height) == layoutWide
	m.stopReq = false
	m.cancelRun = startRun(m.ctx, m.program, m.exec, argv, m.runID)
	return m, nil
}

func (m model) retryFailed(forceUncertain bool) (tea.Model, tea.Cmd) {
	if len(m.runResult.Items) == 0 {
		return m, nil
	}
	var paths []string
	cleanupCount := 0
	for _, item := range m.runResult.Items {
		if item.Retry == RetryUncertain && !forceUncertain {
			m.retryPrompt = true
			return m, nil
		}
		if item.Retry == RetryCleanupOnly {
			if item.SourcePath != "" {
				if err := os.Remove(item.SourcePath); err != nil {
					m.settingsError = err.Error()
					return m, nil
				}
			}
			cleanupCount++
			continue
		}
		if item.SourcePath != "" {
			paths = append(paths, item.SourcePath)
		}
	}
	if len(paths) == 0 {
		if cleanupCount == len(m.runResult.Items) {
			m.runResult = RunResult{}
			m.progress.Failed = max(0, m.progress.Failed-cleanupCount)
			if m.progress.Failed == 0 {
				m.progress.Status = xprogress.StatusDone
			}
			m.settingsError = ""
			return m, nil
		}
		if m.currentRun.ActionID == "dl" {
			m.retryPrompt = false
			return m.launchRunSpec(m.currentRun)
		}
		m.settingsError = m.lang.t("retry.nopath")
		return m, nil
	}
	run := m.currentRun
	if run.ActionID == "up" {
		run.Args = replaceRepeatedFlag(run.Args, "-p", paths)
		run.Inputs = run.Inputs[:0]
		for _, path := range paths {
			run.Inputs = append(run.Inputs, InputRef{Path: path, Kind: "file"})
		}
	}
	m.retryPrompt = false
	return m.launchRunSpec(run)
}

func (m *model) openErrorList() {
	if len(m.runResult.Items) == 0 {
		return
	}
	m.errorCursor = min(m.errorCursor, len(m.runResult.Items)-1)
	m.errorListOpen = true
	m.clipboardNotice = ""
}

func (m *model) copySelectedError() {
	if len(m.runResult.Items) == 0 || m.errorCursor < 0 || m.errorCursor >= len(m.runResult.Items) {
		return
	}
	item := m.runResult.Items[m.errorCursor]
	value := strings.TrimSpace(item.DisplayName + "\n" + item.SourcePath + "\n" + item.Phase + "\n" + item.Err)
	if m.clipboard == nil {
		m.clipboardNotice = m.lang.t("errors.copy.failed")
		return
	}
	if err := m.clipboard.Copy(value); err != nil {
		m.clipboardNotice = m.lang.t("errors.copy.failed") + ": " + err.Error()
		return
	}
	m.clipboardNotice = m.lang.t("errors.copied")
}

func replaceRepeatedFlag(args []string, flag string, values []string) []string {
	out := make([]string, 0, len(args)+len(values)*2)
	for i := 0; i < len(args); i++ {
		if args[i] == flag && i+1 < len(args) {
			i++
			continue
		}
		out = append(out, args[i])
	}
	for _, value := range values {
		out = append(out, flag, value)
	}
	return out
}

func (m *model) applyProgress(event xprogress.Event) {
	if m.progressCollector == nil {
		m.progressCollector = xprogress.NewCollector()
	}
	m.progressCollector.Emit(event)
	m.progress = m.progressCollector.Snapshot()
}
func (m model) progressPercent() (float64, bool) { return m.progress.Percent() }

func (m model) stopRun() (tea.Model, tea.Cmd) {
	m.stopReq = true
	if m.cancelRun != nil {
		m.cancelRun()
	}
	return m, nil
}

// switchNS makes ns the account every subsequent command runs under and
// persists it: tdl sessions live per namespace, so switching to another
// logged-in account needs no separate login.
func (m model) switchNS(ns string) (tea.Model, tea.Cmd) {
	if ns == m.currentNS() {
		return m, nil
	}
	candidate := m.set
	candidate.NS = ns
	if err := saveSettings(candidate); err != nil {
		m.settingsError = err.Error()
		return m, nil
	}
	m.set = candidate
	return m, nil
}

func (m *model) toMenu() {
	m.screenID++
	m.runID++ // invalidate late messages from the completed/aborted run
	m.form = nil
	m.isSetting = false
	m.showOutput = false
	m.live = ""
	m.scrollback = nil
	m.scrollTimes = nil
	m.lastLoggedFile = ""
	m.lastLoggedPct = -1
	m.progress = xprogress.Snapshot{}
	m.progressCollector = nil
	m.detailsOpen = false
	m.follow = true
	m.stopReq = false
	m.runLabel = ""
	m.runCommand = ""
	m.settingsPrompt = false
	m.settingsQuit = false
	m.settingsError = ""
	m.formError = ""
	m.settingsButton = -1
	m.runErr = nil
	m.runLast = 0
	m.cancelRun = nil
	m.currentRun = RunSpec{}
	m.runResult = RunResult{}
	m.previewPath = ""
	m.previewText = ""
	m.previewErr = ""
	m.previewLoading = false
	m.picker = nil
	m.pickerField = 0
	m.chatPicker = nil
	m.chatField = 0
	m.retryPrompt = false
	m.errorListOpen = false
	m.errorCursor = 0
	m.clipboardNotice = ""
	m.vp.SetContent("")
}

func (m *model) startMediaPreview(path string) tea.Cmd {
	m.previewPath = path
	m.previewText = ""
	m.previewErr = ""
	m.previewLoading = true
	renderer := m.mediaPreview
	profile := m.colorProfile
	screenID, runID := m.screenID, m.runID
	width, height := 24, 8
	if chooseLayout(m.width, m.height) == layoutWide {
		previewW := min(32, max(24, m.contentWidth()/5))
		topH := min(18, max(12, m.mainHeight()*38/100))
		width, height = max(8, previewW-4), max(3, topH-5)
	}
	return func() tea.Msg {
		if renderer == nil {
			return mediaPreviewMsg{screenID: screenID, runID: runID, path: path, err: fmt.Errorf("media preview unavailable")}
		}
		text, err := renderer.Render(path, width, height, profile)
		return mediaPreviewMsg{screenID: screenID, runID: runID, path: path, text: text, err: err}
	}
}

func (m model) state() state {
	switch {
	case m.picker != nil:
		return screenFilePicker
	case m.chatPicker != nil:
		return screenChatPicker
	case m.retryPrompt:
		return screenConfirm
	case m.errorListOpen:
		return screenErrorList
	case m.running || m.showOutput:
		return stateRun
	case m.form != nil:
		return stateForm
	default:
		return stateMenu
	}
}

// ---------------------------------------------------------------------------
// scrollback

func (m *model) appendLine(line string) {
	m.scrollback = append(m.scrollback, line)
	m.scrollTimes = append(m.scrollTimes, time.Now())
	if len(m.scrollback) > maxScrollbackLines {
		m.scrollback = m.scrollback[len(m.scrollback)-maxScrollbackLines:]
		m.scrollTimes = m.scrollTimes[len(m.scrollTimes)-maxScrollbackLines:]
	}
	m.renderViewport()
}

func (m *model) appendProgressLog() {
	if m.progress.CurrentFile != "" && m.progress.CurrentFile != m.lastLoggedFile {
		m.lastLoggedFile = m.progress.CurrentFile
		m.lastLoggedPct = -1
		m.appendLine("开始传输 " + m.progress.CurrentFile)
	}
	if pct, ok := m.progress.Percent(); ok {
		step := int(pct) / 10 * 10
		if step >= m.lastLoggedPct+10 {
			m.lastLoggedPct = step
			m.appendLine(fmt.Sprintf("已传输 %s / %s  速度 %s/s  剩余 %s", xprogress.Bytes(m.progress.CompletedBytes), xprogress.Bytes(m.progress.TotalBytes), xprogress.Bytes(int64(m.progress.Speed)), m.progress.ETA()))
		}
	}
}

func (m *model) renderViewport() {
	m.vp.Width = m.width
	m.vp.Height = max(1, m.mainHeight()-len(m.runSummaryRows()))
	if m.follow {
		m.vp.SetContent(strings.Join(m.scrollback, "\n"))
		m.vp.GotoBottom()
	} else {
		y := m.vp.YOffset
		m.vp.SetContent(strings.Join(m.scrollback, "\n"))
		m.vp.SetYOffset(y)
	}
}

func (m model) atBottom() bool {
	return m.vp.YOffset+m.vp.Height >= m.vp.TotalLineCount()
}

// ---------------------------------------------------------------------------
// view

func (m model) mainHeight() int {
	switch chooseLayout(m.width, m.height) {
	case layoutWide:
		return max(6, m.height-wideHeaderHeight-wideFooterHeight)
	case layoutCompact:
		return max(3, m.height-3) // header + compact navigation + footer
	}
	h := m.height - 6 // header 1 + status 1 + prompt 3 + shortcuts 1
	if h < 3 {
		h = 3
	}
	return h
}

func (m model) contentWidth() int {
	if m.contentWidthOverride > 0 {
		return m.contentWidthOverride
	}
	return max(1, m.width-m.sidebarWidth())
}

func (m model) sidebarWidth() int {
	switch chooseLayout(m.width, m.height) {
	case layoutWide:
		return min(wideSidebarWidth, max(0, m.width-64))
	case layoutStandard:
		return min(18, max(0, m.width-40))
	default:
		return 0
	}
}

func (m model) contentTop() int {
	if chooseLayout(m.width, m.height) == layoutWide {
		return wideHeaderHeight
	}
	top := 1 // session header
	if m.sidebarWidth() == 0 {
		top++ // compact navigation
	}
	return top
}

func (m model) View() string { return m.frame().Text }

func (m model) frame() Frame {
	if m.width == 0 {
		return Frame{Text: m.lang.t("status.loading")}
	}
	if chooseLayout(m.width, m.height) == layoutWide {
		return m.wideFrame()
	}
	if chooseLayout(m.width, m.height) == layoutCompact {
		return m.compactFrame()
	}

	var b []string

	// header: one clickable chip per logged-in account
	prefix := stBrand.Render(brandName) + "  " + stBanner.Render(m.lang.t("banner.title")) + "  "
	if chips := m.nsChipLayout(m.width); len(chips) > 0 {
		parts := []string{prefix}
		for _, c := range chips {
			if c.ns == m.currentNS() {
				parts = append(parts, stOK.Render("● "+c.ns))
			} else {
				parts = append(parts, stHint.Render("○ "+c.ns))
			}
		}
		b = append(b, strings.Join(parts, "  "))
	} else {
		b = append(b, prefix+stHint.Render("○ "+m.lang.t("hdr.nosession")))
	}

	var content string
	switch m.state() {
	case stateMenu:
		content = m.viewMenu()
	case stateForm:
		content = m.viewForm()
	case screenFilePicker:
		content = m.viewFilePicker()
	case screenChatPicker:
		content = m.viewChatPicker()
	case screenConfirm:
		content = m.viewRetryConfirm()
	case screenErrorList:
		content = m.viewErrorList()
	default:
		content = m.viewRun()
	}
	if sw := m.sidebarWidth(); sw > 0 {
		content = lipgloss.JoinHorizontal(lipgloss.Top, m.viewSidebar(sw), content)
	} else {
		content = lipgloss.JoinVertical(lipgloss.Left, m.viewCompactNav(), content)
	}
	b = append(b, content)

	// Context status and action line for standard and compact layouts.
	b = append(b, m.viewStatus())

	// prompt box
	b = append(b, m.viewPrompt())

	// shortcuts bar
	b = append(b, m.viewShortcuts())

	for i := range b {
		b[i] = fitScreen(b[i], m.width, 0)
	}
	text := fitScreen(lipgloss.JoinVertical(lipgloss.Left, b...), m.width, m.height)
	return Frame{Text: text, Regions: m.hitRegions()}
}

// menuShowsLogo is retained for compatibility with callers from phase one.
// The phase-two workbench deliberately uses a compact wordmark.
func (m model) menuShowsLogo() bool {
	return false
}

// menuWindow computes where the menu items live on screen: the Y of the
// first visible row, the index of the first visible item, and how many
// items are visible. It is a pure function of the model, so View and
// mouse hit-testing always agree.
func (m model) menuWindow() (firstY, firstIx, count int) {
	firstY = 1  // header
	firstY += 2 // title + blank

	n := len(m.actions)
	avail := m.height - 5 - firstY // rows below the menu: status 1 + prompt 3 + shortcuts 1
	if avail < 1 {
		avail = 1
	}
	if n <= avail {
		return firstY, 0, n
	}
	// window the list around the selection
	firstIx = m.menuIx - avail/2
	if firstIx < 0 {
		firstIx = 0
	}
	if max := n - avail; firstIx > max {
		firstIx = max
	}
	return firstY, firstIx, avail
}

// nsChip is one clickable account chip on the header row.
type nsChip struct {
	ns     string
	x0, x1 int // half-open [x0, x1) in terminal columns
}

func (m model) actionIndex(id string) int {
	for i := range m.actions {
		if m.actions[i].id == id {
			return i
		}
	}
	return 0
}

func (m model) menuSequence() []int {
	if chooseLayout(m.width, m.height) != layoutWide {
		sequence := make([]int, len(m.actions))
		for i := range sequence {
			sequence[i] = i
		}
		return sequence
	}
	ids := []string{"up", "dl", "chatls", "forward", "backup", "recover", "settings", "login", "update", "version", "quit"}
	sequence := make([]int, 0, len(ids))
	for _, id := range ids {
		sequence = append(sequence, m.actionIndex(id))
	}
	return sequence
}

func (m *model) normalizeWideMenu() {
	if chooseLayout(m.width, m.height) != layoutWide {
		return
	}
	for _, index := range m.menuSequence()[:7] {
		if m.menuIx == index {
			return
		}
	}
	m.menuIx = m.actionIndex("up")
}

func (m *model) moveMenu(delta int) {
	sequence := m.menuSequence()
	position := 0
	for i, index := range sequence {
		if index == m.menuIx {
			position = i
			break
		}
	}
	position = (position + delta + len(sequence)) % len(sequence)
	m.menuIx = sequence[position]
}

// currentNS is the namespace every executed command runs under.
func (m model) currentNS() string {
	if m.set.NS == "" {
		return "default"
	}
	return m.set.NS
}

// nsChipLayout computes where each account chip sits on the header line,
// dropping chips that do not fit. Pure function of the model so View and
// mouse hit-testing always agree.
func (m model) nsChipLayout(width int) []nsChip {
	if len(m.namespaces) == 0 {
		return nil
	}
	x := lipgloss.Width(stBrand.Render(brandName)) + 2 +
		lipgloss.Width(stBanner.Render(m.lang.t("banner.title"))) + 2
	if chooseLayout(m.width, m.height) == layoutWide {
		prefix := m.lang.t("header.session") + ": Workstation  │  " + m.lang.t("header.account") + ": "
		x = m.sidebarWidth() + 2 + lipgloss.Width(prefix)
	}
	cur := m.currentNS()
	var out []nsChip
	for _, ns := range m.namespaces {
		label := "○ " + ns
		if ns == cur {
			label = "● " + ns
		}
		w := lipgloss.Width(stHint.Render(label))
		if x+w > width {
			break
		}
		out = append(out, nsChip{ns: ns, x0: x, x1: x + w})
		x += w + 2
	}
	return out
}

func (m model) viewMenu() string {
	if chooseLayout(m.width, m.height) == layoutCompact {
		return m.viewMenuCompact()
	}
	if chooseLayout(m.width, m.height) == layoutWide && m.actions[m.menuIx].id == "chatls" {
		return m.viewChatHub()
	}
	a := m.actions[m.menuIx]
	width := m.contentWidth()
	account := m.currentNS()
	if len(m.namespaces) == 0 {
		account = m.lang.t("hdr.nosession")
	}
	card := []string{
		stHint.Render(strings.ToUpper(m.lang.t("menu.title"))),
		stTitle.Render(a.title(m.lang)),
		stHint.Render(a.desc(m.lang)),
		"",
		stFieldLabel.Render(m.lang.t("home.account")) + "  " + stSys.Render(account),
		stFieldLabel.Render(m.lang.t("home.concurrent")) + "  " + stSys.Render(defaultText(m.set.Limit, "2")),
		"",
		stFieldFocus.Render("[ "+m.lang.t("form.open")+" ]") + "  " + stHint.Render(m.lang.t("home.openhint")),
	}
	if a.id == "login" && m.hasSession {
		card = append(card, stOK.Render("✓ "+strings.Join(m.namespaces, ", ")))
	}
	panel := activeTheme.panelRaised.Width(max(1, width-6)).Render(strings.Join(card, "\n"))
	return lipgloss.NewStyle().Width(width).Height(m.mainHeight()).Padding(1, 2).Render(panel)
}

func (m model) viewMenuCompact() string {
	a := m.actions[m.menuIx]
	account := m.currentNS()
	if len(m.namespaces) == 0 {
		account = m.lang.t("hdr.nosession")
	}
	rows := []string{
		stFieldFocus.Render(a.title(m.lang)),
		activeTheme.text.Width(max(1, m.width-2)).Render(a.desc(m.lang)),
		stHint.Render(m.lang.t("home.account")+": ") + stSys.Render(account),
		stHint.Render(m.lang.t("home.concurrent")+": ") + stSys.Render(defaultText(m.set.Limit, "2")),
		stFieldFocus.Render("[ " + m.lang.t("form.open") + " ]"),
	}
	return fitScreen(strings.Join(rows, "\n"), m.contentWidth(), m.mainHeight())
}

func defaultText(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func (m model) viewSidebar(width int) string {
	rows := []string{
		stBrand.Render(" TDL"),
		stHint.Render(ansi.Truncate(" Telegram Media Transfer", max(1, width-1), "…")),
		stHint.Render(strings.Repeat("─", max(1, width-1))),
	}
	for i, a := range m.actions {
		label := "  " + a.title(m.lang)
		if i == m.menuIx {
			label = "▌ " + a.title(m.lang)
			rows = append(rows, stItemSelected.Width(max(1, width-2)).Render(ansi.Truncate(label, max(1, width-2), "…")))
		} else {
			rows = append(rows, stItem.Width(max(1, width-2)).Render(ansi.Truncate(label, max(1, width-2), "…")))
		}
	}
	return lipgloss.NewStyle().Width(width).Height(m.mainHeight()).Background(colour(activeTheme.palette.Sidebar)).Render(strings.Join(rows, "\n"))
}

func (m model) viewCompactNav() string {
	a := m.actions[m.menuIx]
	return activeTheme.status.Width(max(1, m.width)).Render(stBrand.Render("‹ ") + stFieldFocus.Render(a.title(m.lang)) + stHint.Render("  "+m.lang.t("sc.leftright")))
}

func (m model) hitRegions() []HitRegion {
	regions := make([]HitRegion, 0, len(m.actions)+len(m.namespaces)+len(m.formFields()))
	if m.state() == screenConfirm {
		_, _, x, y := m.retryConfirmGeometry()
		retryLabel := "[ " + m.lang.t("retry.anyway") + " ]"
		cancelLabel := "[ " + m.lang.t("picker.cancel") + " ]"
		retryX := x + 2
		buttonY := y + 5
		return []HitRegion{
			{ID: "retry.anyway", Rect: Rect{X: retryX, Y: buttonY, W: lipgloss.Width(retryLabel), H: 1}, Enabled: true, Action: UIAction{Kind: UIActionButton, ID: "run.retry"}},
			{ID: "retry.cancel", Rect: Rect{X: retryX + lipgloss.Width(retryLabel) + 2, Y: buttonY, W: lipgloss.Width(cancelLabel), H: 1}, Enabled: true, Action: UIAction{Kind: UIActionButton, ID: "retry.cancel"}},
		}
	}
	accountY := 0
	if chooseLayout(m.width, m.height) == layoutWide {
		accountY = 1
	}
	for _, c := range m.nsChipLayout(m.width) {
		regions = append(regions, HitRegion{ID: "account:" + c.ns, Rect: Rect{X: c.x0, Y: accountY, W: c.x1 - c.x0, H: 1}, Enabled: m.state() == stateMenu && !m.settingsPrompt, Action: UIAction{Kind: UIActionAccount, ID: c.ns}})
	}
	if sw := m.sidebarWidth(); sw > 0 {
		if chooseLayout(m.width, m.height) == layoutWide {
			for i, entry := range m.widePrimaryNav() {
				regions = append(regions, HitRegion{ID: "menu:" + m.actions[entry.actionIndex].id, Rect: Rect{X: 0, Y: 5 + i, W: sw, H: 1}, Enabled: m.state() == stateMenu, Action: UIAction{Kind: UIActionMenu, Index: entry.actionIndex, ID: m.actions[entry.actionIndex].id}})
			}
			systemX, systemY := 2, max(0, m.height-wideFooterHeight-6)
			for _, entry := range m.wideSystemNav() {
				w := lipgloss.Width(entry.label)
				regions = append(regions, HitRegion{ID: "menu:" + m.actions[entry.actionIndex].id, Rect: Rect{X: systemX, Y: systemY, W: w, H: 1}, Enabled: m.state() == stateMenu, Action: UIAction{Kind: UIActionMenu, Index: entry.actionIndex, ID: m.actions[entry.actionIndex].id}})
				systemX += w + 3
			}
		} else {
			for i := range m.actions {
				regions = append(regions, HitRegion{ID: "menu:" + m.actions[i].id, Rect: Rect{X: 0, Y: 4 + i, W: sw, H: 1}, Enabled: m.state() == stateMenu, Action: UIAction{Kind: UIActionMenu, Index: i, ID: m.actions[i].id}})
			}
		}
	} else if chooseLayout(m.width, m.height) == layoutCompact && m.state() == stateMenu {
		regions = append(regions, HitRegion{ID: "menu:" + m.actions[m.menuIx].id, Rect: Rect{X: 0, Y: m.contentTop(), W: m.width, H: m.mainHeight()}, Enabled: true, Action: UIAction{Kind: UIActionMenu, Index: m.menuIx, ID: m.actions[m.menuIx].id}})
	}
	if m.state() == stateForm && m.form != nil {
		x := m.sidebarWidth()
		fieldWidth := m.contentWidth()
		fieldYBase := m.contentTop() + 2
		selectorRight := m.width - 1
		if chooseLayout(m.width, m.height) == layoutWide {
			x += 2
			fieldWidth = max(1, m.formPrimaryWidth()-6)
			fieldYBase = m.contentTop() + 3
			selectorRight = m.sidebarWidth() + m.formPrimaryWidth() - 3
		}
		first := max(0, m.formIx-max(1, m.mainHeight()-3)+1)
		last := min(len(m.form.fields), first+max(1, m.mainHeight()-2))
		for i := first; i < last; i++ {
			y := fieldYBase + i - first
			regions = append(regions, HitRegion{ID: fmt.Sprintf("field:%d", i), Rect: Rect{X: x, Y: y, W: fieldWidth, H: 1}, Enabled: true, Action: UIAction{Kind: UIActionField, Index: i}})
			if m.form.fields[i].picker != nil {
				buttonW := lipgloss.Width(m.selectorButtonText(&m.form.fields[i]))
				regions = append(regions, HitRegion{ID: fmt.Sprintf("picker.open:%d", i), Rect: Rect{X: max(x, selectorRight-buttonW), Y: y, W: min(buttonW, fieldWidth), H: 1}, Enabled: true, Action: UIAction{Kind: UIActionButton, ID: fmt.Sprintf("picker.open:%d", i)}})
			}
			if m.form.fields[i].kind == kChat {
				buttonW := lipgloss.Width(m.selectorButtonText(&m.form.fields[i]))
				regions = append(regions, HitRegion{ID: fmt.Sprintf("chat.open:%d", i), Rect: Rect{X: max(x, selectorRight-buttonW), Y: y, W: min(buttonW, fieldWidth), H: 1}, Enabled: true, Action: UIAction{Kind: UIActionButton, ID: fmt.Sprintf("chat.open:%d", i)}})
			}
		}
	}
	regions = append(regions, m.pickerRegions()...)
	regions = append(regions, m.chatRegions()...)
	regions = append(regions, m.errorListRegions()...)
	regions = append(regions, m.chatHubRegions()...)
	if chooseLayout(m.width, m.height) == layoutWide && m.state() == stateRun && m.running {
		for _, button := range m.runButtons() {
			regions = append(regions, HitRegion{ID: "button:" + button.id, Rect: Rect{X: button.x0, Y: button.y, W: button.x1 - button.x0, H: 1}, Enabled: true, Action: UIAction{Kind: UIActionButton, ID: button.id}})
		}
		return regions
	}
	var buttons []uiButton
	if m.isSetting {
		buttons = m.settingsButtons()
	} else if m.state() == stateForm {
		buttons = m.formButtons()
	} else if m.state() == stateRun {
		buttons = m.runButtons()
	}
	for _, b := range buttons {
		regions = append(regions, HitRegion{ID: "button:" + b.id, Rect: Rect{X: b.x0, Y: b.y, W: b.x1 - b.x0, H: 1}, Enabled: true, Action: UIAction{Kind: UIActionButton, ID: b.id}})
	}
	return regions
}

func (m model) selectorButtonText(f *field) string {
	label := m.lang.t("picker.select")
	if f != nil && f.kind == kChat {
		label = m.lang.t("chat.select")
	}
	return "[ " + label + " ]"
}

func (m model) formFields() []field {
	if m.form == nil {
		return nil
	}
	return m.form.fields
}

func (m model) viewForm() string {
	if chooseLayout(m.width, m.height) == layoutWide {
		return m.viewFormWide()
	}
	return m.viewFormClassic()
}

func (m model) viewFormClassic() string {
	width := m.contentWidth()
	subtitle := "$ tdl " + quoteJoin(m.form.argv(m.set.globalArgs()))
	if m.isSetting {
		subtitle = m.lang.t("set.global")
	}
	rows := []string{stTitle.Render(m.form.title(m.lang)), stCmdEcho.Render(subtitle)}
	first := max(0, m.formIx-max(1, m.mainHeight()-3)+1)
	last := min(len(m.form.fields), first+max(1, m.mainHeight()-2))
	for i := first; i < last; i++ {
		f := &m.form.fields[i]
		cursor := "  "
		label := stFieldLabel.Render(f.label(m.lang))
		var value string
		switch f.kind {
		case kBool:
			onoff := m.lang.t("form.bool.off")
			if f.boolVal {
				onoff = stOK.Render(m.lang.t("form.bool.on"))
			}
			value = "[" + onoff + stFieldFlag.Render("]") + "  " + stFieldFlag.Render(f.flag)
		case kChoice:
			chs := make([]string, 0, len(f.choices))
			for j, c := range f.choices {
				if c == "" {
					continue
				}
				if j == f.choiceIx {
					chs = append(chs, stFieldFocus.Render(localizedChoice(m.lang, c)))
				} else {
					chs = append(chs, stHint.Render(localizedChoice(m.lang, c)))
				}
			}
			value = strings.Join(chs, stShortcutSep.Render("/")) + "  " + stFieldFlag.Render(f.flag)
		case kPicker, kChat:
			display := strings.TrimSpace(f.ti.Value())
			if len(f.paths) == 1 {
				display = f.paths[0]
			} else if len(f.paths) > 1 {
				display = fmt.Sprintf("%d items", len(f.paths))
			}
			if display == "" {
				display = localizedPlaceholder(m.lang, f.placeholder)
			}
			budget := max(8, width-lipgloss.Width(f.label(m.lang))-20)
			value = stFieldValue.Render(ansi.Truncate(display, budget, "…"))
		default:
			input := f.ti
			input.Placeholder = localizedPlaceholder(m.lang, f.placeholder)
			input.Width = max(1, width-lipgloss.Width(f.label(m.lang))-lipgloss.Width(f.flag)-10)
			value = stFieldValue.Render(input.View()) + "  " + stFieldFlag.Render(f.flag)
		}
		if i == m.formIx {
			cursor = stFieldFocus.Render("▸ ")
			label = stFieldFocus.Render(f.label(m.lang))
		}
		row := cursor + label + ": " + value
		if f.picker != nil || f.kind == kChat {
			row = joinEdges(row, stFieldFocus.Render(m.selectorButtonText(f)), max(1, width-2))
		}
		rows = append(rows, ansi.Truncate(row, max(1, width-2), "…"))
	}
	if m.formError != "" {
		rows = append(rows, stErr.Render(ansi.Truncate(m.formError, max(1, width-2), "…")))
	}
	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return lipgloss.NewStyle().Width(width).Height(m.mainHeight()).Padding(0, 1).Render(body)
}

func (m model) viewPrompt() string {
	if m.settingsPrompt {
		return stPromptBorderActive.Width(m.width - 2).Render(stHint.Render(m.lang.t("set.unsaved") + " " + m.settingsError))
	}
	var text string
	switch m.state() {
	case stateForm:
		argv := m.form.argv(m.set.globalArgs())
		if m.isSetting {
			text = stCmdEcho.Render(m.currentFieldHelp())
			if m.settingsError != "" {
				text = stErr.Render(m.settingsError)
			}
		} else {
			_ = argv
			if m.formError != "" {
				text = stErr.Render(m.formError)
			} else {
				text = stHint.Render(m.currentFieldHelp())
			}
		}
	case screenFilePicker:
		text = stHint.Render(m.lang.t("picker.help"))
	case screenChatPicker:
		text = stHint.Render(m.lang.t("chat.help"))
	case stateRun:
		if m.running {
			text = stHint.Render(m.lang.t("status.running") + " · ctrl+c " + m.lang.t("sc.ctrlc"))
		} else {
			text = stHint.Render(m.lang.t("status.ready"))
		}
	default:
		text = stBrand.Render("tdl tui") + stHint.Render("  ·  "+m.lang.t("banner.sub"))
	}
	box := stPromptBorderActive
	if m.state() == stateRun && !m.running {
		box = stPromptBorder
	}
	return box.Width(max(1, m.width-2)).MaxHeight(3).Render(text)
}

func (m model) viewShortcuts() string {
	sc := func(parts ...string) string {
		var out []string
		for i := 0; i+1 < len(parts); i += 2 {
			out = append(out, stShortcutKey.Render(parts[i])+" "+parts[i+1])
		}
		return strings.Join(out, stShortcutSep.Render(" · "))
	}
	switch m.state() {
	case stateMenu:
		return sc("↑↓", m.lang.t("sc.updown"), "enter", m.lang.t("sc.enter"), "esc", m.lang.t("sc.quit"))
	case stateForm:
		if m.isSetting {
			return sc("tab", m.lang.t("sc.updown"), "enter", m.lang.t("set.apply"), "esc", m.lang.t("form.back"))
		}
		if m.form != nil && m.formIx >= 0 && m.formIx < len(m.form.fields) {
			f := &m.form.fields[m.formIx]
			if f.picker != nil || f.kind == kChat {
				return sc("↑↓/tab", m.lang.t("sc.updown"), "enter", m.lang.t("picker.select"), "ctrl+enter", m.lang.t("form.run"), "esc", m.lang.t("form.back"))
			}
		}
		return sc("↑↓/tab", m.lang.t("sc.updown"), "space", m.lang.t("sc.space"), "enter", m.lang.t("form.run"), "esc", m.lang.t("form.back"))
	case screenFilePicker:
		return sc("↑↓", m.lang.t("sc.updown"), "space", m.lang.t("picker.select"), "c", m.lang.t("picker.confirm"), "esc", m.lang.t("picker.cancel"))
	case screenChatPicker:
		return sc("↑↓", m.lang.t("sc.updown"), "type", m.lang.t("chat.search"), "c", m.lang.t("chat.confirm"), "esc", m.lang.t("picker.cancel"))
	case screenErrorList:
		return sc("↑↓", m.lang.t("sc.updown"), "c", m.lang.t("errors.copy"), "r", m.lang.t("errors.retry.all"), "esc", m.lang.t("errors.back"))
	default:
		if !m.running {
			if m.progress.Status == xprogress.StatusCanceled && len(m.runResult.Items) > 0 {
				return sc("c", m.lang.t("run.continue"), "d", m.lang.t("run.details"), "enter", m.lang.t("form.back"))
			}
			if len(m.runResult.Items) > 0 {
				return sc("r", m.lang.t("run.retry"), "d", m.lang.t("run.details"), "enter", m.lang.t("form.back"))
			}
			return sc("d", m.lang.t("run.details"), "enter", m.lang.t("form.back"))
		}
		return sc("↑↓", m.lang.t("sc.scroll"), "d", m.lang.t("run.details"), "ctrl+c", m.lang.t("sc.ctrlc"), "ctrl+c ×2", m.lang.t("sc.quit"))
	}
}
