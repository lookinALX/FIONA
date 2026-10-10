package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lookinalx/fiona/internal/messages"
	"github.com/lookinalx/fiona/internal/rules"
	"github.com/lookinalx/fiona/internal/types"
)

var maxWorkers = runtime.NumCPU() * 2

const (
	CritMIMEType  = "mimetype"
	CritExtension = "extension"
	CritYear      = "year"
	CritMonth     = "month"
	CritSize      = "size"
	CritDate      = "date"
	CritML        = "ml"
	CritEmpty     = ""
)

const (
	ConflictReplace = types.OnConflictReplace
	ConflictSkip    = types.OnConflictSkip
	ConflictRename  = types.OnConflictRename
)

type SortOption struct {
	Primary   string
	Secondary string
}

type Opts struct {
	Sort             SortOption
	SourcePath       string
	DestPath         string
	LogPath          string
	MlConfigPath     string
	FileAction       string
	ConflictStrategy string
	DryRun           bool
	Yes              bool
	Recursive        bool
	Hidden           bool
	Workers          int
}

var validPrimaryCriteria = map[string]struct{}{
	CritMIMEType:  {},
	CritExtension: {},
	CritSize:      {},
	CritYear:      {},
	CritMonth:     {},
	CritDate:      {},
	CritML:        {},
}

var validSecondaryCriteria = map[string]struct{}{
	CritEmpty:     {},
	CritMIMEType:  {},
	CritExtension: {},
	CritSize:      {},
	CritYear:      {},
	CritMonth:     {},
	CritDate:      {},
	CritML:        {},
}

var validActions = map[string]struct{}{
	"move": {},
	"copy": {},
}

var validOnConflictFlag = map[string]struct{}{
	ConflictReplace: {},
	ConflictSkip:    {},
	ConflictRename:  {},
}

type ruleFactory func(isPrimary bool) rules.Rule

var ruleFactories = map[string]ruleFactory{
	CritMIMEType: func(isPrimary bool) rules.Rule {
		return &rules.ByTypeRule{IsPrimary: isPrimary}
	},
	CritDate: func(isPrimary bool) rules.Rule {
		return &rules.ByDateRule{IsPrimary: isPrimary}
	},
	CritMonth: func(isPrimary bool) rules.Rule {
		return &rules.ByMonthRule{IsPrimary: isPrimary}
	},
	CritYear: func(isPrimary bool) rules.Rule {
		return &rules.ByYearRule{IsPrimary: isPrimary}
	},
	CritSize: func(isPrimary bool) rules.Rule {
		return &rules.BySizeRule{IsPrimary: isPrimary}
	},
	CritExtension: func(isPrimary bool) rules.Rule {
		return &rules.ByExtensionRule{IsPrimary: isPrimary}
	},
	CritML: func(isPrimary bool) rules.Rule { return &rules.ByMLRule{IsPrimary: isPrimary} },
}

func (opts *Opts) ParseSortFlags(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get work directory: %w", err)
	}

	flags := newFlagSet("sort")
	messages.Must(addFlag(flags, &opts.Sort.Primary, "c", "criteria", "mimetype", "Criteria to group by firstly"))
	messages.Must(addFlag(flags, &opts.Sort.Secondary, "t", "then", "", "Secondary grouping criteria"))
	messages.Must(addFlag(flags, &opts.SourcePath, "s", "source", cwd, "Source directory to take files to sort"))
	messages.Must(addFlag(flags, &opts.DestPath, "d", "dest", cwd, "Destination directory to move or copy files from source directory and sort after"))
	messages.Must(addFlag(flags, &opts.LogPath, "", "log", "", "Directory to write the run journal to"))
	messages.Must(addFlag(flags, &opts.FileAction, "a", "action", "copy", "How to handle files (copy, move)"))
	messages.Must(addFlag(flags, &opts.ConflictStrategy, "", "on-conflict", ConflictRename, "How to handle conflicts"))
	messages.Must(addFlag(flags, &opts.DryRun, "n", "dry-run", false, "Print the plan and exit without changes"))
	messages.Must(addFlag(flags, &opts.Yes, "y", "yes", false, "Execute without confirmation"))
	messages.Must(addFlag(flags, &opts.Recursive, "r", "recursive", false, "Scan subdirectories too"))
	messages.Must(addFlag(flags, &opts.Hidden, "", "hidden", false, "Include hidden files and directories"))
	messages.Must(addFlag(flags, &opts.Workers, "w", "workers", maxWorkers, "Number of workers for the current run"))
	messages.Must(addFlag(flags, &opts.MlConfigPath, "", "ml-config", "", "Path to a ML configuration file with categories and promts"))

	if err := parseFlags(flags, args); err != nil {
		return err
	}

	// everything is validated before any side effect: dest is created only right before execution
	if err := opts.validateSortOptions(); err != nil {
		return fmt.Errorf("invalid options:\n%w", err)
	}

	if opts.SourcePath, err = ResolvePath(opts.SourcePath); err != nil {
		return fmt.Errorf("cannot resolve source directory: %w", err)
	}
	if opts.DestPath, err = ResolvePath(opts.DestPath); err != nil {
		return fmt.Errorf("cannot resolve destination directory: %w", err)
	}
	if opts.LogPath != "" {
		if opts.LogPath, err = filepath.Abs(opts.LogPath); err != nil {
			return fmt.Errorf("cannot resolve log directory: %w", err)
		}
	}

	return nil
}

func (opts *Opts) ParseUndoFlags(args []string) error {
	flags := newFlagSet("undo")
	messages.Must(addFlag(flags, &opts.LogPath, "", "log", "", "Journal file of the run to undo"))
	messages.Must(addFlag(flags, &opts.DestPath, "d", "dest", "", "Destination directory whose last run is undone"))
	messages.Must(addFlag(flags, &opts.DryRun, "n", "dry-run", false, "Print the undo preview and exit without changes"))
	messages.Must(addFlag(flags, &opts.Yes, "y", "yes", false, "Execute without confirmation"))
	messages.Must(addFlag(flags, &opts.Workers, "w", "workers", maxWorkers, "Number of workers for the current run"))

	if err := parseFlags(flags, args); err != nil {
		return err
	}

	if err := opts.validateUndoOptions(); err != nil {
		return fmt.Errorf("invalid options:\n%w", err)
	}

	var err error
	if opts.LogPath != "" {
		if opts.LogPath, err = filepath.Abs(opts.LogPath); err != nil {
			return fmt.Errorf("cannot resolve log path: %w", err)
		}
		return nil
	}

	if opts.DestPath == "" {
		if opts.DestPath, err = os.Getwd(); err != nil {
			return fmt.Errorf("cannot get work directory: %w", err)
		}
	}
	if opts.DestPath, err = ResolvePath(opts.DestPath); err != nil {
		return fmt.Errorf("cannot resolve destination directory: %w", err)
	}

	return nil
}

func (opts *Opts) validateSortOptions() error {
	errs := []error{
		validatePrimaryCriteriaFlag(opts.Sort.Primary),
		validateSecondaryCriteriaFlag(opts.Sort.Secondary),
		validateActionFlag(opts.FileAction),
		validateSourceFlag(opts.SourcePath),
		validateDestFlag(opts.DestPath),
		validateOnConflictFlag(opts.ConflictStrategy),
		validateWorkersFlag(&opts.Workers),
		validateLogFlag(opts.LogPath),
	}

	if opts.Sort.Primary == "ml" || opts.Sort.Secondary == "ml" {
		errs = append(errs, validateConfigMlFlag(opts.MlConfigPath))
	}

	return errors.Join(errs...)
}

func (opts *Opts) validateUndoOptions() error {
	errs := []error{validateWorkersFlag(&opts.Workers)}

	if opts.LogPath != "" && opts.DestPath != "" {
		errs = append(errs, messages.ErrLogAndDestExclusive)
	}
	if opts.LogPath != "" {
		errs = append(errs, validateLogUndoFlag(opts.LogPath))
	}
	if opts.DestPath != "" {
		errs = append(errs, validateUndoDestFlag(opts.DestPath))
	}

	return errors.Join(errs...)
}

func newFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return flags
}

func parseFlags(flags *flag.FlagSet, args []string) error {
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("%w: %s", messages.ErrUnexpectedArguments, strings.Join(flags.Args(), " "))
	}
	return nil
}

func addFlag(flags *flag.FlagSet, p any, short, long string, defVal any, usage string) error {
	if short == "" && long == "" {
		return fmt.Errorf("at least one of short or long flag name must be provided")
	}

	switch v := p.(type) {
	case *string:
		dv, ok := defVal.(string)
		if !ok {
			return fmt.Errorf("for string flag it is expected default value string, given %T", defVal)
		}
		if short != "" {
			flags.StringVar(v, short, dv, usage)
		}
		if long != "" {
			flags.StringVar(v, long, dv, usage)
		}
	case *int:
		dv, ok := defVal.(int)
		if !ok {
			return fmt.Errorf("for int flag it is expected default value bool, given %T", defVal)
		}
		if short != "" {
			flags.IntVar(v, short, dv, usage)
		}
		if long != "" {
			flags.IntVar(v, long, dv, usage)
		}
	case *bool:
		dv, ok := defVal.(bool)
		if !ok {
			return fmt.Errorf("for bool flag it is expected default value bool, given %T", defVal)
		}
		if short != "" {
			flags.BoolVar(v, short, dv, usage)
		}
		if long != "" {
			flags.BoolVar(v, long, dv, usage)
		}
	}
	return nil
}

// Confirm asks a yes/no question. Only "y" and "yes" in any case mean yes;
// end of input means no.
func Confirm(in io.Reader, out io.Writer, question string) bool {
	fmt.Fprintf(out, "%s [y/N] ", question)
	answer, _ := bufio.NewReader(in).ReadString('\n')
	fmt.Fprintln(out)

	answer = strings.TrimSpace(answer)
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")
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

func validatePrimaryCriteriaFlag(crit string) error {
	if _, exists := validPrimaryCriteria[crit]; !exists {
		if crit == "" {
			return messages.ErrEmptyPrimaryCriteria
		}
		return messages.ErrInvalidPrimaryCriteria
	}
	return nil
}

func validateSecondaryCriteriaFlag(crit string) error {
	if _, exists := validSecondaryCriteria[crit]; !exists {
		return messages.ErrInvalidSecondaryCriteria
	}
	return nil
}

func validateActionFlag(crit string) error {
	if _, exists := validActions[crit]; !exists {
		return messages.ErrInvalidAction
	}
	return nil
}

func validateOnConflictFlag(onConflict string) error {
	if _, exists := validOnConflictFlag[onConflict]; !exists {
		return messages.ErrInvalidOnConflict
	}
	return nil
}

func validateWorkersFlag(workers *int) error {
	if *workers < 1 {
		return messages.ErrInvalidWorkers
	}
	if *workers > maxWorkers {
		*workers = maxWorkers
		fmt.Printf("The number of workers given is greater than the number available. Workers are set to %d\n", maxWorkers)
	}
	return nil
}

func validateSourceFlag(crit string) error {
	if crit == "" {
		return messages.ErrEmptySource
	}
	info, exists, err := pathExists(crit)
	if !exists {
		return messages.ErrSourceNotExists
	}
	if !info.IsDir() {
		return messages.ErrSourceIsNotDir
	}
	if err != nil {
		return fmt.Errorf("cannot access source directory: %w", err)
	}
	return nil
}

// validateDestFlag accepts a destination that does not exist yet: it is
// created right before execution.
func validateDestFlag(crit string) error {
	if crit == "" {
		return messages.ErrEmptyDest
	}
	info, exists, err := pathExists(crit)
	if err != nil {
		return fmt.Errorf("cannot access destination directory: %w", err)
	}
	if exists && !info.IsDir() {
		return messages.ErrDestIsNotDir
	}
	return nil
}

func validateUndoDestFlag(crit string) error {
	info, exists, err := pathExists(crit)
	if !exists {
		return messages.ErrDestNotExists
	}
	if !info.IsDir() {
		return messages.ErrDestIsNotDir
	}
	if err != nil {
		return fmt.Errorf("cannot access destination directory: %w", err)
	}
	return nil
}

// validateLogFlag accepts an empty value: the journal then goes to <dest>/.fiona/runs.
func validateLogFlag(crit string) error {
	if crit == "" {
		return nil
	}
	info, exists, err := pathExists(crit)
	if !exists {
		return messages.ErrLogPathNotExists
	}
	if !info.IsDir() {
		return messages.ErrLogPathIsNotDir
	}
	if err != nil {
		return fmt.Errorf("cannot access log directory: %w", err)
	}
	return nil
}

func validateConfigMlFlag(crit string) error {
	if crit == "" {
		return nil
	}
	info, exists, err := pathExists(crit)
	if !exists {
		return messages.ErrConfigMlPathNotExists
	}
	if info.IsDir() {
		return messages.ErrConfigMlPathIsDir
	}
	if err != nil {
		return fmt.Errorf("cannot access config ml: %w", err)
	}
	if filepath.Ext(crit) != ".json" {
		return messages.ErrConfigMLIsNotJson
	}
	return nil
}

func validateLogUndoFlag(crit string) error {
	if crit == "" {
		return messages.ErrLogPathIsEmpty
	}
	info, exists, err := pathExists(crit)
	if !exists {
		return messages.ErrLogPathNotExists
	}
	if info.IsDir() {
		return messages.ErrLogUndoPathIsDir
	}
	if err != nil {
		return fmt.Errorf("cannot access undo log path: %w", err)
	}
	return nil
}

func pathExists(path string) (os.FileInfo, bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info, true, err
	}
	if os.IsNotExist(err) {
		return info, false, nil
	}
	return info, false, err
}

func (opts *Opts) ParseSortFlagsToRules() ([]rules.Rule, error) {
	var result []rules.Rule

	if rule, err := makeRule(opts.Sort.Primary, true); err != nil {
		return nil, err
	} else {
		result = append(result, rule)
	}

	if rule, err := makeRule(opts.Sort.Secondary, false); err != nil {
		return nil, err
	} else {
		result = append(result, rule)
	}

	resultRules := result[:0]
	for _, rule := range result {
		if rule != nil {
			resultRules = append(resultRules, rule)
		}
	}

	return resultRules, nil
}

func makeRule(name string, isPrimary bool) (rules.Rule, error) {
	if CritEmpty == name && !isPrimary {
		return nil, nil
	}
	factory, ok := ruleFactories[name]
	if !ok {
		return nil, messages.ErrUnknownRuleName
	}
	return factory(isPrimary), nil
}
