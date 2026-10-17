package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lookinalx/fiona/assets"
)

// ── palette ───────────────────────────────────────────────────────────────────

var (
	colorDark   = lipgloss.Color("#2E4A22")
	colorMid    = lipgloss.Color("#5C7A3A")
	colorSage   = lipgloss.Color("#8A9E6A")
	colorCream  = lipgloss.Color("#D8DDB5")
	colorDimmed = lipgloss.Color("#4A5E38")
	colorRed    = lipgloss.Color("#C4643C")
	colorYellow = lipgloss.Color("#D9B44A")
)

// ── logo ──────────────────────────────────────────────────────────────────────

var asciiLogo = strings.TrimRight(assets.ASCIILogo, "\n")

var (
	logoWidth  = lipgloss.Width(asciiLogo)
	logoHeight = lipgloss.Height(asciiLogo)
)

// logoMinContent is how many rows must stay for the tree and the plan below the logo.
const logoMinContent = 20

func renderLogo() string {
	var b strings.Builder
	for _, r := range asciiLogo {
		switch r {
		case 'M':
			b.WriteString(logoMStyle.Render("M"))
		case '\n', ' ':
			b.WriteRune(r)
		default:
			b.WriteString(logoStyle.Render(string(r)))
		}
	}
	return b.String()
}

// ── styles ────────────────────────────────────────────────────────────────────

var (
	logoStyle  = lipgloss.NewStyle().Foreground(colorSage)
	logoMStyle = lipgloss.NewStyle().Foreground(colorCream)

	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorDark).Background(colorCream).Padding(0, 1)
	headerStyle = lipgloss.NewStyle().Foreground(colorSage)

	textStyle = lipgloss.NewStyle().Foreground(colorCream)
	dimStyle  = lipgloss.NewStyle().Foreground(colorSage)
	errStyle  = lipgloss.NewStyle().Foreground(colorRed)
	warnStyle = lipgloss.NewStyle().Foreground(colorYellow)
	okStyle   = lipgloss.NewStyle().Foreground(colorSage).Bold(true)

	cursorStyle    = lipgloss.NewStyle().Background(colorMid).Foreground(colorCream).Bold(true)
	cursorDimStyle = lipgloss.NewStyle().Background(colorDimmed).Foreground(colorCream)

	boxActive   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorMid).Padding(0, 1)
	boxInactive = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorDimmed).Padding(0, 1)
	boxTitle    = lipgloss.NewStyle().Bold(true).Foreground(colorCream)

	presetActive   = lipgloss.NewStyle().Background(colorMid).Foreground(colorCream).Bold(true).Padding(0, 1)
	presetInactive = lipgloss.NewStyle().Foreground(colorSage).Padding(0, 1)

	barStyle      = lipgloss.NewStyle().Background(colorDark).Foreground(colorSage)
	barKeyStyle   = lipgloss.NewStyle().Background(colorDark).Foreground(colorCream).Bold(true)
	questionStyle = lipgloss.NewStyle().Background(colorCream).Foreground(colorDark).Bold(true)
)
