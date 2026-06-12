package tui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const treeHeight = 7

// ── node ──────────────────────────────────────────────────────────────────────

type treeNode struct {
	path     string
	name     string
	depth    int
	expanded bool
}

// ── model ─────────────────────────────────────────────────────────────────────

type FileTree struct {
	title    string
	nodes    []treeNode
	cursor   int
	offset   int
	Selected string
	focused  bool
	width    int
}

func NewFileTree(title string) FileTree {
	home, _ := os.UserHomeDir()
	ft := FileTree{
		title:    title,
		Selected: home,
		width:    80,
		nodes: []treeNode{{
			path:  home,
			name:  "~",
			depth: 0,
		}},
	}
	ft.expand(0)
	return ft
}

// ── update ────────────────────────────────────────────────────────────────────

func (ft FileTree) Update(msg tea.Msg) (FileTree, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if ft.cursor > 0 {
				ft.cursor--
				ft.adjustScroll()
			}
		case "down", "j":
			if ft.cursor < len(ft.nodes)-1 {
				ft.cursor++
				ft.adjustScroll()
			}
		case "right", "l":
			if !ft.nodes[ft.cursor].expanded {
				ft.expand(ft.cursor)
			}
		case "left", "h":
			if ft.nodes[ft.cursor].expanded {
				ft.collapse(ft.cursor)
			} else {
				ft.goToParent()
			}
		case "enter", " ":
			ft.Selected = ft.nodes[ft.cursor].path
		}
	}
	return ft, nil
}

// ── view ──────────────────────────────────────────────────────────────────────

var (
	treeCursorStyle = lipgloss.NewStyle().
			Background(colorMid).
			Foreground(colorCream).
			Bold(true)

	treeNodeStyle = lipgloss.NewStyle().
			Foreground(colorSage)

	treeLabelStyle = lipgloss.NewStyle().
			Foreground(colorCream).
			Bold(true)

	treeBoxActive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMid).
			Padding(0, 1)

	treeBoxInactive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorDimmed).
			Padding(0, 1)
)

func (ft FileTree) View() string {
	innerW := ft.width - 4 // border×2 + padding×2
	if innerW < 10 {
		innerW = 10
	}

	header := treeLabelStyle.Render(ft.title+":") + " " + treeNodeStyle.Render(ft.Selected)

	var sb strings.Builder
	end := ft.offset + treeHeight
	if end > len(ft.nodes) {
		end = len(ft.nodes)
	}

	for i := ft.offset; i < end; i++ {
		node := ft.nodes[i]
		indent := strings.Repeat("  ", node.depth)
		icon := "▸ "
		if node.expanded {
			icon = "▾ "
		}
		line := indent + icon + node.name + "/"
		if i == ft.cursor {
			sb.WriteString(treeCursorStyle.Width(innerW).Render(line) + "\n")
		} else {
			sb.WriteString(treeNodeStyle.Width(innerW).Render(line) + "\n")
		}
	}

	for i := end - ft.offset; i < treeHeight; i++ {
		sb.WriteString("\n")
	}

	box := treeBoxInactive
	if ft.focused {
		box = treeBoxActive
	}
	return box.Width(innerW).Render(header + "\n" + sb.String())
}

// ── tree operations ───────────────────────────────────────────────────────────

func (ft *FileTree) expand(i int) {
	if ft.nodes[i].expanded {
		return
	}
	children, err := readDirEntries(ft.nodes[i].path, ft.nodes[i].depth+1)
	if err != nil || len(children) == 0 {
		return
	}
	ft.nodes[i].expanded = true
	next := make([]treeNode, 0, len(ft.nodes)+len(children))
	next = append(next, ft.nodes[:i+1]...)
	next = append(next, children...)
	next = append(next, ft.nodes[i+1:]...)
	ft.nodes = next
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
	if ft.cursor < ft.offset {
		ft.offset = ft.cursor
	}
	if ft.cursor >= ft.offset+treeHeight {
		ft.offset = ft.cursor - treeHeight + 1
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// HandleMouseClick receives a row relative to the tree content area (0 = first visible node).
func (ft FileTree) HandleMouseClick(relRow int) FileTree {
	idx := ft.offset + relRow
	if idx < 0 || idx >= len(ft.nodes) {
		return ft
	}
	if ft.cursor == idx {
		// second click on same node: toggle expand / select
		if ft.nodes[idx].expanded {
			ft.collapse(idx)
		} else {
			ft.expand(idx)
			ft.Selected = ft.nodes[idx].path
		}
	} else {
		ft.cursor = idx
		ft.Selected = ft.nodes[idx].path
	}
	return ft
}

func readDirEntries(path string, depth int) ([]treeNode, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var nodes []treeNode
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		nodes = append(nodes, treeNode{
			path:  filepath.Join(path, e.Name()),
			name:  e.Name(),
			depth: depth,
		})
	}
	return nodes, nil
}
