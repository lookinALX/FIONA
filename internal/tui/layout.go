package tui

// headerH returns the number of terminal rows the header occupies.
// Shows the logo as soon as there is room for logo + selectors + one tree + bar.
// Falls back to a single "FIONA" text line when the terminal is too short.
func (m model) headerH() int {
	minForLogo := logoHeight + treeHeight + 9
	if m.height > 0 && m.height >= minForLogo {
		return logoHeight + 2 // "\n" + logo + "\n\n"
	}
	return 3 // "\n  FIONA\n\n"
}

// sourceTreeY returns the absolute terminal row where the source tree box starts.
func (m model) sourceTreeY() int {
	return m.headerH() + 4 // + primary(1) + secondary(1) + fileaction(1) + blank(1)
}

// destTreeY returns the absolute terminal row where the destination tree box starts.
// Only meaningful in vertical (stacked) mode.
func (m model) destTreeY() int {
	return m.sourceTreeY() + treeHeight + 4 // box(treeHeight+3) + "\n" separator
}

// isHorizontal reports whether both trees should be placed side by side.
// This happens when they don't both fit stacked vertically in the current terminal.
func (m model) isHorizontal() bool {
	if m.height == 0 {
		return false
	}
	return m.destTreeY()+treeHeight+3 >= m.height
}

// updateTreeWidths sets each tree's width based on the current layout mode.
// Trees are sized to logoWidth so they stay centred with the rest of the UI.
func (m *model) updateTreeWidths() {
	cw := logoWidth
	if cw > m.width {
		cw = m.width
	}
	if m.isHorizontal() {
		half := (cw - 1) / 2 // -1 reserves space for the │ divider
		m.sourceTree.width = half
		m.destTree.width = cw - 1 - half
	} else {
		m.sourceTree.width = cw
		m.destTree.width = cw
	}
}
