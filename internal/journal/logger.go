package journal

import (
	"FIONA/internal/sorter"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Journal struct {
	mu   sync.Mutex
	Logs []LogEntry
}

type LogEntry struct {
	Timestamp  time.Time
	FileAction string // "move" or "copy"
	SourcePath string
	DestPath   string
	Status     string // "success" or "failed"
	Error      string
}

func NewJournal() Journal {
	return Journal{}
}

func (jrn *Journal) AppendLogEntryFromAction(action sorter.Action, fileAction, status, actionError string) {
	jrn.mu.Lock()
	defer jrn.mu.Unlock()
	LogEntry := LogEntry{
		Timestamp:  time.Now(),
		FileAction: fileAction,
		SourcePath: action.SourcePath,
		DestPath:   action.DestPath,
		Status:     status,
		Error:      actionError,
	}
	jrn.Logs = append(jrn.Logs, LogEntry)
}

func (jrn *Journal) AppendLogEntry(entry LogEntry) {
	jrn.mu.Lock()
	defer jrn.mu.Unlock()
	jrn.Logs = append(jrn.Logs, entry)
}

func (jrn *Journal) SaveAsJson(path string) error {
	jrn.mu.Lock()
	defer jrn.mu.Unlock()

	data, err := json.MarshalIndent(jrn.Logs, "", "  ") // ← изменить тут
	if err != nil {
		return fmt.Errorf("failed to marshal journal: %w", err)
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write journal file: %w", err)
	}

	return nil
}

func (jrn *Journal) LoadFromJson(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read journal file: %w", err)
	}

	err = json.Unmarshal(data, &jrn.Logs)
	if err != nil {
		return fmt.Errorf("failed to unmarshal journal: %w", err)
	}

	return nil
}
