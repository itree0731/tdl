package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ColorProfile is the terminal colour capability used by the TUI. Keeping
// the mapping here makes the warm graphite/copper design deterministic and
// gives low-colour and NO_COLOR terminals an intentional fallback.
type ColorProfile uint8

const (
	ColorTrue ColorProfile = iota
	ColorANSI256
	ColorANSI16
	ColorNone
)

// Palette contains semantic colours only. Views use the meaning of a colour
// (data, success, warning, etc.), not a one-off literal.
type Palette struct {
	Canvas, Sidebar, Surface, SurfaceRaised string
	Border, BorderMuted                     string
	Text, TextMuted                         string
	Copper, CopperBright, DataBlue          string
	Success, Warning, Error                 string
}

func paletteFor(profile ColorProfile) Palette {
	switch profile {
	case ColorANSI256:
		return Palette{
			Canvas: "233", Sidebar: "234", Surface: "235", SurfaceRaised: "236",
			Border: "239", BorderMuted: "237", Text: "223", TextMuted: "246",
			Copper: "173", CopperBright: "215", DataBlue: "117",
			Success: "114", Warning: "179", Error: "203",
		}
	case ColorANSI16:
		return Palette{
			Canvas: "0", Sidebar: "0", Surface: "0", SurfaceRaised: "8",
			Border: "8", BorderMuted: "8", Text: "15", TextMuted: "7",
			Copper: "3", CopperBright: "11", DataBlue: "14",
			Success: "10", Warning: "3", Error: "9",
		}
	case ColorNone:
		return Palette{}
	default:
		return Palette{
			Canvas: "#0B0F10", Sidebar: "#0E1314", Surface: "#111718", SurfaceRaised: "#182022",
			Border: "#4B514E", BorderMuted: "#303735", Text: "#F0E4D2", TextMuted: "#A89B8C",
			Copper: "#D39463", CopperBright: "#F0AC73", DataBlue: "#8EC8E8",
			Success: "#7DD889", Warning: "#E0B36E", Error: "#FF6B6B",
		}
	}
}

func detectColorProfile() ColorProfile {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return ColorNone
	}
	term := strings.ToLower(os.Getenv("TERM"))
	colorterm := strings.ToLower(os.Getenv("COLORTERM"))
	if strings.Contains(colorterm, "truecolor") || strings.Contains(colorterm, "24bit") {
		return ColorTrue
	}
	if strings.Contains(term, "256color") {
		return ColorANSI256
	}
	if term == "" || term == "dumb" {
		// Windows Terminal does not always set TERM, but supports true colour.
		if os.Getenv("WT_SESSION") != "" || os.Getenv("TERM_PROGRAM") != "" {
			return ColorTrue
		}
		return ColorANSI16
	}
	return ColorANSI16
}

func colour(v string) lipgloss.TerminalColor {
	if v == "" {
		return lipgloss.NoColor{}
	}
	return lipgloss.Color(v)
}

type uiTheme struct {
	palette Palette

	brand, title, hint, text, data, success, warning, failure lipgloss.Style
	item, itemSelected, itemDesc                              lipgloss.Style
	command, output, spinner, stop                            lipgloss.Style
	status, panel, panelRaised                                lipgloss.Style
	prompt, promptActive                                      lipgloss.Style
	shortcutKey, shortcutSep                                  lipgloss.Style
	fieldLabel, fieldFocus, fieldValue, fieldFlag             lipgloss.Style
}

func buildTheme(profile ColorProfile) uiTheme {
	p := paletteFor(profile)
	fg := func(v string) lipgloss.Style { return lipgloss.NewStyle().Foreground(colour(v)) }
	panel := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(colour(p.Border)).Padding(0, 1)
	return uiTheme{
		palette: p,
		brand:   fg(p.CopperBright).Bold(true),
		title:   fg(p.Text).Bold(true),
		hint:    fg(p.TextMuted),
		text:    fg(p.Text),
		data:    fg(p.DataBlue),
		success: fg(p.Success),
		warning: fg(p.Warning),
		failure: fg(p.Error),

		item:         fg(p.Text).Padding(0, 1),
		itemSelected: lipgloss.NewStyle().Background(colour(p.SurfaceRaised)).Foreground(colour(p.CopperBright)).Bold(true).Padding(0, 1),
		itemDesc:     fg(p.TextMuted).Padding(0, 1),

		command: fg(p.Copper),
		output:  fg(p.TextMuted),
		spinner: fg(p.CopperBright),
		stop:    fg(p.Error).Bold(true),

		status:      lipgloss.NewStyle().Background(colour(p.Surface)),
		panel:       panel,
		panelRaised: panel.BorderForeground(colour(p.Copper)),

		prompt:       panel.BorderForeground(colour(p.BorderMuted)),
		promptActive: panel.BorderForeground(colour(p.CopperBright)),

		shortcutKey: fg(p.DataBlue).Bold(true),
		shortcutSep: fg(p.Border),

		fieldLabel: fg(p.Text),
		fieldFocus: fg(p.CopperBright).Bold(true),
		fieldValue: fg(p.DataBlue),
		fieldFlag:  fg(p.TextMuted),
	}
}

var activeTheme = buildTheme(detectColorProfile())

// Compatibility aliases keep the rendering code small while all visual
// values still come from the one semantic theme above.
var (
	stBrand = activeTheme.brand
	stTitle = activeTheme.title
	stHint  = activeTheme.hint

	stItemSelected = activeTheme.itemSelected
	stItem         = activeTheme.item
	stItemDesc     = activeTheme.itemDesc

	stCmdEcho = activeTheme.command
	stOut     = activeTheme.output
	stErr     = activeTheme.failure
	stOK      = activeTheme.success
	stSys     = activeTheme.data

	stSpinner = activeTheme.spinner
	stStopKey = activeTheme.stop

	stStatusBg = activeTheme.status

	stPromptBorderActive = activeTheme.promptActive
	stPromptBorder       = activeTheme.prompt

	stShortcutKey = activeTheme.shortcutKey
	stShortcutSep = activeTheme.shortcutSep

	stFieldLabel = activeTheme.fieldLabel
	stFieldFocus = activeTheme.fieldFocus
	stFieldValue = activeTheme.fieldValue
	stFieldFlag  = activeTheme.fieldFlag

	stBanner = activeTheme.hint.Italic(true)
)

const brandName = "TDL"
