package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lookinalx/fiona/internal/cli"
	"github.com/lookinalx/fiona/internal/config"
	"github.com/lookinalx/fiona/internal/report"
)

func (e env) runPresets(args []string) int {
	cfg, err := config.Load()
	if err != nil {
		return e.fail(err)
	}

	switch {
	case len(args) == 0:
		return e.listPresets(cfg)
	case len(args) == 2 && args[0] == "show":
		return e.showPreset(cfg, args[1])
	default:
		return e.fail(&cli.UsageError{Err: fmt.Errorf("usage: fiona presets [show NAME]")})
	}
}

func (e env) listPresets(cfg *config.Config) int {
	names := cfg.Names()
	nameW, descW := 0, 0
	for _, name := range names {
		preset, _ := cfg.Preset(name)
		nameW = max(nameW, len(name))
		descW = max(descW, lipgloss.Width(preset.Description))
	}

	for _, name := range names {
		preset, _ := cfg.Preset(name)

		var notes []string
		if name == cfg.DefaultPresetName() {
			notes = append(notes, "default")
		}
		if origin := cfg.Origin(name); origin != "built-in" {
			notes = append(notes, origin)
		}

		line := fmt.Sprintf("  %-*s  %s", nameW, name, preset.Description)
		if len(notes) > 0 {
			line += strings.Repeat(" ", descW-lipgloss.Width(preset.Description)) + "  " + e.styles.Dim.Render(strings.Join(notes, ", "))
		}
		e.println(line)
	}

	path, err := config.FilePath()
	if err != nil {
		return e.fail(err)
	}
	state := ""
	if _, err := os.Stat(path); err != nil {
		state = " (not created yet)"
	}
	e.println("", "Config: "+report.Path(path)+state)
	e.println("To make your own preset, start from one: fiona presets show downloads")
	return ExitSuccess
}

// showPreset prints a preset as a piece of config, ready to be pasted and edited.
func (e env) showPreset(cfg *config.Config, name string) int {
	preset, err := cfg.Preset(name)
	if err != nil {
		return e.fail(&cli.UsageError{Err: err})
	}

	snippet := map[string]map[string]config.Preset{"presets": {name: preset}}
	data, err := json.MarshalIndent(snippet, "", "  ")
	if err != nil {
		return e.fail(err)
	}
	fmt.Fprintln(e.stdout, string(data))
	return ExitSuccess
}
