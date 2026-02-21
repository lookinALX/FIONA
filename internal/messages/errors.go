package messages

import "errors"

var (
	ErrInvalidPrimaryCriteria   = errors.New("invalid primary sort criteria")
	ErrEmptyPrimaryCriteria     = errors.New("empty primary sort criteria")
	ErrInvalidSecondaryCriteria = errors.New("invalid secondary sort criteria")
	ErrInvalidAction            = errors.New("invalid action, must be 'copy' or 'move'")
	ErrEmptySource              = errors.New("source directory is required")
	ErrEmptyDest                = errors.New("destination directory is required")
	ErrSourceNotExists          = errors.New("source path doesn't exists")
	ErrDestNotExists            = errors.New("destination path doesn't exists")
	ErrSourceIsNotDir           = errors.New("source path is not a path to a directory")
	ErrDestIsNotDir             = errors.New("destination path is not a path to a directory")
	ErrLogPathNotExists         = errors.New("log path doesn't exists")
	ErrLogPathIsEmpty           = errors.New("log path is empty")
	ErrLogPathIsNotDir          = errors.New("log path is not a directory")
	ErrLogUndoPathIsDir         = errors.New("log undo path is a directory")
	ErrUnknownRuleName          = errors.New("unknown rule name")
	ErrInvalidOnConflict        = errors.New("invalid on-conflict, valid on-conflict values are 'replace', 'skip' or 'rename'")
	ErrInvalidWorkers           = errors.New("invalud number of workers, must be positiv int")
)

func Must(err error) {
	if err != nil {
		panic(err)
	}
}
