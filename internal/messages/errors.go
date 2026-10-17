package messages

import "errors"

var (
	ErrInvalidCriteria       = errors.New("unknown criterion")
	ErrThenWithoutCriteria   = errors.New("--then needs --criteria")
	ErrInvalidAction         = errors.New("invalid action, must be 'copy' or 'move'")
	ErrCopyAndAction         = errors.New("--copy cannot be combined with --action move")
	ErrSourceTwice           = errors.New("the source folder is given twice: as an argument and with --source")
	ErrDestTwice             = errors.New("the destination folder is given twice: as an argument and with --dest")
	ErrSourceNotExists       = errors.New("source folder doesn't exist")
	ErrDestNotExists         = errors.New("destination folder doesn't exist")
	ErrSourceIsNotDir        = errors.New("source is not a folder")
	ErrDestIsNotDir          = errors.New("destination is not a folder")
	ErrLogPathNotExists      = errors.New("log path doesn't exist")
	ErrLogAndDestExclusive   = errors.New("use only one of RUN-ID, --log and --dest")
	ErrUnexpectedArguments   = errors.New("unexpected arguments")
	ErrLogPathIsNotDir       = errors.New("log path is not a folder")
	ErrLogUndoPathIsDir      = errors.New("--log must be a journal file, not a folder")
	ErrInvalidOnConflict     = errors.New("invalid on-conflict, valid on-conflict values are 'replace', 'skip' or 'rename'")
	ErrInvalidWorkers        = errors.New("invalid number of workers, must be 1 or more")
	ErrConfigMlPathNotExists = errors.New("ML config doesn't exist")
	ErrConfigMlPathIsDir     = errors.New("ML config is a folder")
	ErrConfigMLIsNotJson     = errors.New("ML config must be a .json file")
)
