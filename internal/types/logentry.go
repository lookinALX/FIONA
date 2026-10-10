package types

import "time"

const (
	StatusOK     = "ok"
	StatusFailed = "failed"
)

// LogEntry is one operation of a run as read back from the journal.
type LogEntry struct {
	Action
	Result *LogResult // nil if the run was interrupted before the operation finished
}

type LogResult struct {
	Status  string // "ok" or "failed"
	Error   string
	Size    int64     // actual size of the target file after the operation
	ModTime time.Time // actual mtime of the target file after the operation
}
