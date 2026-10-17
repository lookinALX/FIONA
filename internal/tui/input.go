package tui

import tea "github.com/charmbracelet/bubbletea"

// lineInput is a one-line text field: typing appends, backspace deletes.
type lineInput struct {
	value   string
	focused bool
}

// Update returns true when the value changed.
func (in lineInput) Update(msg tea.KeyMsg) (lineInput, bool) {
	switch msg.Type {
	case tea.KeyBackspace:
		if r := []rune(in.value); len(r) > 0 {
			in.value = string(r[:len(r)-1])
			return in, true
		}
	case tea.KeyRunes, tea.KeySpace:
		in.value += string(msg.Runes)
		return in, true
	}
	return in, false
}

func (in lineInput) View(prompt string) string {
	if in.focused {
		return textStyle.Render(prompt+in.value) + cursorStyle.Render(" ")
	}
	return dimStyle.Render(prompt + in.value)
}
