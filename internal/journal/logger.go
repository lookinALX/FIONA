package journal

import (
	"FIONA/internal/types"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Journal struct {
	mu         sync.Mutex
	FileAction string // "move" or "copy"
	Logs       []types.LogEntry
}

func NewJournal(fileAction string) Journal {
	return Journal{FileAction: fileAction}
}

func (jrn *Journal) AppendLogEntryFromAction(action types.Action, status, actionError string) {
	jrn.mu.Lock()
	defer jrn.mu.Unlock()
	LogEntry := types.LogEntry{
		Timestamp:  time.Now(),
		SourcePath: action.SourcePath,
		DestPath:   action.DestPath,
		Status:     status,
		Error:      actionError,
	}
	jrn.Logs = append(jrn.Logs, LogEntry)
}

func (jrn *Journal) AppendLogEntry(entry types.LogEntry) {
	jrn.mu.Lock()
	defer jrn.mu.Unlock()
	jrn.Logs = append(jrn.Logs, entry)
}

func (jrn *Journal) SaveAsJson(path string) error {
	jrn.mu.Lock()
	defer jrn.mu.Unlock()

	data, err := json.MarshalIndent(jrn.Logs, "", "  ")
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
