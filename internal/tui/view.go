package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if m.width > 0 && m.width < logoWidth {
		return renderTooNarrow(m.width)
	}

	var header string
	if m.headerH() == logoHeight+2 {
		header = "\n" + centerIn(renderLogo(), m.width) + "\n\n"
	} else {
		header = "\n" + centerIn(fallbackTitleStyle.Render("FIONA"), m.width) + "\n\n"
	}

	s := header
	fa := "move"
	if m.copyMode {
		fa = "copy"
	}
	s += centerIn(renderSelector("Primary", primaryOptions[m.primary], m.focused == fieldPrimary), m.width) + "\n"
	s += centerIn(renderSelector("Secondary", secondaryOptions[m.secondary], m.focused == fieldSecondary), m.width) + "\n"
	s += centerIn(renderSelector("File action", fa, m.focused == fieldAction), m.width) + "\n\n"

	if m.isHorizontal() {
		div := renderDivider(treeHeight + 3)
		trees := lipgloss.JoinHorizontal(lipgloss.Top, m.sourceTree.View(), div, m.destTree.View())
		s += centerIn(trees, m.width) + "\n"
	} else {
		s += centerIn(m.sourceTree.View(), m.width) + "\n"
		s += centerIn(m.destTree.View(), m.width) + "\n"
	}

	bar := renderBar(m.focused, m.width)

	contentLines := strings.Count(s, "\n")
	if gap := m.height - contentLines - 1; gap > 0 {
		s += strings.Repeat("\n", gap)
	}

	return s + bar
}

// ── render helpers ────────────────────────────────────────────────────────────

func centerIn(s string, width int) string {
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, s)
}

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
	undoItem := barItemInactive.Render("Undo")
	quitItem := barItemInactive.Render("Quit")

	if focused == fieldSort {
		sortItem = barItemActive.Render("Sort")
	}
	if focused == fieldUndo {
		undoItem = barItemActive.Render("Undo")
	}
	if focused == fieldQuit {
		quitItem = barItemActive.Render("Quit")
	}

	items := sortItem + " " + undoItem + " " + quitItem
	hintText := contextHint(focused)
	hintW := lipgloss.Width(barHint.Render(hintText))
	gapW := width - lipgloss.Width(items) - hintW
	if gapW < 4 {
		gapW = 4
	}
	bar := items + strings.Repeat(" ", gapW) + barHint.Render(hintText)
	return barStyle.Width(width).Render(bar)
}

func contextHint(focused int) string {
	switch focused {
	case fieldPrimary, fieldSecondary, fieldAction:
		return "  ◀/▶ change   tab next  "
	case fieldSource, fieldDest:
		return "  ↑/↓ navigate   →/← expand   enter select   e edit path  "
	case fieldSort, fieldUndo, fieldQuit:
		return "  enter confirm   tab navigate  "
	default:
		return "  tab navigate  "
	}
}

func renderDivider(height int) string {
	lines := make([]string, height)
	for i := range lines {
		lines[i] = dividerStyle.Render("│")
	}
	return strings.Join(lines, "\n")
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
