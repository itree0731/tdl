package tui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	xprogress "github.com/iyear/tdl/pkg/progress"
)

type state int

const (
	stateMenu state = iota
	stateForm
	stateRun
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
	settingsButton    int
	running           bool
	showOutput        bool
	runErr            error
	runLabel          string
	runCommand        string
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

	scrollback []string

	width, height int
}

func newModel(exec Executor, namespaces []string) model {
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
		ctx:        context.Background(),
		exec:       exec,
		lang:       Lang(set.Language),
		set:        set,
		namespaces: namespaces,
		hasSession: len(namespaces) > 0,
		actions:    newActions(),
		vp:         viewport.New(0, 0),
		follow:     true,
	}
	return m
}

// Run starts the TUI program. exec is injected from package cmd;
// namespaces are the logged-in namespaces shown in the header.
func Run(ctx context.Context, exec Executor, namespaces []string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	m := newModel(exec, namespaces)
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
	candidate := settings{Language: f[0].choices[f[0].choiceIx], NS: f[1].value(), Proxy: f[2].value(), Threads: f[3].value(), Limit: f[4].value()}
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
		if msg.runID == m.runID && m.running {
			m.progress = msg.snapshot
			m.renderViewport()
		}
		return m, nil
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
		m.appendLine("")
		m.appendLine(m.resultLabel() + " " + msg.elapsed.Truncate(time.Millisecond).String())
		if msg.err != nil {
			m.appendLine(msg.err.Error())
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
		// account chips on the header row: click to switch namespace
		if !m.running && !m.isSetting && !m.settingsPrompt && int(msg.Y) == 0 {
			for _, c := range m.nsChipLayout(m.width) {
				if x := int(msg.X); x >= c.x0 && x < c.x1 {
					return m.switchNS(c.ns)
				}
			}
		}
		switch m.state() {
		case stateRun:
			for _, button := range m.runButtons() {
				if button.contains(int(msg.X), int(msg.Y)) {
					return m.activateButton(button.id)
				}
			}
		case stateForm:
			if m.isSetting && !m.settingsPrompt {
				for _, button := range m.settingsButtons() {
					if button.contains(int(msg.X), int(msg.Y)) {
						return m.activateButton(button.id)
					}
				}
			}

		case stateMenu:
			firstY, firstIx, count := m.menuWindow()
			ix := int(msg.Y) - firstY + firstIx
			if ix >= firstIx && ix < firstIx+count {
				if ix == m.menuIx {
					return m.openMenuItem(ix)
				}
				m.menuIx = ix
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
			if m.menuIx > 0 {
				m.menuIx--
			}
		case "down", "j":
			if m.menuIx < len(m.actions)-1 {
				m.menuIx++
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
			} else if f.kind == kText || f.kind == kExtra {
				var cmd tea.Cmd
				m.form.fields[m.formIx].ti, cmd = f.ti.Update(msg)
				return m, cmd
			}
		case "enter":
			return m.launchForm()
		case "esc":
			if m.isSetting && m.settingsDirty {
				m.settingsPrompt = true
				return m, nil
			}
			m.toMenu()
		default:
			if f.kind == kText || f.kind == kExtra {
				var cmd tea.Cmd
				m.form.fields[m.formIx].ti, cmd = f.ti.Update(msg)
				return m, cmd
			}
		}

	case stateRun:
		if !m.running {
			switch msg.String() {
			case "enter", "esc":
				m.toMenu()
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
		if k := m.form.fields[m.formIx].kind; k == kText || k == kExtra {
			m.form.fields[m.formIx].ti.Blur()
		}
	}
	m.formIx = ix
	if k := m.form.fields[ix].kind; k == kText || k == kExtra {
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

func (m model) launchAction(a *action) (tea.Model, tea.Cmd) {
	argv := a.argv(m.set.globalArgs())

	m.vp = viewport.New(m.width, m.mainHeight())
	m.vp.SetContent("")
	m.follow = true
	m.appendLine(stCmdEcho.Render("$ tdl " + quoteJoin(argv)))

	m.running = true
	m.showOutput = true
	m.runErr = nil
	m.runLabel = a.title(m.lang)
	m.runCommand = "$ tdl " + quoteJoin(argv)
	m.runStart = time.Now()
	m.live = ""
	m.stopReq = false

	m.runID++
	m.progressCollector = xprogress.NewCollector()
	m.progress = m.progressCollector.Snapshot()
	m.detailsOpen = false
	m.stopReq = false
	m.cancelRun = startRun(m.ctx, m.program, m.exec, argv, m.runID)
	return m, nil
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
	m.runID++ // invalidate late messages from the completed/aborted run
	m.form = nil
	m.isSetting = false
	m.showOutput = false
	m.live = ""
	m.scrollback = nil
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
	m.settingsButton = -1
	m.runErr = nil
	m.runLast = 0
	m.cancelRun = nil
	m.vp.SetContent("")
}

func (m model) state() state {
	switch {
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
	if len(m.scrollback) > maxScrollbackLines {
		m.scrollback = m.scrollback[len(m.scrollback)-maxScrollbackLines:]
	}
	m.renderViewport()
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
	h := m.height - 6 // header 1 + status 1 + prompt 3 + shortcuts 1
	if h < 3 {
		h = 3
	}
	return h
}

func (m model) View() string {
	if m.width == 0 {
		return m.lang.t("status.loading")
	}

	var b []string

	// header: one clickable chip per logged-in account
	prefix := stBrand.Render("tdl") + "  " + stBanner.Render(m.lang.t("banner.title")) + "  "
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

	// main area
	switch m.state() {
	case stateMenu:
		b = append(b, m.viewMenu())
	case stateForm:
		b = append(b, m.viewForm())
	default:
		b = append(b, m.viewRun())
	}

	// status line (visible while running, grok-style)
	b = append(b, m.viewStatus())

	// prompt box
	b = append(b, m.viewPrompt())

	// shortcuts bar
	b = append(b, m.viewShortcuts())

	for i := range b {
		b[i] = fitScreen(b[i], m.width, 0)
	}
	return fitScreen(lipgloss.JoinVertical(lipgloss.Left, b...), m.width, m.height)
}

// menuShowsLogo reports whether the ASCII logo fits above the menu.
func (m model) menuShowsLogo() bool {
	return m.mainHeight() >= len(asciiLogo)+2+len(m.actions)+1
}

// menuWindow computes where the menu items live on screen: the Y of the
// first visible row, the index of the first visible item, and how many
// items are visible. It is a pure function of the model, so View and
// mouse hit-testing always agree.
func (m model) menuWindow() (firstY, firstIx, count int) {
	firstY = 1 // header
	if m.menuShowsLogo() {
		firstY += len(asciiLogo) + 1 // logo + blank
	}
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
	x := lipgloss.Width(stBrand.Render("tdl")) + 2 +
		lipgloss.Width(stBanner.Render(m.lang.t("banner.title"))) + 2
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
	var body []string
	if m.menuShowsLogo() {
		for i, l := range asciiLogo {
			body = append(body, logoStyles[i%len(logoStyles)].Render(l))
		}
		body = append(body, "")
	}
	body = append(body, stTitle.Render(m.lang.t("menu.title")), "")

	_, firstIx, count := m.menuWindow()
	for i := firstIx; i < firstIx+count; i++ {
		a := m.actions[i]
		desc := a.desc(m.lang)
		if a.id == "login" && m.hasSession {
			desc += "  " + stOK.Render("✓ "+strings.Join(m.namespaces, ", "))
		}
		var row string
		if i == m.menuIx {
			row = stItemSelected.Render("▶ "+a.title(m.lang)) + stItemDesc.Render(desc)
		} else {
			row = stItem.Render("  "+a.title(m.lang)) + stItemDesc.Render(desc)
		}
		body = append(body, row)
	}
	content := lipgloss.JoinVertical(lipgloss.Left, body...)
	return lipgloss.NewStyle().Height(m.mainHeight()).Render(content)
}

func (m model) viewForm() string {
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
		default:
			input := f.ti
			input.Placeholder = localizedPlaceholder(m.lang, f.placeholder)
			input.Width = max(1, m.width-lipgloss.Width(f.label(m.lang))-lipgloss.Width(f.flag)-10)
			value = stFieldValue.Render(input.View()) + "  " + stFieldFlag.Render(f.flag)
		}
		if i == m.formIx {
			cursor = stFieldFocus.Render("▸ ")
			label = stFieldFocus.Render(f.label(m.lang))
		}
		rows = append(rows, cursor+label+": "+value)
	}
	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return lipgloss.NewStyle().Height(m.mainHeight()).Render(body)
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
			text = stHint.Render(m.currentFieldHelp())
		}
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
		return sc("↑↓/tab", m.lang.t("sc.updown"), "space", m.lang.t("sc.space"), "enter", m.lang.t("form.run"), "esc", m.lang.t("form.back"))
	default:
		return sc("↑↓", m.lang.t("sc.scroll"), "enter", m.lang.t("form.back"),
			"d", m.lang.t("run.details"), "ctrl+c", m.lang.t("sc.ctrlc"), "ctrl+c ×2", m.lang.t("sc.quit"))
	}
}
