package tui

import tea "github.com/charmbracelet/bubbletea"

// Result holds the user's selections after the TUI session ends.
//
// Action indicates what the user chose:
//   - ActionSort   — run sorting with the provided criteria and paths
//   - ActionUndo   — undo the last sort (criteria/paths are empty)
//   - ActionCancel — user quit without choosing an action
type Result struct {
	Action     Action // what the user requested
	Primary    string // sort criterion: "mimetype" | "extension" | "date" | "year" | "month" | "size" | "ml"
	Secondary  string // secondary criterion: same options + "none"
	SourcePath string // absolute path chosen in the Source tree
	DestPath   string // absolute path chosen in the Destination tree
	FileAction string // "move" (default) | "copy"
}

// Action is the operation the user requested from the TUI.
type Action int

const (
	ActionCancel Action = iota // Quit or ctrl+c
	ActionSort                 // Sort button confirmed
	ActionUndo                 // Undo button confirmed
)

// Cancelled reports whether the user exited without choosing an action.
func (r Result) Cancelled() bool { return r.Action == ActionCancel }

// Run launches the TUI and blocks until the user picks an action or cancels.
// Check Result.Action to determine what to do: ActionSort, ActionUndo, or ActionCancel.
func Run() (Result, error) {
	m, err := tea.NewProgram(newModel(), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	if err != nil {
		return Result{Action: ActionCancel}, err
	}
	final := m.(model)
	switch final.action {
	case ActionSort:
		fa := "move"
		if final.copyMode {
			fa = "copy"
		}
		return Result{
			Action:     ActionSort,
			Primary:    primaryOptions[final.primary],
			Secondary:  secondaryOptions[final.secondary],
			SourcePath: final.sourceTree.Selected,
			DestPath:   final.destTree.Selected,
			FileAction: fa,
		}, nil
	case ActionUndo:
		return Result{Action: ActionUndo}, nil
	default:
		return Result{Action: ActionCancel}, nil
	}
}
