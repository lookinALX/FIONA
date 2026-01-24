package cli

import (
	"flag"
	"fmt"
	"os"
)

const (
	CritMIMEType  = "mimetype"
	CritExtention = "extension"
	CritYear      = "year"
	CritMonth     = "month"
	CritSize      = "size"
	CritModDate   = "moddate"
	CritCrDate    = "createdate"
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
	CritExtention: {},
	CritSize:      {},
	CritYear:      {},
	CritMonth:     {},
	CritModDate:   {},
	CritCrDate:    {},
}

var validSecondaryCriteria = map[string]struct{}{
	CritEmpty:     {},
	CritMIMEType:  {},
	CritExtention: {},
	CritSize:      {},
	CritYear:      {},
	CritMonth:     {},
	CritModDate:   {},
	CritCrDate:    {},
}

var validActions = map[string]struct{}{
	"move": {},
	"copy": {},
}

func (opts *Opts) ParseFlags() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get work directory: %w", err)
	}

	addFlag(&opts.Sort.Primary, "c", "criteria", "mimetype", "Criteria to group by firstly")
	addFlag(&opts.Sort.Secondary, "t", "then", "", "Secondary grouping criteria")
	addFlag(&opts.Source, "s", "source", cwd, "Source directory to take files to sort")
	addFlag(&opts.Dest, "d", "dest", cwd, "Destination directory to move or copy files from source directory and sort after")
	addFlag(&opts.Action, "a", "action", "copy", "How to handle files (copy, move)")
	addFlag(&opts.DryRun, "n", "dry-run", false, "Preview without changes")

	flag.Parse()

	err = opts.validateOptions()
	if err != nil {
		return fmt.Errorf("invalid options:\n    %w", err)
	}
	return nil
}

func (opts *Opts) validateOptions() error {
	var err error = nil

	err = validatePrimaryCriteriaFlag(opts.Sort.Primary)
	err = validateSecondaryCriteriaFlag(opts.Sort.Secondary)
	err = validateActionFlag(opts.Action)
	err = validateSourceFlag(opts.Source)

	return err
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
	exists, err := pathExists(crit)
	if !exists {
		return ErrSourceNotExists
	}
	if err != nil {
		return fmt.Errorf("cannot access source directory: %w", err)
	}
	return nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, err
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
