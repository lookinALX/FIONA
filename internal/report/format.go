// Package report turns plans and results into text. The CLI prints it, the
// TUI shows the same lines, so both always tell the same numbers.
package report

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Styles colour the marks and dim the secondary text. The zero value is plain text.
type Styles struct {
	OK, Warn, Bad, Dim, Bold lipgloss.Style
	enabled                  bool
}

// NewStyles colours text for the renderer; it stays plain when the renderer
// writes to something that is not a terminal or NO_COLOR is set.
func NewStyles(r *lipgloss.Renderer) Styles {
	return Styles{
		OK:      r.NewStyle().Foreground(lipgloss.Color("2")),
		Warn:    r.NewStyle().Foreground(lipgloss.Color("3")),
		Bad:     r.NewStyle().Foreground(lipgloss.Color("1")),
		Dim:     r.NewStyle().Faint(true),
		Bold:    r.NewStyle().Bold(true),
		enabled: true,
	}
}

func (s Styles) ok(text string) string   { return s.apply(s.OK, text) }
func (s Styles) warn(text string) string { return s.apply(s.Warn, text) }
func (s Styles) bad(text string) string  { return s.apply(s.Bad, text) }
func (s Styles) dim(text string) string  { return s.apply(s.Dim, text) }
func (s Styles) bold(text string) string { return s.apply(s.Bold, text) }

func (s Styles) apply(style lipgloss.Style, text string) string {
	if !s.enabled {
		return text
	}
	return style.Render(text)
}

// Size formats bytes like 512 B, 2.4 KB, 412 MB, 1.2 GB.
func Size(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	value, exp := float64(bytes)/unit, 0
	for value >= unit && exp < 5 {
		value /= unit
		exp++
	}
	if value < 10 {
		return fmt.Sprintf("%.1f %cB", value, "KMGTPE"[exp])
	}
	return fmt.Sprintf("%.0f %cB", value, "KMGTPE"[exp])
}

// Count formats a number with a noun: 1 file, 2 files.
func Count(n int, noun string) string {
	return fmt.Sprintf("%d %s", n, Plural(n, noun))
}

func Plural(n int, noun string) string {
	if n == 1 {
		return noun
	}
	switch {
	case strings.HasSuffix(noun, "y"):
		return strings.TrimSuffix(noun, "y") + "ies"
	case strings.HasSuffix(noun, "s"):
		return noun + "es"
	default:
		return noun + "s"
	}
}

// Path shortens the home directory to ~.
func Path(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if rel, err := filepath.Rel(home, path); err == nil && filepath.IsLocal(rel) {
		return "~" + string(filepath.Separator) + rel
	}
	return path
}

// ShellQuote quotes a path for a command the user may copy.
func ShellQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n'\"\\$`!*?[]{}()<>|&;#~") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// When formats a moment relative to now: today 15:28, yesterday 09:10,
// 12 Oct 15:28, 12 Oct 2025 15:28.
func When(t, now time.Time) string {
	t, now = t.Local(), now.Local()
	day := func(x time.Time) time.Time { return time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, time.Local) }

	switch diff := day(now).Sub(day(t)); {
	case diff == 0:
		return "today " + t.Format("15:04")
	case diff == 24*time.Hour:
		return "yesterday " + t.Format("15:04")
	case t.Year() == now.Year():
		return t.Format("2 Jan 15:04")
	default:
		return t.Format("2 Jan 2006 15:04")
	}
}

// Duration formats how long a run took: 0.3s, 12s, 2m05s.
func Duration(d time.Duration) string {
	switch {
	case d < 100*time.Millisecond:
		return "under 0.1s"
	case d < 10*time.Second:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Minute:
		return fmt.Sprintf("%.0fs", d.Seconds())
	default:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
}

// table aligns rows: the first column to the left, the next two to the right,
// the last one as is.
type table struct {
	rows [][4]string
}

func (t *table) add(name, files, size, note string) {
	t.rows = append(t.rows, [4]string{name, files, size, note})
}

func (t *table) render(indent string, st Styles) []string {
	var w [3]int
	for _, r := range t.rows {
		for i := 0; i < 3; i++ {
			w[i] = max(w[i], lipgloss.Width(r[i]))
		}
	}

	lines := make([]string, len(t.rows))
	for i, r := range t.rows {
		line := fmt.Sprintf("%s%-*s  %*s  %*s", indent, w[0], r[0], w[1], r[1], w[2], r[2])
		if r[3] != "" {
			line += "  " + st.dim(r[3])
		}
		lines[i] = strings.TrimRight(line, " ")
	}
	return lines
}

// filesColumn right-aligns the number but not the noun: " 1 file", "86 files".
func filesColumn(counts []int) []string {
	width := 0
	for _, n := range counts {
		width = max(width, len(fmt.Sprint(n)))
	}
	out := make([]string, len(counts))
	for i, n := range counts {
		out[i] = fmt.Sprintf("%*d %s", width, n, Plural(n, "file"))
		if n == 1 {
			out[i] += " "
		}
	}
	return out
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
