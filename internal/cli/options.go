package cli

import (
	"FIONA/internal/messages"
	"FIONA/internal/rules"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

var maxWorkers = runtime.NumCPU() * 2

const (
	CritMIMEType  = "mimetype"
	CritExtension = "extension"
	CritYear      = "year"
	CritMonth     = "month"
	CritSize      = "size"
	CritDate      = "date"
	CritEmpty     = ""
)

const (
	ConflictReplace = "replace"
	ConflictSkip    = "skip"
	ConflictRename  = "rename"
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
	FileAction       string
	ConflictStrategy string
	Force            string
	DryRun           bool
	Workers          int
}

var validPrimaryCriteria = map[string]struct{}{
	CritMIMEType:  {},
	CritExtension: {},
	CritSize:      {},
	CritYear:      {},
	CritMonth:     {},
	CritDate:      {},
}

var validSecondaryCriteria = map[string]struct{}{
	CritEmpty:     {},
	CritMIMEType:  {},
	CritExtension: {},
	CritSize:      {},
	CritYear:      {},
	CritMonth:     {},
	CritDate:      {},
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
}

func (opts *Opts) ParseSortFlags() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get work directory: %w", err)
	}

	messages.Must(addFlag(&opts.Sort.Primary, "c", "criteria", "mimetype", "Criteria to group by firstly"))
	messages.Must(addFlag(&opts.Sort.Secondary, "t", "then", "", "Secondary grouping criteria"))
	messages.Must(addFlag(&opts.SourcePath, "s", "source", cwd, "Source directory to take files to sort"))
	messages.Must(addFlag(&opts.DestPath, "d", "dest", cwd, "Destination directory to move or copy files from source directory and sort after"))
	messages.Must(addFlag(&opts.LogPath, "", "log", cwd, "Log directory to write log files to"))
	messages.Must(addFlag(&opts.FileAction, "a", "action", "copy", "How to handle files (copy, move)"))
	messages.Must(addFlag(&opts.ConflictStrategy, "", "on-conflict", ConflictReplace, "How to handle conflicts"))
	messages.Must(addFlag(&opts.DryRun, "n", "dry-run", true, "Preview without changes"))
	messages.Must(addFlag(&opts.Force, "", "force", "N", "Force execute without conformation if 'yes'"))
	messages.Must(addFlag(&opts.Workers, "w", "workers", maxWorkers, "Number of workers for the current run"))
	flag.Parse()

	err = opts.validateSortOptions()

	switch {
	case err == nil:
		return nil
	case errors.Is(err, messages.ErrDestNotExists):
		if nestedErr := os.MkdirAll(opts.DestPath, 0755); nestedErr != nil {
			return fmt.Errorf("destination directory is not exists, cannot create one: %w", nestedErr)
		}
		return nil
	default:
		return fmt.Errorf("invalid options:\n    %w", err)
	}
}

func (opts *Opts) ParseUndoFlags() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get work directory: %w", err)
	}

	messages.Must(addFlag(&opts.LogPath, "", "log", filepath.Join(cwd, "fiona_logs.json"), "Log directory to read log file from"))
	flag.Parse()

	err = validateLogUndoFlag(opts.LogPath)
	if err != nil {
		return fmt.Errorf("invalid options:\n    %w", err)
	}
	return nil
}

func (opts *Opts) validateSortOptions() error {
	if err := validatePrimaryCriteriaFlag(opts.Sort.Primary); err != nil {
		return err
	}
	if err := validateSecondaryCriteriaFlag(opts.Sort.Secondary); err != nil {
		return err
	}
	if err := validateActionFlag(opts.FileAction); err != nil {
		return err
	}
	if err := validateSourceFlag(opts.SourcePath); err != nil {
		return err
	}
	if err := validateDestFlag(opts.DestPath); err != nil {
		return err
	}
	if err := validateOnConflictFlag(opts.ConflictStrategy); err != nil {
		return err
	}
	if err := validateWorkersFlag(&opts.Workers); err != nil {
		return err
	}
	if err := validateLogFlag(opts.LogPath); err != nil {
		return err
	}
	return nil
}

func addFlag(p any, short, long string, defVal any, usage string) error {
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
			flag.StringVar(v, short, dv, usage)
		}
		if long != "" {
			flag.StringVar(v, long, dv, usage)
		}
	case *int:
		dv, ok := defVal.(int)
		if !ok {
			return fmt.Errorf("for int flag it is expected default value bool, given %T", defVal)
		}
		if short != "" {
			flag.IntVar(v, short, dv, usage)
		}
		if long != "" {
			flag.IntVar(v, long, dv, usage)
		}
	case *bool:
		dv, ok := defVal.(bool)
		if !ok {
			return fmt.Errorf("for bool flag it is expected default value bool, given %T", defVal)
		}
		if short != "" {
			flag.BoolVar(v, short, dv, usage)
		}
		if long != "" {
			flag.BoolVar(v, long, dv, usage)
		}
	}
	return nil
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
	var i any = *workers
	value, ok := i.(int)
	if !ok {
		return messages.ErrInvalidWorkers
	}
	if value < 0 {
		return messages.ErrInvalidWorkers
	}
	if value > maxWorkers {
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

func validateDestFlag(crit string) error {
	if crit == "" {
		return messages.ErrEmptyDest
	}
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

func validateLogFlag(crit string) error {
	if crit == "" {
		return messages.ErrLogPathIsEmpty
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
