package cli

import (
	"errors"
	"flag"
	"fmt"
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

var validPrimaryCriteria = [5]string{"mimetype", "extention", "year", "month", "size"}
var validSecondaryCriteria = [6]string{"", "mimetype", "extention", "year", "month", "size"}
var validActions = [2]string{"move", "copy"}

func (opts *Opts) ParseFlags() error {
	addFlag(&opts.Sort.Primary, "c", "criteria", "mimetype", "Criteria to group by firstly")
	addFlag(&opts.Sort.Secondary, "t", "then", "", "Secondary grouping criteria")
	addFlag(&opts.Source, "s", "source", "", "Source directory to take files to sort")
	addFlag(&opts.Dest, "d", "dest", "", "Destination directory to move or copy files from source directory and sort after")
	addFlag(&opts.Action, "a", "action", "copy", "How to handle files (copy, move)")
	addFlag(&opts.DryRun, "n", "dry-run", false, "Preview without changes")

	err := opts.ValidateOptions()
	if err != nil {
		return fmt.Errorf("invalid options")
	}

	return nil
}

func (opts *Opts) ValidateOptions() error {
	isError := true

	for _, crit := range validPrimaryCriteria {
		if crit == opts.Sort.Primary {
			isError = false
			break
		}
	}
	if isError == false {
		isError = true
	} else {
		return errors.New("Custom error")
	}

	// TODO: do for all valid options

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
