package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type state int

const (
	stateMenu state = iota
	stateForm
	stateRun
)

const maxScrollbackLines = 5000

type model struct {
	program *tea.Program
	exec    Executor

	lang Lang
	set  settings

	actions []action
	menuIx  int

	// form state (also used for settings)
	form      *action
	formIx    int
	isSetting bool

	spinner    spinner.Model
	running    bool
	showOutput bool
	runErr     error
	runLabel   string
	runStart   time.Time
	runLast    time.Duration
	live       string
	cancelRun  func()

	vp     viewport.Model
	follow bool

	scrollback []string

	width, height int

	// mouse hit boxes, recomputed every View
	menuY   int // screen Y of first menu row
	stopBox [4]int
}

func newModel(exec Executor) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = stSpinner

	set := loadSettings()

	m := model{
		exec:    exec,
		lang:    Lang(set.Language),
		set:     set,
		actions: newActions(),
		spinner: sp,
		vp:      viewport.New(0, 0),
		follow:  true,
	}
	return m
}

// Run starts the TUI program. exec is injected from package cmd.
func Run(exec Executor) error {
	m := newModel(exec)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.program = p
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd { return m.spinner.Tick }

// ---------------------------------------------------------------------------
// settings form

func (m *model) openSettings() {
	langIx := 0
	if m.lang == LangZh {
		langIx = 1
	}
	m.form = &action{
		id:       "settings",
		titleKey: "menu.settings",
		fields: []field{
			{kind: kChoice, labelKey: "set.language", choices: []string{"en", "zh"}, choiceIx: langIx},
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

func (m *model) saveSettingsFromForm() {
	f := m.form.fields
	m.lang = Lang(f[0].choices[f[0].choiceIx])
	m.set.Language = string(m.lang)
	m.set.NS = f[1].value()
	m.set.Proxy = f[2].value()
	m.set.Threads = f[3].value()
	m.set.Limit = f[4].value()
	_ = saveSettings(m.set)
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
		if m.form != nil {
			for i := range m.form.fields {
				m.form.fields[i].ti.Width = w - 24
				if m.form.fields[i].ti.Width < 10 {
					m.form.fields[i].ti.Width = 10
				}
			}
		}
		return m, nil

	case spinner.TickMsg:
		if !m.running {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case outputLineMsg:
		m.appendLine(msg.text)
		return m, nil

	case liveLineMsg:
		m.live = msg.text
		return m, nil

	case runDoneMsg:
		m.running = false
		m.runErr = msg.err
		m.runLast = msg.elapsed
		m.live = ""
		m.appendLine("")
		if msg.err != nil {
			m.appendLine(stErr.Render(m.lang.t("status.failed") + " " + msg.elapsed.Truncate(time.Millisecond).String()))
			m.appendLine(stErr.Render(msg.err.Error()))
		} else {
			m.appendLine(stOK.Render(m.lang.t("status.done") + " " + msg.elapsed.Truncate(time.Millisecond).String()))
		}
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
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
		if m.state() == stateRun && m.running {
			x, y := int(msg.X), int(msg.Y)
			if x >= m.stopBox[0] && x <= m.stopBox[2] && y >= m.stopBox[1] && y <= m.stopBox[3] {
				return m.stopRun()
			}
		}
		if m.state() == stateMenu {
			ix := int(msg.Y) - m.menuY
			if ix >= 0 && ix < len(m.actions) {
				if ix == m.menuIx {
					return m.openMenuItem(ix)
				}
				m.menuIx = ix
			}
		}
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ctrl+C: stop a running command, otherwise quit
	if msg.Type == tea.KeyCtrlC {
		if m.running && m.state() == stateRun {
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
		f := &m.form.fields[m.formIx]
		switch msg.String() {
		case "up":
			m.focusFormField(m.formIx - 1)
		case "down", "tab":
			m.focusFormField((m.formIx + 1) % len(m.form.fields))
		case "shift+tab":
			m.focusFormField(m.formIx - 1)
		case " ":
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
			if f.kind == kBool {
				f.boolVal = !f.boolVal
				return m, nil
			}
			if f.kind == kChoice {
				f.cycle(true)
				return m, nil
			}
			return m.launchForm()
		case "esc":
			m.form = nil
			m.isSetting = false
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
		case "pgup":
			m.follow = false
			m.vp.LineUp(m.vp.Height / 2)
		case "pgdown":
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
	a := &m.actions[ix]
	switch a.id {
	case "settings":
		m.openSettings()
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
		m.saveSettingsFromForm()
		m.form = nil
		m.isSetting = false
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
	m.runStart = time.Now()
	m.live = ""

	m.cancelRun = startRun(m.program, m.exec, argv)
	return m, m.spinner.Tick
}

func (m model) stopRun() (tea.Model, tea.Cmd) {
	if m.cancelRun != nil {
		m.cancelRun()
	}
	return m, nil
}

func (m *model) toMenu() {
	m.form = nil
	m.isSetting = false
	m.showOutput = false
	m.live = ""
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
		return "loading..."
	}

	var b []string

	// header
	b = append(b, stBrand.Render("tdl")+"  "+stBanner.Render(m.lang.t("banner.title")))

	// main area
	switch m.state() {
	case stateMenu:
		b = append(b, m.viewMenu())
	case stateForm:
		b = append(b, m.viewForm())
	default:
		b = append(b, m.vp.View())
	}

	// status line (visible while running, grok-style)
	b = append(b, m.viewStatus())

	// prompt box
	b = append(b, m.viewPrompt())

	// shortcuts bar
	b = append(b, m.viewShortcuts())

	return lipgloss.JoinVertical(lipgloss.Left, b...)
}

func (m model) viewMenu() string {
	rows := make([]string, 0, len(m.actions))
	m.menuY = 2 // header 1 + blank? rendered position of first item
	for i, a := range m.actions {
		title := a.title(m.lang)
		desc := a.desc(m.lang)
		var row string
		if i == m.menuIx {
			row = stItemSelected.Render("▶ "+title) + stItemDesc.Render(desc)
		} else {
			row = stItem.Render("  "+title) + stItemDesc.Render(desc)
		}
		rows = append(rows, row)
	}
	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	if m.lang.t("menu.title") != "" {
		body = lipgloss.JoinVertical(lipgloss.Left,
			stTitle.Render(m.lang.t("menu.title")), "", body)
		m.menuY = 3
	}
	return lipgloss.NewStyle().Height(m.mainHeight()).Render(body)
}

func (m model) viewForm() string {
	rows := []string{stTitle.Render(m.form.title(m.lang)), ""}
	for i := range m.form.fields {
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
					chs = append(chs, stFieldFocus.Render(c))
				} else {
					chs = append(chs, stHint.Render(c))
				}
			}
			value = strings.Join(chs, stShortcutSep.Render("/")) + "  " + stFieldFlag.Render(f.flag)
		default:
			value = stFieldValue.Render(f.ti.View()) + "  " + stFieldFlag.Render(f.flag)
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

func (m model) viewStatus() string {
	if !m.running {
		return ""
	}
	elapsed := time.Since(m.runStart).Truncate(time.Millisecond)
	live := m.live
	// keep the line inside the width
	budget := m.width - lipgloss.Width(m.runLabel) - 20
	if budget > 4 && lipgloss.Width(live) > budget {
		r := []rune(live)
		live = string(r[:min(budget-1, len(r))]) + "…"
	}
	stop := stStopKey.Render("[stop]")
	stopPlain := "[stop]"
	pad := m.width - lipgloss.Width(m.spinner.View()) - lipgloss.Width(m.runLabel) - len(elapsed.String()) - lipgloss.Width(live) - len(stopPlain) - 4
	if pad < 1 {
		pad = 1
	}
	line := m.spinner.View() + " " + stFieldFocus.Render(m.runLabel) + " " +
		stHint.Render(elapsed.String()) + " " + live + strings.Repeat(" ", pad) + stop
	m.stopBox = [4]int{m.width - len(stopPlain) - 1, m.height - 5, m.width - 1, m.height - 5}
	return stStatusBg.Render(line)
}

func (m model) viewPrompt() string {
	var text string
	switch m.state() {
	case stateForm:
		argv := m.form.argv(m.set.globalArgs())
		if m.isSetting {
			text = stCmdEcho.Render("tdl " + m.lang.t("set.global"))
		} else {
			text = stCmdEcho.Render("$ tdl " + quoteJoin(argv))
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
	return box.Width(m.width - 2).MaxHeight(3).Render(text)
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
		return sc("↑↓/tab", m.lang.t("sc.updown"), "space", m.lang.t("sc.space"), "enter", m.lang.t("form.run"), "esc", m.lang.t("form.back"))
	default:
		return sc("↑↓", m.lang.t("sc.scroll"), "enter", m.lang.t("form.back"), "ctrl+c", m.lang.t("sc.quit"))
	}
}
