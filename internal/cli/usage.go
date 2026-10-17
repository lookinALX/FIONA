package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/lookinalx/fiona/internal/config"
	"github.com/lookinalx/fiona/internal/rules"
	"github.com/lookinalx/fiona/internal/scanner"
)

const commands = `Usage:
  fiona [FOLDER]                      sort FOLDER (default: the current one) with the default preset
  fiona sort [FLAGS] [SOURCE] [DEST]  sort SOURCE into folders inside DEST (default: SOURCE)
  fiona undo [RUN-ID] [FLAGS]         undo the last run, or the one given; again for the one before
  fiona history [--all]               list runs and whether they are undone
  fiona presets [show NAME]           list presets, or print one as JSON to copy into the config
  fiona tui [FOLDER]                  interactive mode, also started by fiona without arguments
`

// PrintUsage prints the full help. The flag lists come from the flag sets
// themselves, so they cannot go out of date.
func PrintUsage(w io.Writer, version string) {
	fmt.Fprintf(w, "FIONA %s — sorts files into folders.\n\n", version)
	fmt.Fprint(w, commands)

	fmt.Fprintf(w, "\nSort flags:\n%s", SortFlags(&SortOpts{}).FlagUsages())
	fmt.Fprintf(w, "\nUndo flags:\n%s", UndoFlags(&UndoOpts{}).FlagUsages())

	fmt.Fprintln(w, "\nRules come from, first found: --to, -c/-t, --preset, default_preset of the config, downloads.")
	fmt.Fprintln(w, "\nPresets:")
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(w, "  (the config cannot be read: %v)\n", err)
	} else {
		for _, name := range cfg.Names() {
			preset, _ := cfg.Preset(name)
			fmt.Fprintf(w, "  %-12s %s\n", name, preset.Description)
		}
	}
	if path, err := config.FilePath(); err == nil {
		fmt.Fprintf(w, "  Your own presets go to %s; start from `fiona presets show downloads`.\n", path)
	}

	fmt.Fprintln(w, "\nPlaceholders of --to and of presets:")
	for _, p := range rules.Placeholders {
		fmt.Fprintf(w, "  %-12s %s\n", "{"+p.Name+"}", p.Description)
	}

	fmt.Fprintln(w, "\nCategories:")
	for _, c := range scanner.Categories {
		exts := scanner.CategoryExtensions[c]
		if c == scanner.CategoryOther {
			fmt.Fprintf(w, "  %-12s everything else; files without a known extension are recognised by content\n", c)
			continue
		}
		fmt.Fprintf(w, "  %-12s %s\n", c, strings.Join(exts, " "))
	}

	fmt.Fprint(w, `
Examples:
  fiona ~/Downloads                          sort by type, after one question
  fiona -p photos ~/Pictures                 photos by year and month taken
  fiona sort ~/Downloads --to "{year}/{ext}" -n
  fiona sort ~/Downloads ~/Sorted --copy -r  copy, subfolders included
  fiona undo                                 undo the last run

Exit codes: 0 done or cancelled, 1 something failed, 2 mistake in the command or the config.
More: https://github.com/lookinalx/fiona
`)
}

func PrintShortUsage(w io.Writer) {
	fmt.Fprint(w, commands)
	fmt.Fprintln(w, "\nUse fiona --help for flags, presets and examples.")
}
