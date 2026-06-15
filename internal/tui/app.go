package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lookinalx/fiona/assets"
)

// ── palette (from FIONA logo) ─────────────────────────────────────────────────

var (
	colorDark   = lipgloss.Color("#2E4A22")
	colorMid    = lipgloss.Color("#5C7A3A")
	colorSage   = lipgloss.Color("#8A9E6A")
	colorCream  = lipgloss.Color("#D8DDB5")
	colorDimmed = lipgloss.Color("#4A5E38")
)

// ── ascii art ─────────────────────────────────────────────────────────────────

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

var fallbackTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(colorDark).
	Background(colorCream).
	Padding(0, 2)

// ── criteria ──────────────────────────────────────────────────────────────────

var primaryOptions = []string{"mimetype", "extension", "date", "year", "month", "size", "ml"}
var secondaryOptions = []string{"none", "mimetype", "extension", "date", "year", "month", "size", "ml"}

// ── fields ────────────────────────────────────────────────────────────────────

const (
	fieldPrimary = iota
	fieldSecondary
	fieldSource
	fieldDest
	fieldSort
	fieldQuit
	fieldCount
)

// ── styles ────────────────────────────────────────────────────────────────────

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
)

// ── model ─────────────────────────────────────────────────────────────────────

type model struct {
	focused    int
	primary    int
	secondary  int
	width      int
	height     int
	sourceTree FileTree
	destTree   FileTree
}

func newModel() model {
	return model{
		sourceTree: NewFileTree("Source"),
		destTree:   NewFileTree("Destination"),
	}
}

func (m model) Init() tea.Cmd { return tea.HideCursor }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateTreeWidths()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "tab":
			m.focused = (m.focused + 1) % fieldCount
			m.sourceTree.focused = m.focused == fieldSource
			m.destTree.focused = m.focused == fieldDest

		case "shift+tab":
			m.focused = (m.focused - 1 + fieldCount) % fieldCount
			m.sourceTree.focused = m.focused == fieldSource
			m.destTree.focused = m.focused == fieldDest

		default:
			switch m.focused {
			case fieldPrimary:
				switch msg.String() {
				case "left":
					m.primary = (m.primary - 1 + len(primaryOptions)) % len(primaryOptions)
				case "right":
					m.primary = (m.primary + 1) % len(primaryOptions)
				}
			case fieldSecondary:
				switch msg.String() {
				case "left":
					m.secondary = (m.secondary - 1 + len(secondaryOptions)) % len(secondaryOptions)
				case "right":
					m.secondary = (m.secondary + 1) % len(secondaryOptions)
				}
			case fieldSource:
				var cmd tea.Cmd
				m.sourceTree, cmd = m.sourceTree.Update(msg)
				return m, cmd
			case fieldDest:
				var cmd tea.Cmd
				m.destTree, cmd = m.destTree.Update(msg)
				return m, cmd
			case fieldSort:
				if msg.String() == "enter" {
					// TODO: запустить сортировку
				}
			case fieldQuit:
				if msg.String() == "enter" {
					return m, tea.Quit
				}
			}
		}

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			m = m.handleClick(msg.X, msg.Y)
			if m.focused == fieldQuit {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m model) handleClick(x, y int) model {
	// any click cancels an in-progress path edit; a header click below re-enables it
	m.sourceTree.editing = false
	m.destTree.editing = false

	// ── bottom bar ────────────────────────────────────────────────────────────
	if y == m.height-1 {
		sortW := lipgloss.Width(barItemInactive.Render("Sort"))
		quitStart := sortW + 1
		quitW := lipgloss.Width(barItemInactive.Render("Quit"))
		switch {
		case x < sortW:
			m.focused = fieldSort
		case x >= quitStart && x < quitStart+quitW:
			m.focused = fieldQuit
		}
		m.sourceTree.focused = false
		m.destTree.focused = false
		return m
	}

	// ── selectors ────────────────────────────────────────────────────────────
	primaryY := m.headerH()
	secondaryY := m.headerH() + 1
	if y == primaryY {
		m.focused = fieldPrimary
		m.sourceTree.focused = false
		m.destTree.focused = false
		return m
	}
	if y == secondaryY {
		m.focused = fieldSecondary
		m.sourceTree.focused = false
		m.destTree.focused = false
		return m
	}

	// ── trees ─────────────────────────────────────────────────────────────────
	srcY := m.sourceTreeY()
	treeBoxH := treeHeight + 3 // border top(1) + header(1) + rows(treeHeight) + border bottom(1)

	if m.isHorizontal() {
		if y >= srcY && y < srcY+treeBoxH {
			relRow := y - srcY - 2
			if x < m.sourceTree.width {
				m.focused = fieldSource
				m.sourceTree.focused = true
				m.destTree.focused = false
				if relRow >= 0 && relRow < treeHeight {
					m.sourceTree = m.sourceTree.HandleMouseClick(relRow)
				} else if relRow == -1 {
					m.sourceTree = m.sourceTree.StartEditing()
				}
			} else {
				m.focused = fieldDest
				m.sourceTree.focused = false
				m.destTree.focused = true
				if relRow >= 0 && relRow < treeHeight {
					m.destTree = m.destTree.HandleMouseClick(relRow)
				} else if relRow == -1 {
					m.destTree = m.destTree.StartEditing()
				}
			}
			return m
		}
	} else {
		dstY := m.destTreeY()
		if y >= srcY && y < srcY+treeBoxH {
			m.focused = fieldSource
			m.sourceTree.focused = true
			m.destTree.focused = false
			relRow := y - srcY - 2
			if relRow >= 0 && relRow < treeHeight {
				m.sourceTree = m.sourceTree.HandleMouseClick(relRow)
			} else if relRow == -1 {
				m.sourceTree = m.sourceTree.StartEditing()
			}
			return m
		}
		if y >= dstY && y < dstY+treeBoxH {
			m.focused = fieldDest
			m.sourceTree.focused = false
			m.destTree.focused = true
			relRow := y - dstY - 2
			if relRow >= 0 && relRow < treeHeight {
				m.destTree = m.destTree.HandleMouseClick(relRow)
			} else if relRow == -1 {
				m.destTree = m.destTree.StartEditing()
			}
			return m
		}
	}

	return m
}

func (m model) headerH() int {
	// show logo as soon as there's room for logo + selectors + one tree row + bar
	minForLogo := logoHeight + treeHeight + 9
	if m.height > 0 && m.height >= minForLogo {
		return logoHeight + 2 // "\n" + logo + "\n\n"
	}
	return 3 // "\n  FIONA\n\n"
}

// sourceTreeY returns the absolute terminal row where the source tree box starts.
func (m model) sourceTreeY() int {
	return m.headerH() + 3 // + primary(1) + secondary(1) + blank(1)
}

// destTreeY is only meaningful in vertical mode.
func (m model) destTreeY() int {
	return m.sourceTreeY() + treeHeight + 4 // box(treeHeight+3) + "\n" after View()
}

// isHorizontal returns true when both trees don't fit stacked vertically.
func (m model) isHorizontal() bool {
	if m.height == 0 {
		return false
	}
	return m.destTreeY()+treeHeight+3 >= m.height
}

func (m *model) updateTreeWidths() {
	if m.isHorizontal() {
		half := (m.width - 1) / 2
		m.sourceTree.width = half
		m.destTree.width = m.width - half - 1
	} else {
		m.sourceTree.width = m.width - 4
		m.destTree.width = m.width - 4
	}
}

func (m model) View() string {
	if m.width > 0 && m.width < logoWidth {
		return renderTooNarrow(m.width)
	}

	var header string
	if m.headerH() == logoHeight+2 {
		header = "\n" + renderLogo() + "\n\n"
	} else {
		header = "\n  " + fallbackTitleStyle.Render("FIONA") + "\n\n"
	}
	s := header
	s += "  " + renderSelector("Primary", primaryOptions[m.primary], m.focused == fieldPrimary) + "\n"
	s += "  " + renderSelector("Secondary", secondaryOptions[m.secondary], m.focused == fieldSecondary) + "\n\n"
	if m.isHorizontal() {
		s += lipgloss.JoinHorizontal(lipgloss.Top, m.sourceTree.View(), m.destTree.View()) + "\n"
	} else {
		s += m.sourceTree.View() + "\n"
		s += m.destTree.View() + "\n"
	}

	bar := renderBar(m.focused, m.width)

	contentLines := strings.Count(s, "\n")
	gap := m.height - contentLines - 1
	if gap > 0 {
		s += strings.Repeat("\n", gap)
	}

	return s + bar
}

// ── render helpers ────────────────────────────────────────────────────────────

// renderLogo paints the ascii logo in sage, with every 'M' in cream.
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

func renderTooNarrow(width int) string {
	line := strings.Repeat("─", width-2)
	msg := fmt.Sprintf("\n\n\n\n\n  Terminal is too narrow.\n  ┌%s┐\n  Resize to at least %d columns.\n  └%s┘",
		line, logoWidth, line)
	return tooNarrowStyle.Render(msg)
}

func renderSelector(label, value string, active bool) string {
	lbl := labelStyle.Render(label + ":")
	var val string
	if active {
		val = selectorBoxActive.Render("◀ " + padRight(value, 9) + " ▶")
	} else {
		val = selectorBoxInactive.Render("  " + padRight(value, 9) + "  ")
	}
	return lbl + " " + val
}

func renderBar(focused, width int) string {
	sortItem := barItemInactive.Render("Sort")
	quitItem := barItemInactive.Render("Quit")

	if focused == fieldSort {
		sortItem = barItemActive.Render("Sort")
	}
	if focused == fieldQuit {
		quitItem = barItemActive.Render("Quit")
	}

	items := sortItem + " " + quitItem
	hintText := "  tab navigate   ◀/▶ change   ↑/↓ tree   enter select  "
	hintW := lipgloss.Width(barHint.Render(hintText))
	gapW := width - lipgloss.Width(items) - hintW
	if gapW < 4 {
		gapW = 4
	}
	bar := items + strings.Repeat(" ", gapW) + barHint.Render(hintText)
	return barStyle.Width(width).Render(bar)
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// ── entry point ───────────────────────────────────────────────────────────────

func Run() error {
	_, err := tea.NewProgram(newModel(), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}
