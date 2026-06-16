package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func pressKey(m model, k tea.KeyType) (model, tea.Cmd) {
	next, cmd := m.Update(tea.KeyMsg{Type: k})
	return next.(model), cmd
}

func withSize(m model, w, h int) model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(model)
}

// tallModel gives a terminal large enough that both trees stack vertically.
func tallModel() model { return withSize(newModel(), 200, 60) }

// ── window size ───────────────────────────────────────────────────────────────

func TestWindowSizeSetsFields(t *testing.T) {
	m := withSize(newModel(), 120, 40)
	if m.width != 120 || m.height != 40 {
		t.Fatalf("width=%d height=%d, want 120 40", m.width, m.height)
	}
}

// ── tab navigation ────────────────────────────────────────────────────────────

func TestTabCyclesAllFields(t *testing.T) {
	m := newModel()
	for i := 0; i < fieldCount; i++ {
		m, _ = pressKey(m, tea.KeyTab)
	}
	if m.focused != fieldPrimary {
		t.Fatalf("after full cycle focused=%d, want fieldPrimary (%d)", m.focused, fieldPrimary)
	}
}

func TestShiftTabGoesBack(t *testing.T) {
	m := newModel()
	m, _ = pressKey(m, tea.KeyTab)    // → fieldSecondary
	m, _ = pressKey(m, tea.KeyShiftTab) // → fieldPrimary
	if m.focused != fieldPrimary {
		t.Fatalf("focused=%d, want fieldPrimary", m.focused)
	}
}

func TestShiftTabWrapsToLast(t *testing.T) {
	m := newModel() // focused=fieldPrimary
	m, _ = pressKey(m, tea.KeyShiftTab)
	if m.focused != fieldCount-1 {
		t.Fatalf("focused=%d, want last field (%d)", m.focused, fieldCount-1)
	}
}

func TestTreeFocusSyncsOnTab(t *testing.T) {
	m := newModel()
	for m.focused != fieldSource {
		m, _ = pressKey(m, tea.KeyTab)
	}
	if !m.sourceTree.focused {
		t.Error("sourceTree.focused should be true when fieldSource is active")
	}
	if m.destTree.focused {
		t.Error("destTree.focused should be false when fieldSource is active")
	}

	m, _ = pressKey(m, tea.KeyTab) // → fieldDest
	if m.sourceTree.focused {
		t.Error("sourceTree.focused should be false when fieldDest is active")
	}
	if !m.destTree.focused {
		t.Error("destTree.focused should be true when fieldDest is active")
	}
}

func TestTreeFocusClearsOnLeavingTree(t *testing.T) {
	m := newModel()
	for m.focused != fieldSource {
		m, _ = pressKey(m, tea.KeyTab)
	}
	m, _ = pressKey(m, tea.KeyTab) // → fieldDest
	m, _ = pressKey(m, tea.KeyTab) // → fieldSort
	if m.sourceTree.focused || m.destTree.focused {
		t.Error("tree focus should clear when moving past fieldDest")
	}
}

// ── primary selector ──────────────────────────────────────────────────────────

func TestPrimaryRightAdvances(t *testing.T) {
	m := newModel() // focused=fieldPrimary by default
	m, _ = pressKey(m, tea.KeyRight)
	if m.primary != 1 {
		t.Fatalf("primary=%d, want 1", m.primary)
	}
}

func TestPrimaryLeftWrapsAround(t *testing.T) {
	m := newModel()
	m, _ = pressKey(m, tea.KeyLeft)
	want := len(primaryOptions) - 1
	if m.primary != want {
		t.Fatalf("primary=%d, want %d (wrap)", m.primary, want)
	}
}

func TestPrimaryRightWrapsAround(t *testing.T) {
	m := newModel()
	m.primary = len(primaryOptions) - 1
	m, _ = pressKey(m, tea.KeyRight)
	if m.primary != 0 {
		t.Fatalf("primary=%d after wrap, want 0", m.primary)
	}
}

func TestPrimaryKeyIgnoredWhenNotFocused(t *testing.T) {
	m := newModel()
	m, _ = pressKey(m, tea.KeyTab) // → fieldSecondary
	before := m.primary
	m, _ = pressKey(m, tea.KeyRight)
	if m.primary != before {
		t.Error("primary should not change when fieldSecondary is focused")
	}
}

// ── secondary selector ────────────────────────────────────────────────────────

func TestSecondaryRightAdvances(t *testing.T) {
	m := newModel()
	m, _ = pressKey(m, tea.KeyTab) // → fieldSecondary
	m, _ = pressKey(m, tea.KeyRight)
	if m.secondary != 1 {
		t.Fatalf("secondary=%d, want 1", m.secondary)
	}
}

func TestSecondaryLeftWrapsAround(t *testing.T) {
	m := newModel()
	m, _ = pressKey(m, tea.KeyTab) // → fieldSecondary
	m, _ = pressKey(m, tea.KeyLeft)
	want := len(secondaryOptions) - 1
	if m.secondary != want {
		t.Fatalf("secondary=%d, want %d (wrap)", m.secondary, want)
	}
}

func TestSecondaryKeyIgnoredWhenNotFocused(t *testing.T) {
	m := newModel() // focused=fieldPrimary
	before := m.secondary
	m, _ = pressKey(m, tea.KeyRight)
	if m.secondary != before {
		t.Error("secondary should not change when fieldPrimary is focused")
	}
}

// ── confirm / cancel ──────────────────────────────────────────────────────────

func TestEnterOnSortSetsActionSort(t *testing.T) {
	m := newModel()
	m.focused = fieldSort
	m, cmd := pressKey(m, tea.KeyEnter)
	if m.action != ActionSort {
		t.Fatalf("action=%v, want ActionSort", m.action)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (tea.Quit)")
	}
}

func TestEnterOnUndoSetsActionUndo(t *testing.T) {
	m := newModel()
	m.focused = fieldUndo
	m, cmd := pressKey(m, tea.KeyEnter)
	if m.action != ActionUndo {
		t.Fatalf("action=%v, want ActionUndo", m.action)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (tea.Quit)")
	}
}

func TestEnterOnQuitLeavesActionCancel(t *testing.T) {
	m := newModel()
	m.focused = fieldQuit
	m, cmd := pressKey(m, tea.KeyEnter)
	if m.action != ActionCancel {
		t.Fatalf("action=%v, want ActionCancel", m.action)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (tea.Quit)")
	}
}

func TestCtrlCLeavesActionCancel(t *testing.T) {
	m := newModel()
	m, cmd := pressKey(m, tea.KeyCtrlC)
	if m.action != ActionCancel {
		t.Fatalf("action=%v, want ActionCancel", m.action)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (tea.Quit)")
	}
}

func TestEnterOnNonConfirmFieldDoesNothing(t *testing.T) {
	m := newModel() // focused=fieldPrimary
	m, cmd := pressKey(m, tea.KeyEnter)
	if m.action != ActionCancel {
		t.Errorf("action=%v, want ActionCancel (enter on selector should be no-op)", m.action)
	}
	if cmd != nil {
		t.Error("expected nil cmd when enter on non-confirm field")
	}
}

// ── Result ────────────────────────────────────────────────────────────────────

func TestResultCancelledHelper(t *testing.T) {
	cases := []struct {
		action Action
		want   bool
	}{
		{ActionCancel, true},
		{ActionSort, false},
		{ActionUndo, false},
	}
	for _, c := range cases {
		r := Result{Action: c.action}
		if got := r.Cancelled(); got != c.want {
			t.Errorf("Result{Action:%v}.Cancelled()=%v, want %v", c.action, got, c.want)
		}
	}
}

// ── handleClick — bar ─────────────────────────────────────────────────────────

// barX returns the x coordinate of the leftmost character of a bar button.
func barX(field int) int {
	sortW := lipgloss.Width(barItemInactive.Render("Sort"))
	undoStart := sortW + 1
	undoW := lipgloss.Width(barItemInactive.Render("Undo"))
	quitStart := undoStart + undoW + 1
	switch field {
	case fieldSort:
		return 0
	case fieldUndo:
		return undoStart
	case fieldQuit:
		return quitStart
	}
	return 0
}

func TestClickSortInBar(t *testing.T) {
	m := withSize(newModel(), 120, 30)
	m = m.handleClick(barX(fieldSort), m.height-1)
	if m.focused != fieldSort {
		t.Fatalf("focused=%d, want fieldSort (%d)", m.focused, fieldSort)
	}
}

func TestClickUndoInBar(t *testing.T) {
	m := withSize(newModel(), 120, 30)
	m = m.handleClick(barX(fieldUndo), m.height-1)
	if m.focused != fieldUndo {
		t.Fatalf("focused=%d, want fieldUndo (%d)", m.focused, fieldUndo)
	}
}

func TestClickQuitInBar(t *testing.T) {
	m := withSize(newModel(), 120, 30)
	m = m.handleClick(barX(fieldQuit), m.height-1)
	if m.focused != fieldQuit {
		t.Fatalf("focused=%d, want fieldQuit (%d)", m.focused, fieldQuit)
	}
}

func TestClickFileActionSelector(t *testing.T) {
	m := tallModel()
	m = m.handleClick(20, m.headerH()+2)
	if m.focused != fieldAction {
		t.Fatalf("focused=%d, want fieldAction (%d)", m.focused, fieldAction)
	}
}

func TestKeyboardToggleCopyMode(t *testing.T) {
	m := newModel()
	m.focused = fieldAction
	m, _ = pressKey(m, tea.KeyEnter)
	if !m.copyMode {
		t.Fatal("enter on fieldAction should flip copyMode to true")
	}
	m, _ = pressKey(m, tea.KeyEnter)
	if m.copyMode {
		t.Fatal("second enter should flip back to false")
	}
}

func TestLeftRightToggleCopyMode(t *testing.T) {
	m := newModel()
	m.focused = fieldAction
	m, _ = pressKey(m, tea.KeyRight)
	if !m.copyMode {
		t.Fatal("right on fieldAction should flip copyMode to true")
	}
	m, _ = pressKey(m, tea.KeyLeft)
	if m.copyMode {
		t.Fatal("left on fieldAction should flip copyMode back to false")
	}
}

func TestBarClickClearsTreeFocus(t *testing.T) {
	m := withSize(newModel(), 120, 30)
	m.sourceTree.focused = true
	m = m.handleClick(barX(fieldSort), m.height-1)
	if m.sourceTree.focused || m.destTree.focused {
		t.Error("bar click should clear tree focus")
	}
}

// ── handleClick — selectors ───────────────────────────────────────────────────

func TestClickPrimarySelector(t *testing.T) {
	m := tallModel()
	m = m.handleClick(20, m.headerH())
	if m.focused != fieldPrimary {
		t.Fatalf("focused=%d, want fieldPrimary (%d)", m.focused, fieldPrimary)
	}
}

func TestClickSecondarySelector(t *testing.T) {
	m := tallModel()
	m = m.handleClick(20, m.headerH()+1)
	if m.focused != fieldSecondary {
		t.Fatalf("focused=%d, want fieldSecondary (%d)", m.focused, fieldSecondary)
	}
}

// ── handleClick — trees (vertical mode) ──────────────────────────────────────

func TestClickSourceTreeVertical(t *testing.T) {
	m := tallModel()
	if m.isHorizontal() {
		t.Skip("layout is horizontal at this terminal size")
	}
	m = m.handleClick(20, m.sourceTreeY()+1)
	if m.focused != fieldSource {
		t.Fatalf("focused=%d, want fieldSource (%d)", m.focused, fieldSource)
	}
	if !m.sourceTree.focused {
		t.Error("sourceTree.focused should be true after clicking it")
	}
}

func TestClickDestTreeVertical(t *testing.T) {
	m := tallModel()
	if m.isHorizontal() {
		t.Skip("layout is horizontal at this terminal size")
	}
	m = m.handleClick(20, m.destTreeY()+1)
	if m.focused != fieldDest {
		t.Fatalf("focused=%d, want fieldDest (%d)", m.focused, fieldDest)
	}
	if !m.destTree.focused {
		t.Error("destTree.focused should be true after clicking it")
	}
}

func TestClickOutsideTreesDoesNotChangeFocus(t *testing.T) {
	m := tallModel()
	if m.isHorizontal() {
		t.Skip("layout is horizontal at this terminal size")
	}
	m.focused = fieldPrimary
	// click between the two trees (below source, above dest)
	midY := (m.sourceTreeY() + treeHeight + 3 + m.destTreeY()) / 2
	m = m.handleClick(20, midY)
	if m.focused != fieldPrimary {
		t.Errorf("focused changed to %d after clicking empty area", m.focused)
	}
}

// ── layout helpers ────────────────────────────────────────────────────────────

func TestHeaderHLargeTerminal(t *testing.T) {
	m := model{height: 200}
	if got := m.headerH(); got != logoHeight+2 {
		t.Fatalf("headerH=%d, want %d (logoHeight+2)", got, logoHeight+2)
	}
}

func TestHeaderHSmallTerminal(t *testing.T) {
	m := model{height: 5}
	if got := m.headerH(); got != 3 {
		t.Fatalf("headerH=%d, want 3 (fallback)", got)
	}
}

func TestHeaderHZeroHeight(t *testing.T) {
	m := model{height: 0}
	if got := m.headerH(); got != 3 {
		t.Fatalf("headerH=%d, want 3 for zero height", got)
	}
}

func TestIsHorizontalSmallHeight(t *testing.T) {
	m := model{height: 20}
	if !m.isHorizontal() {
		t.Error("expected isHorizontal=true for small height (trees don't fit vertically)")
	}
}

func TestIsHorizontalLargeHeight(t *testing.T) {
	m := model{height: 200}
	if m.isHorizontal() {
		t.Error("expected isHorizontal=false for large height (trees fit vertically)")
	}
}

func TestIsHorizontalZeroHeight(t *testing.T) {
	m := model{height: 0}
	if m.isHorizontal() {
		t.Error("expected isHorizontal=false for zero height (not yet sized)")
	}
}

func TestSourceTreeYAfterHeader(t *testing.T) {
	m := tallModel()
	srcY := m.sourceTreeY()
	if srcY <= m.headerH() {
		t.Errorf("sourceTreeY=%d should be > headerH=%d", srcY, m.headerH())
	}
}

func TestDestTreeYAfterSourceTree(t *testing.T) {
	m := tallModel()
	if m.destTreeY() <= m.sourceTreeY() {
		t.Errorf("destTreeY=%d should be > sourceTreeY=%d", m.destTreeY(), m.sourceTreeY())
	}
}

// ── View smoke ────────────────────────────────────────────────────────────────

func TestViewNoSizeDoesNotPanic(t *testing.T) {
	m := newModel()
	out := m.View()
	if out == "" {
		t.Error("View() returned empty string")
	}
}

func TestViewContainsBarButtons(t *testing.T) {
	m := tallModel()
	out := m.View()
	for _, label := range []string{"Sort", "Undo", "Quit"} {
		if !strings.Contains(out, label) {
			t.Errorf("View() missing %q button", label)
		}
	}
}

func TestViewToggleShowsCopy(t *testing.T) {
	m := tallModel()
	m.copyMode = true
	out := m.View()
	if !strings.Contains(out, "copy") {
		t.Error("View() should show 'copy' when copyMode is true")
	}
}

func TestViewContainsSelectorLabels(t *testing.T) {
	m := tallModel()
	out := m.View()
	for _, label := range []string{"Primary", "Secondary"} {
		if !strings.Contains(out, label) {
			t.Errorf("View() missing selector label %q", label)
		}
	}
}

func TestViewTooNarrowMessage(t *testing.T) {
	m := withSize(newModel(), 10, 40)
	out := m.View()
	if !strings.Contains(out, "narrow") {
		t.Error("View() should show too-narrow message when width < logoWidth")
	}
}

func TestViewDefaultCriteriaVisible(t *testing.T) {
	m := tallModel()
	out := m.View()
	// primary[0]="mimetype", secondary[0]="none"
	if !strings.Contains(out, primaryOptions[0]) {
		t.Errorf("View() missing default primary option %q", primaryOptions[0])
	}
	if !strings.Contains(out, secondaryOptions[0]) {
		t.Errorf("View() missing default secondary option %q", secondaryOptions[0])
	}
}
