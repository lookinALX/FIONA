package tui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type treeNode struct {
	path     string
	name     string
	depth    int
	expanded bool
}

// FileTree is a folder picker. The selected folder is always the one under
// the cursor, so that the plan follows the cursor.
type FileTree struct {
	nodes   []treeNode
	cursor  int
	offset  int
	height  int // rows of folders
	width   int
	editing bool   // typing a path
	input   string // the path being typed
	err     string // the typed path is not a folder
}

// NewFileTree opens the tree at start: rooted at the home folder when start
// lies inside it, at the file system root otherwise, expanded down to start.
func NewFileTree(start string) FileTree {
	root := treeRoot(start)
	ft := FileTree{
		nodes:  []treeNode{{path: root, name: rootName(root)}},
		height: 10,
		width:  30,
	}

	idx := 0
	if rel, err := filepath.Rel(root, start); err == nil && rel != "." && filepath.IsLocal(rel) {
		for _, part := range strings.Split(rel, string(filepath.Separator)) {
			idx = ft.ensureChild(idx, filepath.Join(ft.nodes[idx].path, part))
		}
	}
	ft.expand(idx)
	ft.cursor = idx
	ft.adjustScroll()
	return ft
}

func treeRoot(start string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(home, start); err == nil && (rel == "." || filepath.IsLocal(rel)) {
			return home
		}
	}
	return filepath.VolumeName(start) + string(filepath.Separator)
}

func rootName(root string) string {
	if home, err := os.UserHomeDir(); err == nil && root == home {
		return "~"
	}
	return root
}

// Selected is the folder under the cursor.
func (ft FileTree) Selected() string {
	return ft.nodes[ft.cursor].path
}

// Update handles a key; changed reports that another folder is selected now.
func (ft FileTree) Update(msg tea.KeyMsg) (_ FileTree, changed bool) {
	if ft.editing {
		return ft.updateEditing(msg)
	}

	before := ft.cursor
	switch msg.String() {
	case "e":
		ft.editing = true
		ft.input = ft.Selected()
		ft.err = ""
	case "up", "k":
		ft.move(-1)
	case "down", "j":
		ft.move(1)
	case "pgup":
		ft.move(-ft.height)
	case "pgdown":
		ft.move(ft.height)
	case "home":
		ft.move(-len(ft.nodes))
	case "end":
		ft.move(len(ft.nodes))
	case "right", "l":
		ft.expand(ft.cursor)
	case "left", "h":
		if ft.nodes[ft.cursor].expanded {
			ft.collapse(ft.cursor)
		} else {
			ft.goToParent()
		}
	}
	return ft, ft.cursor != before
}

func (ft *FileTree) move(delta int) {
	ft.cursor = max(0, min(len(ft.nodes)-1, ft.cursor+delta))
	ft.adjustScroll()
}

func (ft FileTree) updateEditing(msg tea.KeyMsg) (FileTree, bool) {
	switch msg.Type {
	case tea.KeyEsc:
		ft.editing, ft.input, ft.err = false, "", ""
	case tea.KeyEnter:
		path := expandHome(ft.input)
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			ft.err = "not a folder"
			return ft, false
		}
		height, width := ft.height, ft.width
		ft = NewFileTree(path)
		ft.height, ft.width = height, width
		ft.adjustScroll()
		return ft, true
	case tea.KeyBackspace:
		if r := []rune(ft.input); len(r) > 0 {
			ft.input = string(r[:len(r)-1])
		}
		ft.err = ""
	case tea.KeyRunes, tea.KeySpace:
		ft.input += string(msg.Runes)
		ft.err = ""
	}
	return ft, false
}

// StopEditing leaves the path field, keeping the selection.
func (ft FileTree) StopEditing() FileTree {
	ft.editing, ft.input, ft.err = false, "", ""
	return ft
}

// Click selects the folder on a visible row; a second click expands or collapses it.
func (ft FileTree) Click(row int) (_ FileTree, changed bool) {
	idx := ft.offset + row
	if row < 0 || idx >= len(ft.nodes) {
		return ft, false
	}
	if idx == ft.cursor {
		if ft.nodes[idx].expanded {
			ft.collapse(idx)
		} else {
			ft.expand(idx)
		}
		return ft, false
	}
	ft.cursor = idx
	return ft, true
}

// SetSize sets the rows of folders and the width of the lines.
func (ft FileTree) SetSize(width, height int) FileTree {
	ft.width, ft.height = max(width, 10), max(height, 1)
	ft.adjustScroll()
	return ft
}

// Lines renders the visible part of the tree, one string per row.
func (ft FileTree) Lines(focused bool) []string {
	var lines []string
	if ft.editing {
		field := "Path: " + ft.input + "█"
		if ft.err != "" {
			field += "  " + ft.err
		}
		lines = append(lines, fitWidth(field, ft.width))
	}

	rows := ft.height - len(lines)
	end := min(ft.offset+rows, len(ft.nodes))
	for i := ft.offset; i < end; i++ {
		node := ft.nodes[i]
		icon := "▸ "
		if node.expanded {
			icon = "▾ "
		}
		name := node.name
		if !strings.HasSuffix(name, string(filepath.Separator)) {
			name += "/"
		}
		line := fitWidth(strings.Repeat("  ", node.depth)+icon+name, ft.width)

		switch {
		case i == ft.cursor && focused:
			line = cursorStyle.Width(ft.width).Render(line)
		case i == ft.cursor:
			line = cursorDimStyle.Width(ft.width).Render(line)
		default:
			line = textStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}

// ── tree operations ───────────────────────────────────────────────────────────

func (ft *FileTree) expand(i int) {
	if ft.nodes[i].expanded {
		return
	}
	children := readDirs(ft.nodes[i].path, ft.nodes[i].depth+1)
	if len(children) == 0 {
		return
	}
	ft.nodes[i].expanded = true
	next := make([]treeNode, 0, len(ft.nodes)+len(children))
	next = append(next, ft.nodes[:i+1]...)
	next = append(next, children...)
	next = append(next, ft.nodes[i+1:]...)
	ft.nodes = next
}

// ensureChild expands node i and returns the index of its child at path,
// adding it when the listing does not show it (a hidden folder).
func (ft *FileTree) ensureChild(i int, path string) int {
	ft.expand(i)
	depth := ft.nodes[i].depth + 1
	j := i + 1
	for ; j < len(ft.nodes) && ft.nodes[j].depth >= depth; j++ {
		if ft.nodes[j].depth == depth && ft.nodes[j].path == path {
			return j
		}
	}
	ft.nodes[i].expanded = true
	node := treeNode{path: path, name: filepath.Base(path), depth: depth}
	ft.nodes = append(ft.nodes[:i+1], append([]treeNode{node}, ft.nodes[i+1:]...)...)
	return i + 1
}

func (ft *FileTree) collapse(i int) {
	if !ft.nodes[i].expanded {
		return
	}
	depth := ft.nodes[i].depth
	end := i + 1
	for end < len(ft.nodes) && ft.nodes[end].depth > depth {
		end++
	}
	ft.nodes[i].expanded = false
	removed := end - i - 1
	ft.nodes = append(ft.nodes[:i+1], ft.nodes[end:]...)
	if ft.cursor > i && ft.cursor < end {
		ft.cursor = i
	} else if ft.cursor >= end {
		ft.cursor -= removed
	}
	ft.adjustScroll()
}

func (ft *FileTree) goToParent() {
	depth := ft.nodes[ft.cursor].depth
	for i := ft.cursor - 1; i >= 0; i-- {
		if ft.nodes[i].depth < depth {
			ft.cursor = i
			ft.adjustScroll()
			return
		}
	}
}

func (ft *FileTree) adjustScroll() {
	rows := ft.height
	if ft.editing {
		rows--
	}
	rows = max(rows, 1)
	if ft.cursor < ft.offset {
		ft.offset = ft.cursor
	}
	if ft.cursor >= ft.offset+rows {
		ft.offset = ft.cursor - rows + 1
	}
}

func readDirs(path string, depth int) []treeNode {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	var nodes []treeNode
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		nodes = append(nodes, treeNode{path: filepath.Join(path, e.Name()), name: e.Name(), depth: depth})
	}
	return nodes
}

// expandHome resolves a leading "~" to the home folder.
func expandHome(path string) string {
	path = strings.TrimSpace(path)
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

// fitWidth cuts a line that is too wide, marking the cut with "…".
func fitWidth(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > width {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
