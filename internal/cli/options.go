package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/pflag"

	"github.com/lookinalx/fiona/internal/config"
	"github.com/lookinalx/fiona/internal/messages"
	"github.com/lookinalx/fiona/internal/rules"
	"github.com/lookinalx/fiona/internal/suggest"
	"github.com/lookinalx/fiona/internal/types"
)

var maxWorkers = runtime.NumCPU() * 2

// Criteria of -c and -t, each standing for one placeholder.
var Criteria = []struct{ Name, Placeholder string }{
	{"category", rules.PhCategory},
	{"mimetype", rules.PhCategory},
	{"extension", rules.PhExt},
	{"year", rules.PhYear},
	{"month", rules.PhMonth},
	{"day", rules.PhDay},
	{"date", rules.PhDate},
	{"size", rules.PhSize},
	{"ml", rules.PhML},
}

// criterionAliases are accepted but not listed: the placeholder names.
var criterionAliases = map[string]string{"ext": "extension"}

func criterionPlaceholder(name string) (string, bool) {
	if alias, ok := criterionAliases[name]; ok {
		name = alias
	}
	for _, c := range Criteria {
		if c.Name == name {
			return c.Placeholder, true
		}
	}
	return "", false
}

func criteriaNames() []string {
	names := make([]string, len(Criteria))
	for i, c := range Criteria {
		names[i] = c.Name
	}
	return names
}

// UsageError is a mistake in the command line or the configuration: nothing
// is touched and the exit code is 2.
type UsageError struct {
	Err error
}

func (e *UsageError) Error() string { return e.Err.Error() }
func (e *UsageError) Unwrap() error { return e.Err }

func usageErr(err error) error {
	if err == nil {
		return nil
	}
	return &UsageError{Err: err}
}

type SortOpts struct {
	Source, Dest   string
	Criteria, Then string
	Preset, To     string
	Action         string
	Copy           bool
	OnConflict     string
	DryRun         bool
	Yes            bool
	List           bool
	Recursive      bool
	Hidden         bool
	Workers        int
	LogPath        string
	MLConfigPath   string

	flags *pflag.FlagSet
}

// SortFlags defines the flags of sort; the help is generated from them too.
func SortFlags(o *SortOpts) *pflag.FlagSet {
	flags := newFlagSet("sort")
	flags.StringVarP(&o.Preset, "preset", "p", "", "preset of rules, see `fiona presets`")
	flags.StringVar(&o.To, "to", "", "path template for all files, like \"{year}/{month}\"")
	flags.StringVarP(&o.Criteria, "criteria", "c", "", "group by: "+strings.Join(criteriaNames(), ", "))
	flags.StringVarP(&o.Then, "then", "t", "", "then group inside by another criterion")
	flags.StringVarP(&o.Source, "source", "s", "", "folder to sort, same as SOURCE (default: the current folder)")
	flags.StringVarP(&o.Dest, "dest", "d", "", "where the folders go, same as DEST (default: SOURCE)")
	flags.BoolVar(&o.Copy, "copy", false, "copy files instead of moving them")
	flags.StringVarP(&o.Action, "action", "a", string(types.OpMove), "move or copy")
	flags.StringVar(&o.OnConflict, "on-conflict", types.OnConflictRename, "when the name is taken: rename, skip or replace;\nreplace moves the old file to <dest>/.fiona/trash, kept 5 days")
	flags.BoolVarP(&o.Recursive, "recursive", "r", false, "sort files in subfolders too; bundles (.app, …) and projects (.git, go.mod, …) stay whole")
	flags.BoolVar(&o.Hidden, "hidden", false, "include files and folders starting with a dot")
	flags.BoolVarP(&o.List, "list", "l", false, "list every file in the plan")
	flags.BoolVarP(&o.DryRun, "dry-run", "n", false, "show the plan and exit, nothing is changed")
	flags.BoolVarP(&o.Yes, "yes", "y", false, "do not ask for confirmation")
	flags.IntVarP(&o.Workers, "workers", "w", maxWorkers, "number of files processed at once")
	flags.StringVar(&o.LogPath, "log", "", "folder for the run journal (default: <dest>/.fiona/runs)")
	flags.StringVar(&o.MLConfigPath, "ml-config", "", "JSON file with ML categories and their prompts, for {ml}")
	return flags
}

// ParseSort reads the flags and arguments of sort and checks all of them
// before anything is touched.
func ParseSort(args []string) (*SortOpts, error) {
	o := &SortOpts{}
	o.flags = SortFlags(o)
	if err := o.flags.Parse(args); err != nil {
		return nil, usageErr(err)
	}

	positional := o.flags.Args()
	var errs []error
	switch {
	case len(positional) > 2:
		errs = append(errs, fmt.Errorf("%w: %s", messages.ErrUnexpectedArguments, strings.Join(positional[2:], " ")))
	case len(positional) == 2:
		if o.flags.Changed("dest") {
			errs = append(errs, messages.ErrDestTwice)
		}
		o.Dest = positional[1]
		fallthrough
	case len(positional) == 1:
		if o.flags.Changed("source") {
			errs = append(errs, messages.ErrSourceTwice)
		}
		o.Source = positional[0]
	}

	if o.Source == "" {
		o.Source = "."
	}
	if o.Dest == "" {
		o.Dest = o.Source
	}

	errs = append(errs,
		o.validateRules(),
		validateAction(o.Action),
		validateOnConflict(o.OnConflict),
		validateWorkers(&o.Workers),
		validateSource(o.Source),
		validateDest(o.Dest),
		validateLogDir(o.LogPath),
		validateMLConfig(o.MLConfigPath),
	)
	if o.Copy {
		if o.flags.Changed("action") && o.Action != string(types.OpCopy) {
			errs = append(errs, messages.ErrCopyAndAction)
		}
		o.Action = string(types.OpCopy)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, usageErr(err)
	}

	var err error
	if o.Source, err = ResolvePath(o.Source); err != nil {
		return nil, fmt.Errorf("cannot resolve source folder: %w", err)
	}
	if o.Dest, err = ResolvePath(o.Dest); err != nil {
		return nil, fmt.Errorf("cannot resolve destination folder: %w", err)
	}
	if o.LogPath != "" {
		if o.LogPath, err = filepath.Abs(o.LogPath); err != nil {
			return nil, fmt.Errorf("cannot resolve log folder: %w", err)
		}
	}

	return o, nil
}

func (o *SortOpts) validateRules() error {
	var errs []error
	for _, c := range []string{o.Criteria, o.Then} {
		if c == "" {
			continue
		}
		if _, ok := criterionPlaceholder(c); !ok {
			msg := fmt.Sprintf("%v %q.", messages.ErrInvalidCriteria, c)
			if guess, ok := suggest.Closest(c, criteriaNames()); ok {
				msg += fmt.Sprintf(" Did you mean %q?", guess)
			}
			errs = append(errs, errors.New(msg+" Available: "+strings.Join(criteriaNames(), ", ")))
		}
	}
	if o.Then != "" && o.Criteria == "" {
		errs = append(errs, messages.ErrThenWithoutCriteria)
	}
	return errors.Join(errs...)
}

// ApplyConfig takes the action and the conflict strategy from the
// configuration, unless they are given on the command line.
func (o *SortOpts) ApplyConfig(cfg *config.Config) {
	if cfg.Action != "" && !o.Copy && !o.flags.Changed("action") {
		o.Action = cfg.Action
	}
	if cfg.OnConflict != "" && !o.flags.Changed("on-conflict") {
		o.OnConflict = cfg.OnConflict
	}
}

// Rules picks the rules: --to, then -c/-t, then --preset, then the default
// preset of the configuration, then the built-in downloads. The description
// tells the user where they come from.
func (o *SortOpts) Rules(cfg *config.Config) (*rules.Set, string, error) {
	switch {
	case o.To != "":
		set, err := rules.Compile([]rules.Rule{{To: o.To}})
		if err != nil {
			return nil, "", usageErr(fmt.Errorf("--to: %w", unwrapRule(err)))
		}
		return set, fmt.Sprintf("with --to %q", o.To), nil

	case o.Criteria != "":
		primary, _ := criterionPlaceholder(o.Criteria)
		to, source := "{"+primary+"}", "by "+o.Criteria
		if o.Then != "" {
			secondary, _ := criterionPlaceholder(o.Then)
			to += "/{" + secondary + "}"
			source += ", then " + o.Then
		}
		set, err := rules.Compile([]rules.Rule{{To: to}})
		return set, source, usageErr(err)

	case o.Preset != "":
		set, err := cfg.Rules(o.Preset)
		return set, fmt.Sprintf("with preset %q", o.Preset), usageErr(err)

	default:
		name := cfg.DefaultPresetName()
		set, err := cfg.Rules(name)
		source := fmt.Sprintf("with preset %q", name)
		if cfg.DefaultPreset != "" {
			source += " (default from config)"
		}
		return set, source, usageErr(err)
	}
}

// UsesML reports whether the rules need the classifier at all.
func UsesML(set *rules.Set) bool {
	return set.Uses(rules.PhML)
}

// unwrapRule drops the "rule 1: " prefix: --to is a single rule.
func unwrapRule(err error) error {
	return errors.New(strings.TrimPrefix(err.Error(), "rule 1: "))
}

type UndoOpts struct {
	RunID   string
	LogPath string
	Dest    string
	DryRun  bool
	Yes     bool
	Workers int
}

func UndoFlags(o *UndoOpts) *pflag.FlagSet {
	flags := newFlagSet("undo")
	flags.StringVar(&o.LogPath, "log", "", "journal file of the run to undo (<run-id>.jsonl)")
	flags.StringVarP(&o.Dest, "dest", "d", "", "undo the last run that sorted into this folder")
	flags.BoolVarP(&o.DryRun, "dry-run", "n", false, "show what would be undone and exit")
	flags.BoolVarP(&o.Yes, "yes", "y", false, "do not ask for confirmation")
	flags.IntVarP(&o.Workers, "workers", "w", maxWorkers, "number of files processed at once")
	return flags
}

func ParseUndo(args []string) (*UndoOpts, error) {
	o := &UndoOpts{}
	flags := UndoFlags(o)
	if err := flags.Parse(args); err != nil {
		return nil, usageErr(err)
	}

	var errs []error
	switch positional := flags.Args(); {
	case len(positional) > 1:
		errs = append(errs, fmt.Errorf("%w: %s", messages.ErrUnexpectedArguments, strings.Join(positional[1:], " ")))
	case len(positional) == 1:
		o.RunID = positional[0]
	}

	given := 0
	for _, v := range []string{o.RunID, o.LogPath, o.Dest} {
		if v != "" {
			given++
		}
	}
	if given > 1 {
		errs = append(errs, messages.ErrLogAndDestExclusive)
	}
	errs = append(errs, validateWorkers(&o.Workers))
	if o.LogPath != "" {
		errs = append(errs, validateJournal(o.LogPath))
	}
	if o.Dest != "" {
		errs = append(errs, validateExistingDest(o.Dest))
	}
	if err := errors.Join(errs...); err != nil {
		return nil, usageErr(err)
	}

	var err error
	if o.LogPath != "" {
		if o.LogPath, err = filepath.Abs(o.LogPath); err != nil {
			return nil, fmt.Errorf("cannot resolve log path: %w", err)
		}
	}
	if o.Dest != "" {
		if o.Dest, err = ResolvePath(o.Dest); err != nil {
			return nil, fmt.Errorf("cannot resolve destination folder: %w", err)
		}
	}
	return o, nil
}

type HistoryOpts struct {
	All bool
}

func HistoryFlags(o *HistoryOpts) *pflag.FlagSet {
	flags := newFlagSet("history")
	flags.BoolVar(&o.All, "all", false, "show every run, not only the last 20")
	return flags
}

func ParseHistory(args []string) (*HistoryOpts, error) {
	o := &HistoryOpts{}
	flags := HistoryFlags(o)
	if err := flags.Parse(args); err != nil {
		return nil, usageErr(err)
	}
	if flags.NArg() > 0 {
		return nil, usageErr(fmt.Errorf("%w: %s", messages.ErrUnexpectedArguments, strings.Join(flags.Args(), " ")))
	}
	return o, nil
}

func newFlagSet(name string) *pflag.FlagSet {
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.SortFlags = false
	return flags
}

// ResolvePath makes path absolute and resolves symlinks in the part of it
// that exists, so that two roots can be compared.
func ResolvePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	existing, rest := abs, ""
	for {
		resolved, err := filepath.EvalSymlinks(existing)
		if err == nil {
			return filepath.Join(resolved, rest), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}

		parent := filepath.Dir(existing)
		if parent == existing {
			return abs, nil
		}
		rest = filepath.Join(filepath.Base(existing), rest)
		existing = parent
	}
}

// IsInside reports whether path lies strictly inside root. Both must be resolved.
func IsInside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && filepath.IsLocal(rel)
}

func validateAction(action string) error {
	if action != string(types.OpMove) && action != string(types.OpCopy) {
		return messages.ErrInvalidAction
	}
	return nil
}

func validateOnConflict(onConflict string) error {
	switch onConflict {
	case types.OnConflictRename, types.OnConflictSkip, types.OnConflictReplace:
		return nil
	}
	return messages.ErrInvalidOnConflict
}

func validateWorkers(workers *int) error {
	if *workers < 1 {
		return messages.ErrInvalidWorkers
	}
	if *workers > maxWorkers {
		*workers = maxWorkers
		fmt.Fprintf(os.Stderr, "fiona: only %d workers are useful here, using %d\n", maxWorkers, maxWorkers)
	}
	return nil
}

func validateSource(path string) error {
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: %s", messages.ErrSourceNotExists, path)
	case err != nil:
		return fmt.Errorf("cannot access source folder: %w", err)
	case !info.IsDir():
		return fmt.Errorf("%w: %s", messages.ErrSourceIsNotDir, path)
	}
	return nil
}

// validateDest accepts a folder that does not exist yet: it is created right before execution.
func validateDest(path string) error {
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("cannot access destination folder: %w", err)
	case !info.IsDir():
		return fmt.Errorf("%w: %s", messages.ErrDestIsNotDir, path)
	}
	return nil
}

func validateExistingDest(path string) error {
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: %s", messages.ErrDestNotExists, path)
	case err != nil:
		return fmt.Errorf("cannot access destination folder: %w", err)
	case !info.IsDir():
		return fmt.Errorf("%w: %s", messages.ErrDestIsNotDir, path)
	}
	return nil
}

// validateLogDir accepts an empty value: the journal then goes to <dest>/.fiona/runs.
func validateLogDir(path string) error {
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: %s", messages.ErrLogPathNotExists, path)
	case err != nil:
		return fmt.Errorf("cannot access log folder: %w", err)
	case !info.IsDir():
		return fmt.Errorf("%w: %s", messages.ErrLogPathIsNotDir, path)
	}
	return nil
}

func validateJournal(path string) error {
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: %s", messages.ErrLogPathNotExists, path)
	case err != nil:
		return fmt.Errorf("cannot access journal: %w", err)
	case info.IsDir():
		return messages.ErrLogUndoPathIsDir
	}
	return nil
}

func validateMLConfig(path string) error {
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: %s", messages.ErrConfigMlPathNotExists, path)
	case err != nil:
		return fmt.Errorf("cannot access ML config: %w", err)
	case info.IsDir():
		return messages.ErrConfigMlPathIsDir
	case filepath.Ext(path) != ".json":
		return messages.ErrConfigMLIsNotJson
	}
	return nil
}
