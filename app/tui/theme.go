package tui

import "github.com/charmbracelet/lipgloss"

// GrokNight palette, after the default theme of xai-org/grok-build
// (itself derived from TokyoNight Night).
var (
	cTerminal = lipgloss.Color("#0a0a0a") // terminal background
	cBg       = lipgloss.Color("#141414") // main background
	cFg       = lipgloss.Color("#e1e1e1") // main foreground
	cMagenta  = lipgloss.Color("#bb9af7") // running / assistant accent
	cBlue     = lipgloss.Color("#7aa2f7") // system info
	cGreen    = lipgloss.Color("#9ece6a") // success
	cRed      = lipgloss.Color("#f7768e") // error
	cOrange   = lipgloss.Color("#ff9e64") // paths / numbers
	cYellow   = lipgloss.Color("#e0af68") // commands / warnings
	cCyan     = lipgloss.Color("#7dcfff") // secondary accent
	cTeal     = lipgloss.Color("#1abc9c") // brand
	cDim      = lipgloss.Color("#565f89") // comments / disabled
)

var (
	stBrand = lipgloss.NewStyle().Foreground(cTeal).Bold(true)

	stTitle = lipgloss.NewStyle().Foreground(cFg).Bold(true)
	stHint  = lipgloss.NewStyle().Foreground(cDim)

	stItemSelected = lipgloss.NewStyle().
			Background(lipgloss.Color("#1f2233")).
			Foreground(cMagenta).Bold(true).Padding(0, 1)
	stItem     = lipgloss.NewStyle().Foreground(cFg).Padding(0, 1)
	stItemDesc = lipgloss.NewStyle().Foreground(cDim).Padding(0, 1)

	stCmdEcho = lipgloss.NewStyle().Foreground(cYellow)
	stOut     = lipgloss.NewStyle().Foreground(cFg).Faint(true)
	stErr     = lipgloss.NewStyle().Foreground(cRed)
	stOK      = lipgloss.NewStyle().Foreground(cGreen)
	stSys     = lipgloss.NewStyle().Foreground(cBlue)

	stSpinner = lipgloss.NewStyle().Foreground(cMagenta)
	stStopKey = lipgloss.NewStyle().Foreground(cRed).Bold(true)

	stStatusBg = lipgloss.NewStyle().Background(cBg)

	stPromptBorderActive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(cMagenta).
				Padding(0, 1)
	stPromptBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#33364a")).
			Padding(0, 1)

	stShortcutKey = lipgloss.NewStyle().Foreground(cCyan).Bold(true)
	stShortcutSep = lipgloss.NewStyle().Foreground(cDim)

	stFieldLabel = lipgloss.NewStyle().Foreground(cFg)
	stFieldFocus = lipgloss.NewStyle().Foreground(cMagenta).Bold(true)
	stFieldValue = lipgloss.NewStyle().Foreground(cOrange)
	stFieldFlag  = lipgloss.NewStyle().Foreground(cDim)

	stBanner = lipgloss.NewStyle().Foreground(cDim).Italic(true)
)

// asciiLogo is "TDL" in figlet ANSI-shadow, shown above the menu when the
// terminal is tall enough; logoStyles gives it a teal→cyan→blue→magenta
// gradient, echoing the accent ramp of grok-build's GrokNight theme.
var asciiLogo = []string{
	`████████╗ ██████╗ ██╗     `,
	`╚══██╔══╝██╔═══██╗██║     `,
	`   ██║   ██║   ██║██║     `,
	`   ██║   ██║   ██║██║     `,
	`   ██║   ╚██████╔╝███████╗`,
	`   ╚═╝    ╚═════╝ ╚══════╝`,
}

var logoStyles = []lipgloss.Style{
	lipgloss.NewStyle().Foreground(cTeal),
	lipgloss.NewStyle().Foreground(cTeal),
	lipgloss.NewStyle().Foreground(cCyan),
	lipgloss.NewStyle().Foreground(cBlue),
	lipgloss.NewStyle().Foreground(cMagenta),
	lipgloss.NewStyle().Foreground(cMagenta),
}
