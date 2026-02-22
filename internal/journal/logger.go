package journal

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/lookinalx/fiona/internal/types"
)

type journalData struct {
	FileAction string                 `json:"file_action"`
	Logs       map[int]types.LogEntry `json:"logs"`
}

type Journal struct {
	mu         sync.Mutex
	path       string
	FileAction string                 // "move" or "copy"
	Logs       map[int]types.LogEntry // key = operation ID, value = log entry
	nextID     int                    // counter for auto-incrementing IDs
}

func NewJournal(path, fileAction string) Journal {
	return Journal{
		path:       path,
		FileAction: fileAction,
		Logs:       make(map[int]types.LogEntry),
		nextID:     0,
	}
}

func (jrn *Journal) AppendLogEntryFromAction(action types.Action, status, conflictResolution, actionError string) int {
	jrn.mu.Lock()
	defer jrn.mu.Unlock()

	entry := types.LogEntry{
		Timestamp:          time.Now(),
		SourcePath:         action.SourcePath,
		DestPath:           action.DestPath,
		Status:             status,
		ConflictResolution: conflictResolution,
		Error:              actionError,
	}

	id := jrn.nextID
	jrn.Logs[id] = entry
	jrn.nextID++

	return id
}

func (jrn *Journal) AppendLogEntry(entry types.LogEntry) int {
	jrn.mu.Lock()
	defer jrn.mu.Unlock()

	id := jrn.nextID
	jrn.Logs[id] = entry
	jrn.nextID++

	return id
}

func (jrn *Journal) GetEntry(id int) (types.LogEntry, bool) {
	jrn.mu.Lock()
	defer jrn.mu.Unlock()

	entry, exists := jrn.Logs[id]
	return entry, exists
}

func (jrn *Journal) SaveAsJson() error {
	jrn.mu.Lock()
	defer jrn.mu.Unlock()

	wrapper := journalData{
		FileAction: jrn.FileAction,
		Logs:       jrn.Logs,
	}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal journal: %w", err)
	}

	err = os.WriteFile(jrn.path, data, 0644)
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

	jrn.mu.Lock()
	defer jrn.mu.Unlock()

	var wrapper journalData
	err = json.Unmarshal(data, &wrapper)
	if err != nil {
		return fmt.Errorf("failed to unmarshal journal: %w", err)
	}

	jrn.FileAction = wrapper.FileAction
	jrn.Logs = wrapper.Logs

	maxID := -1
	for id := range jrn.Logs {
		if id > maxID {
			maxID = id
		}
	}
	jrn.nextID = maxID + 1

	return nil
}

func (jrn *Journal) Count() int {
	jrn.mu.Lock()
	defer jrn.mu.Unlock()
	return len(jrn.Logs)
}
