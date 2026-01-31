package cli

import (
	"FIONA/internal/rules"
	"errors"
	"flag"
	"fmt"
	"os"
)

const (
	CritMIMEType  = "mimetype"
	CritExtension = "extension"
	CritYear      = "year"
	CritMonth     = "month"
	CritSize      = "size"
	CritDate      = "date"
	CritEmpty     = ""
)

type SortOption struct {
	Primary   string
	Secondary string
}

type Opts struct {
	Sort   SortOption
	Source string
	Dest   string
	Action string
	DryRun bool
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

func (opts *Opts) ParseFlags() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get work directory: %w", err)
	}

	Must(addFlag(&opts.Sort.Primary, "c", "criteria", "mimetype", "Criteria to group by firstly"))
	Must(addFlag(&opts.Sort.Secondary, "t", "then", "", "Secondary grouping criteria"))
	Must(addFlag(&opts.Source, "s", "source", cwd, "Source directory to take files to sort"))
	Must(addFlag(&opts.Dest, "d", "dest", cwd, "Destination directory to move or copy files from source directory and sort after"))
	Must(addFlag(&opts.Action, "a", "action", "copy", "How to handle files (copy, move)"))
	Must(addFlag(&opts.DryRun, "n", "dry-run", false, "Preview without changes"))

	flag.Parse()

	err = opts.validateOptions()

	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrDestNotExists):
		if nestedErr := os.MkdirAll(opts.Dest, 0755); nestedErr != nil {
			return fmt.Errorf("destination directory is not exists, cannot create one: %w", nestedErr)
		}
		return nil
	default:
		return fmt.Errorf("invalid options:\n    %w", err)
	}
}

func (opts *Opts) validateOptions() error {
	err := validatePrimaryCriteriaFlag(opts.Sort.Primary)
	if err != nil {
		return err
	}
	err = validateSecondaryCriteriaFlag(opts.Sort.Secondary)
	if err != nil {
		return err
	}
	err = validateActionFlag(opts.Action)
	if err != nil {
		return err
	}
	err = validateSourceFlag(opts.Source)
	if err != nil {
		return err
	}
	err = validateDestFlag(opts.Dest)
	if err != nil {
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
			return ErrEmptyPrimaryCriteria
		}
		return ErrInvalidPrimaryCriteria
	}
	return nil
}

func validateSecondaryCriteriaFlag(crit string) error {
	if _, exists := validSecondaryCriteria[crit]; !exists {
		return ErrInvalidSecondaryCriteria
	}
	return nil
}

func validateActionFlag(crit string) error {
	if _, exists := validActions[crit]; !exists {
		return ErrInvalidAction
	}
	return nil
}

func validateSourceFlag(crit string) error {
	if crit == "" {
		return ErrEmptySource
	}
	info, exists, err := pathExists(crit)
	if !exists {
		return ErrSourceNotExists
	}
	if !info.IsDir() {
		return ErrSourceIsNotDir
	}
	if err != nil {
		return fmt.Errorf("cannot access source directory: %w", err)
	}
	return nil
}

func validateDestFlag(crit string) error {
	if crit == "" {
		return ErrEmptyDest
	}
	info, exists, err := pathExists(crit)
	if !exists {
		return ErrDestNotExists
	}
	if !info.IsDir() {
		return ErrDestIsNotDir
	}
	if err != nil {
		return fmt.Errorf("cannot access destination directory: %w", err)
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

func (opts *Opts) PrintOptions() {
	fmt.Println("Parsed flags:")
	fmt.Println("   Primary sorting option: ", opts.Sort.Primary)
	fmt.Println("   Secondary sorting option: ", opts.Sort.Secondary)
	fmt.Println("   Action with files option: ", opts.Action)
	fmt.Println("   Source directory path: ", opts.Source)
	fmt.Println("   Destination directory path: ", opts.Dest)
	fmt.Println("   Is Dry Run: ", opts.DryRun)
}

func (opts *Opts) ParseToRules() ([]rules.Rule, error) {
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
		return nil, ErrUnknownRuleName
	}
	return factory(isPrimary), nil
}
