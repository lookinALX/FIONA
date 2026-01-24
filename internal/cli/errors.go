package cli

import "errors"

var (
	ErrInvalidPrimaryCriteria   = errors.New("invalid primary sort criteria")
	ErrInvalidSecondaryCriteria = errors.New("invalid secondary sort criteria")
	ErrInvalidAction            = errors.New("invalid action, must be 'copy' or 'move'")
	ErrEmptySource              = errors.New("source directory is required")
	ErrEmptyDest                = errors.New("destination directory is required")
	ErrSourceNotExists          = errors.New("source path doesn't exists")
	ErrDestNotExists            = errors.New("destination path doesn't exists")
)
