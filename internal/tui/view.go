package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/lookinalx/fiona/internal/history"
	"github.com/lookinalx/fiona/internal/report"
	"github.com/lookinalx/fiona/internal/types"
)

const (
	minWidth  = 60
	minHeight = 14
)

// layout is where the parts of the screen are; mouse clicks use it too.
type layout struct {
	logo     bool
	headerH  int // rows above the boxes
	boxTop   int
	boxH     int // outer height of both boxes
	treeW    int // outer width of the tree box
	planW    int
	presetsY int
	barY     int
}

func (m model) layout() layout {
	l := layout{headerH: 1}
	if m.height >= logoHeight+1+logoMinContent+2 && m.width >= logoWidth {
		l.logo = true
		l.headerH = logoHeight + 1
	}
	l.boxTop = l.headerH
	l.boxH = max(m.height-l.headerH-2, 4)
	l.treeW = min(max(m.width/3, 28), 48)
	l.planW = max(m.width-l.treeW, 20)
	l.presetsY = l.boxTop + l.boxH
	l.barY = l.presetsY + 1
	return l
}

func (l layout) innerW(outer int) int { return max(outer-4, 10) } // border and padding
func (l layout) innerH() int          { return max(l.boxH-2, 2) }

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.width < minWidth || m.height < minHeight {
		return dimStyle.Render(fmt.Sprintf("\n  The terminal is too small: %d×%d.\n  FIONA needs at least %d×%d.", m.width, m.height, minWidth, minHeight))
	}

	l := m.layout()
	var b strings.Builder

	if l.logo {
		b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, renderLogo()))
		b.WriteString("\n")
	}
	b.WriteString(m.headerLine())
	b.WriteString("\n")

	treeTitle := "Folder"
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		box(treeTitle, m.tree.Lines(m.focus == focusTree && m.screen == screenMain), l.treeW, l.boxH, m.focus == focusTree && m.screen == screenMain),
		box(m.planTitle(), m.planLines(l), l.planW, l.boxH, m.focus != focusTree || m.screen != screenMain),
	))
	b.WriteString("\n")
	b.WriteString(m.presetsLine())
	b.WriteString("\n")
	b.WriteString(m.barLine())
	return b.String()
}

// box draws a bordered panel with a title line, cutting what does not fit.
func box(title string, lines []string, width, height int, active bool) string {
	innerW, innerH := max(width-4, 10), max(height-2, 2)
	content := []string{boxTitle.Render(fitWidth(title, innerW))}
	for _, line := range lines {
		if len(content) == innerH {
			break
		}
		if lipgloss.Width(line) > innerW {
			line = fitWidth(line, innerW)
		}
		content = append(content, line)
	}
	for len(content) < innerH {
		content = append(content, "")
	}

	style := boxInactive
	if active {
		style = boxActive
	}
	return style.Width(width - 2).Height(innerH).Render(strings.Join(content, "\n"))
}

func (m model) headerLine() string {
	action := "move"
	if m.copy {
		action = "copy"
	}
	settings := []string{action}
	if m.dest != "" {
		settings = append(settings, "into "+report.Path(m.dest))
	} else {
		settings = append(settings, "into the same folder")
	}
	if m.recurse {
		settings = append(settings, "subfolders")
	}
	if m.hidden {
		settings = append(settings, "hidden files")
	}
	right := fitWidth(strings.Join(settings, " · "), m.width/2)

	title := titleStyle.Render("FIONA") + " "
	// the path gives way to the settings when the line is short
	pathW := m.width - lipgloss.Width(title) - lipgloss.Width(right) - 2
	left := title + textStyle.Render(fitWidth(report.Path(m.source()), max(pathW, 5)))

	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right)-1, 1)
	return left + strings.Repeat(" ", gap) + headerStyle.Render(right)
}

func (m model) planTitle() string {
	switch {
	case m.help:
		return "Keys"
	case m.screen == screenRunning:
		return "Running"
	case m.screen == screenDone:
		return "Done"
	case m.screen == screenHistory:
		return "History"
	case m.screen == screenUndoConfirm:
		return "Undo"
	case m.focus == focusDest:
		return "Destination: choose a folder"
	default:
		return "Result · " + m.ruleSource
	}
}

func (m model) planLines(l layout) []string {
	switch {
	case m.help:
		return helpLines()
	case m.screen == screenRunning:
		return m.runningLines(l)
	case m.screen == screenDone:
		return styleLines(m.outcome)
	case m.screen == screenHistory:
		return m.historyLines(l)
	case m.screen == screenUndoConfirm:
		return append(styleLines(m.undoLines), "", dimStyle.Render(m.undoQuestion))
	case m.focus == focusDest:
		return m.destTree.Lines(true)
	}

	lines := m.resultLines()
	visible := l.innerH() - 1
	if m.planOffset > 0 && len(lines) > visible {
		offset := min(m.planOffset, len(lines)-visible)
		lines = lines[offset:]
	}
	return lines
}

// resultLines is the plan of the selected folder, as the CLI shows it.
func (m model) resultLines() []string {
	switch {
	case m.rulesErr != nil:
		return []string{errStyle.Render(m.rulesErr.Error())}
	case m.scanErr != nil:
		return []string{errStyle.Render(m.scanErr.Error())}
	case m.scanning || m.scanned == nil:
		return []string{dimStyle.Render("Scanning " + report.Path(m.source()) + "…")}
	case m.plan == nil:
		return []string{dimStyle.Render("Planning…")}
	}

	view := report.PlanView{Plan: m.plan, Recursive: m.recurse}
	if report.Empty(m.plan) {
		return []string{dimStyle.Render(report.NothingToSort(view))}
	}

	lines := styleLines(report.Overview(view, report.Styles{}))
	switch {
	case m.classifying:
		lines = append(lines, "", warnStyle.Render(fmt.Sprintf("Classifying %s…", report.Count(m.mlPending, "file"))))
	case m.classifyErr != nil:
		lines = append(lines, "", errStyle.Render("ML: "+m.classifyErr.Error()))
	case m.mlPending > 0:
		lines = append(lines, "", warnStyle.Render(fmt.Sprintf("ML classification: %s wait — press m", report.Count(m.mlPending, "file"))))
	}
	if m.plan.Stats().Files == 0 {
		lines = append(lines, "", dimStyle.Render("Nothing to do."))
	}
	if m.showList {
		lines = append(lines, "")
		lines = append(lines, styleLines(report.FileList(m.plan, report.Styles{}))...)
	}
	return lines
}

func styleLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "✓"):
			out[i] = okStyle.Render(line)
		case strings.HasPrefix(line, "!"), strings.HasPrefix(line, "Stopped"), strings.Contains(line, "⊘"):
			out[i] = warnStyle.Render(line)
		case strings.HasPrefix(line, "✗"), strings.Contains(line, " ✗ "):
			out[i] = errStyle.Render(line)
		default:
			out[i] = textStyle.Render(line)
		}
	}
	return out
}

func (m model) runningLines(l layout) []string {
	done, total, current := m.progress.snapshot()
	verb := "Moving"
	switch {
	case m.progress.undo:
		verb = "Undoing"
	case m.copy:
		verb = "Copying"
	}

	barW := max(l.innerW(l.planW)-14, 10)
	filled := 0
	if total > 0 {
		filled = barW * done / total
	}
	bar := okStyle.Render(strings.Repeat("█", filled)) + dimStyle.Render(strings.Repeat("░", barW-filled))

	lines := []string{
		textStyle.Render(fmt.Sprintf("%s files…", verb)),
		"",
		bar + textStyle.Render(fmt.Sprintf(" %d/%d", done, total)),
		dimStyle.Render(current),
		"",
		dimStyle.Render(report.Duration(time.Since(m.runStart))),
	}
	return lines
}

func (m model) historyLines(l layout) []string {
	if m.histErr != nil {
		return []string{errStyle.Render(m.histErr.Error())}
	}
	if m.records == nil {
		return []string{dimStyle.Render("Reading the history…")}
	}
	if len(m.records) == 0 {
		return []string{dimStyle.Render("No runs yet.")}
	}

	now := time.Now()
	width := l.innerW(l.planW)
	var lines []string
	for i, r := range m.records {
		where := report.Path(r.Dest)
		if r.Source != r.Dest {
			where = report.Path(r.Source) + " → " + report.Path(r.Dest)
		}
		line := fitWidth(fmt.Sprintf("%-16s %s %s · %s · %s", report.When(r.StartedAt, now), r.Action, report.Count(r.Planned, "file"), r.State, where), width)
		switch {
		case i == m.histCursor:
			line = cursorStyle.Width(width).Render(line)
		case r.State == history.StateUndone:
			line = dimStyle.Render(line)
		default:
			line = textStyle.Render(line)
		}
		lines = append(lines, line)
	}

	visible := l.innerH() - 1
	if m.histCursor >= visible {
		lines = lines[m.histCursor-visible+1:]
	}
	return lines
}

func helpLines() []string {
	keys := [][2]string{
		{"↑ ↓ → ←", "move through the folders, open and close them"},
		{"e", "type a folder path"},
		{"tab", "switch between the folders and the plan"},
		{"l", "in the plan: show every file"},
		{"p, shift+p", "next or previous preset"},
		{"t", "edit the template of the custom preset"},
		{"c", "copy instead of move"},
		{"d", "sort into another folder; esc there returns to the same one"},
		{"r", "include subfolders"},
		{".", "include hidden files"},
		{"m", "run the ML classifier when the preset uses {ml}"},
		{"enter", "sort, after one question"},
		{"h", "history of runs, undo any of them"},
		{"q", "quit"},
	}
	lines := make([]string, 0, len(keys)+2)
	for _, k := range keys {
		lines = append(lines, barKeyStyle.UnsetBackground().Render(fmt.Sprintf("%-11s", k[0]))+" "+textStyle.Render(k[1]))
	}
	return append(lines, "", dimStyle.Render("Any key closes this help."))
}

// ── presets line ──────────────────────────────────────────────────────────────

const presetsLabel = " Preset "

func (m model) presetsLine() string {
	var b strings.Builder
	b.WriteString(dimStyle.Render(presetsLabel))
	for i, name := range m.presets {
		if i == m.preset {
			b.WriteString(presetActive.Render(name))
		} else {
			b.WriteString(presetInactive.Render(name))
		}
	}
	if m.presets[m.preset] == customPreset {
		b.WriteString("  ")
		b.WriteString(m.template.View("To: "))
	}
	return fitWidthStyled(b.String(), m.width)
}

// presetAt finds the preset under column x; the items are as wide as presetsLine draws them.
func presetAt(presets []string, active, x int) (int, bool) {
	pos := lipgloss.Width(presetsLabel)
	for i, name := range presets {
		style := presetInactive
		if i == active {
			style = presetActive
		}
		w := lipgloss.Width(style.Render(name))
		if x >= pos && x < pos+w {
			return i, true
		}
		pos += w
	}
	return 0, false
}

// ── hint bar ──────────────────────────────────────────────────────────────────

type hint struct {
	key, label string
}

// hints are the keys that make sense right now, most important first.
func (m model) hints() []hint {
	switch {
	case m.help:
		return []hint{{"esc", "close"}}
	case m.screen == screenConfirm:
		return []hint{{"y", "yes"}, {"n", "no"}}
	case m.screen == screenUndoConfirm:
		return []hint{{"y", "yes"}, {"n", "no"}}
	case m.screen == screenRunning:
		return []hint{{"ctrl+c", "stop after the current files"}}
	case m.screen == screenDone:
		hs := []hint{}
		if m.journal != "" {
			hs = append(hs, hint{"u", "undo"})
		}
		return append(hs, hint{"o", "open folder"}, hint{"n", "new sort"}, hint{"h", "history"}, hint{"q", "quit"})
	case m.screen == screenHistory:
		return []hint{{"enter", "undo this run"}, {"esc", "back"}}
	case m.focus == focusTemplate:
		return []hint{{"enter", "done"}}
	case m.focus == focusDest:
		return []hint{{"enter", "sort into this folder"}, {"esc", "into the same folder"}}
	}

	hs := []hint{{"enter", m.enterLabel()}}
	if m.mlPending > 0 && !m.classifying {
		hs = append(hs, hint{"m", "classify"})
	}
	if m.focus == focusPlan {
		hs = append(hs, hint{"l", "files"})
	}
	hs = append(hs, hint{"p", "preset"})
	if m.presets[m.preset] == customPreset {
		hs = append(hs, hint{"t", "template"})
	}
	copyLabel := "copy"
	if m.copy {
		copyLabel = "move"
	}
	return append(hs, hint{"c", copyLabel}, hint{"d", "destination"}, hint{"r", "subfolders"}, hint{"h", "history"}, hint{"?", "help"}, hint{"q", "quit"})
}

func (m model) enterLabel() string {
	if m.plan == nil {
		return "sort"
	}
	verb := "move"
	if m.plan.FileAction == string(types.OpCopy) {
		verb = "copy"
	}
	return verb + " " + report.Count(m.plan.Stats().Files, "file")
}

// hintSegments are the hints as drawn, with their columns.
func hintSegments(hints []hint) (text []string, from, to []int) {
	pos := 1
	for _, h := range hints {
		seg := h.key + " " + h.label
		text = append(text, seg)
		from = append(from, pos)
		to = append(to, pos+lipgloss.Width(seg))
		pos += lipgloss.Width(seg) + 3
	}
	return text, from, to
}

// hintAt finds the hint right under column x; the gaps between hints are not hints.
func hintAt(hints []hint, x int) (string, bool) {
	_, from, to := hintSegments(hints)
	for i := range hints {
		if x >= from[i] && x < to[i] {
			return hints[i].key, true
		}
	}
	return "", false
}

func (m model) barLine() string {
	if m.screen == screenConfirm || m.screen == screenUndoConfirm {
		question := m.undoQuestion
		if m.screen == screenConfirm {
			question = report.Question(m.plan)
		}
		line := " " + question + "   y yes · n no"
		return questionStyle.Width(m.width).Render(fitWidth(line, m.width))
	}

	if m.notice != "" {
		return questionStyle.Width(m.width).Render(fitWidth(" "+m.notice, m.width))
	}

	hints := m.hints()
	text, _, _ := hintSegments(hints)
	var b strings.Builder
	b.WriteString(barStyle.Render(" "))
	width := 1
	for i, seg := range text {
		w := lipgloss.Width(seg) + 3
		if width+w > m.width {
			break
		}
		b.WriteString(barKeyStyle.Render(hints[i].key))
		b.WriteString(barStyle.Render(" " + hints[i].label + "   "))
		width += w
	}
	if width < m.width {
		b.WriteString(barStyle.Render(strings.Repeat(" ", m.width-width)))
	}
	return b.String()
}

// fitWidthStyled cuts a styled line by its visible width.
func fitWidthStyled(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}
