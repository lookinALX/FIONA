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
)

// ── logo ──────────────────────────────────────────────────────────────────────

var asciiLogo = assets.ASCIILogo

var (
	logoWidth  int
	logoHeight int
)

func init() {
	lines := strings.Split(asciiLogo, "\n")
	logoHeight = len(lines)
	for _, line := range lines {
		if len(line) > logoWidth {
			logoWidth = len(line)
		}
	}
}

// ── styles ────────────────────────────────────────────────────────────────────

var fallbackTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(colorDark).
	Background(colorCream).
	Padding(0, 2)

var (
	logoStyle  = lipgloss.NewStyle().Foreground(colorSage)
	logoMStyle = lipgloss.NewStyle().Foreground(colorCream)

	tooNarrowStyle = lipgloss.NewStyle().Foreground(colorSage)

	labelStyle = lipgloss.NewStyle().
			Width(14).
			Foreground(colorCream)

	selectorBoxActive = lipgloss.NewStyle().
				Foreground(colorCream).
				Bold(true).
				Padding(0, 1)

	selectorBoxInactive = lipgloss.NewStyle().
				Foreground(colorSage).
				Padding(0, 1)

	barStyle = lipgloss.NewStyle().
			Background(colorDark).
			Foreground(colorSage)

	barItemActive = lipgloss.NewStyle().
			Background(colorMid).
			Foreground(colorCream).
			Bold(true).
			Padding(0, 2)

	barItemInactive = lipgloss.NewStyle().
			Background(colorDark).
			Foreground(colorSage).
			Padding(0, 2)

	barHint = lipgloss.NewStyle().
		Background(colorCream).
		Foreground(colorDark)

	dividerStyle = lipgloss.NewStyle().Foreground(colorDimmed)
)
