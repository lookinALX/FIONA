package types

import "time"

type LogEntry struct {
	Timestamp          time.Time
	SourcePath         string
	DestPath           string
	Status             string // "success" or "failed"
	ConflictResolution string
	Error              string
}

func NewLogEntry() LogEntry {
	return LogEntry{}
}

func NewLogEntryFromAction(action Action) LogEntry {
	return LogEntry{
		Timestamp:          time.Now(),
		SourcePath:         action.SourcePath,
		DestPath:           action.DestPath,
		Status:             "",
		ConflictResolution: "",
		Error:              "",
	}
}
