package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── criteria ──────────────────────────────────────────────────────────────────

var primaryOptions = []string{"mimetype", "extension", "date", "year", "month", "size", "ml"}
var secondaryOptions = []string{"none", "mimetype", "extension", "date", "year", "month", "size", "ml"}

// ── fields ────────────────────────────────────────────────────────────────────

const (
	fieldPrimary = iota
	fieldSecondary
	fieldAction // file action selector (move / copy)
	fieldSource
	fieldDest
	fieldSort
	fieldUndo
	fieldQuit
	fieldCount
)

// ── model ─────────────────────────────────────────────────────────────────────

type model struct {
	focused    int
	primary    int
	secondary  int
	copyMode   bool // false = move (default), true = copy
	width      int
	height     int
	sourceTree FileTree
	destTree   FileTree
	action     Action // set on exit: ActionSort, ActionUndo, or ActionCancel
}

func newModel() model {
	return model{
		sourceTree: NewFileTree("Source"),
		destTree:   NewFileTree("Destination"),
	}
}

func (m model) Init() tea.Cmd { return tea.HideCursor }

// ── update ────────────────────────────────────────────────────────────────────

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
					m.action = ActionSort
					return m, tea.Quit
				}
			case fieldUndo:
				if msg.String() == "enter" {
					m.action = ActionUndo
					return m, tea.Quit
				}
			case fieldAction:
				switch msg.String() {
				case "enter", " ", "left", "right":
					m.copyMode = !m.copyMode
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
			switch m.focused {
			case fieldSort:
				m.action = ActionSort
				return m, tea.Quit
			case fieldUndo:
				m.action = ActionUndo
				return m, tea.Quit
			case fieldQuit:
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

// ── click routing ─────────────────────────────────────────────────────────────

func (m model) handleClick(x, y int) model {
	m.sourceTree.editing = false
	m.destTree.editing = false

	// bottom bar
	if y == m.height-1 {
		sortW := lipgloss.Width(barItemInactive.Render("Sort"))
		undoStart := sortW + 1
		undoW := lipgloss.Width(barItemInactive.Render("Undo"))
		quitStart := undoStart + undoW + 1
		quitW := lipgloss.Width(barItemInactive.Render("Quit"))
		switch {
		case x < sortW:
			m.focused = fieldSort
		case x >= undoStart && x < undoStart+undoW:
			m.focused = fieldUndo
		case x >= quitStart && x < quitStart+quitW:
			m.focused = fieldQuit
		}
		m.sourceTree.focused = false
		m.destTree.focused = false
		return m
	}

	// selectors
	primaryY := m.headerH()
	secondaryY := m.headerH() + 1
	actionY := m.headerH() + 2
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
	if y == actionY {
		m.focused = fieldAction
		m.sourceTree.focused = false
		m.destTree.focused = false
		return m
	}

	// trees
	srcY := m.sourceTreeY()
	treeBoxH := treeHeight + 3

	if m.isHorizontal() {
		if y >= srcY && y < srcY+treeBoxH {
			relRow := y - srcY - 2
			leftPad := (m.width - (m.sourceTree.width + 1 + m.destTree.width)) / 2
			relX := x - leftPad
			if relX == m.sourceTree.width {
				return m // divider column — ignore
			}
			if relX < m.sourceTree.width {
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
