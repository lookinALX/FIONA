// Package tui is the interactive mode: pick a folder and a preset, see the
// plan change as you go, run it, undo it.
package tui

import (
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lookinalx/fiona/internal/app"
	"github.com/lookinalx/fiona/internal/config"
)

// Options start the TUI.
type Options struct {
	Source     string // resolved folder shown first
	Config     *config.Config
	HistoryDir string
	Version    string
	Workers    int // default: twice the number of CPUs

	// NewClassifier makes the ML classifier on first use; default fiona-classifier.
	NewClassifier func() app.Classifier
}

func (o Options) withDefaults() Options {
	if o.Workers < 1 {
		o.Workers = runtime.NumCPU() * 2
	}
	if o.NewClassifier == nil {
		o.NewClassifier = func() app.Classifier { return app.NewSidecar("") }
	}
	return o
}

// Run shows the TUI until the user quits.
func Run(opts Options) error {
	final, err := tea.NewProgram(newModel(opts), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	if m, ok := final.(model); ok {
		m.close()
	}
	return err
}
