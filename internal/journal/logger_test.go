package journal

import (
	"FIONA/internal/sorter"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJournal_AppendLogEntry(t *testing.T) {
	tests := []struct {
		name        string
		input       LogEntry
		wantLogsLen int
	}{
		{"empty", LogEntry{}, 1},
		{"normal", LogEntry{Timestamp: time.Now()}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jrn := NewJournal()
			jrn.AppendLogEntry(tt.input)
			if len(jrn.Logs) != tt.wantLogsLen {
				t.Errorf("Journal.AppendLogEntry() len = %v, want %v", jrn.Logs, tt.wantLogsLen)
			}
		})
	}
}

func TestAppendLogEntryFromAction(t *testing.T) {
	jrn := NewJournal()

	action := sorter.Action{
		SourcePath: "/source/photo.jpg",
		DestPath:   "/dest/photo.jpg",
	}

	jrn.AppendLogEntryFromAction(action, "move", "success", "")

	if len(jrn.Logs) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(jrn.Logs))
	}

	entry := jrn.Logs[0]

	if entry.FileAction != "move" {
		t.Errorf("expected FileAction 'move', got '%s'", entry.FileAction)
	}

	if entry.SourcePath != action.SourcePath {
		t.Errorf("expected SourcePath %s, got %s", action.SourcePath, entry.SourcePath)
	}

	if entry.DestPath != action.DestPath {
		t.Errorf("expected DestPath %s, got %s", action.DestPath, entry.DestPath)
	}

	if entry.Status != "success" {
		t.Errorf("expected Status 'success', got '%s'", entry.Status)
	}
}

func TestSaveAsJson(t *testing.T) {
	tmpDir := t.TempDir()
	journalPath := filepath.Join(tmpDir, "journal.json")

	jrn := NewJournal()
	jrn.AppendLogEntry(LogEntry{
		Timestamp:  time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC),
		FileAction: "copy",
		SourcePath: "/source/file.txt",
		DestPath:   "/dest/file.txt",
		Status:     "success",
		Error:      "",
	})

	err := jrn.SaveAsJson(journalPath)
	if err != nil {
		t.Fatalf("failed to save journal: %v", err)
	}

	if _, err := os.Stat(journalPath); os.IsNotExist(err) {
		t.Fatal("journal file was not created")
	}

	data, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("failed to read journal file: %v", err)
	}

	if len(data) == 0 {
		t.Error("journal file is empty")
	}
}

func TestLoadFromJson(t *testing.T) {
	tmpDir := t.TempDir()
	journalPath := filepath.Join(tmpDir, "journal.json")

	// Создаём и сохраняем журнал
	original := NewJournal()
	original.AppendLogEntry(LogEntry{
		Timestamp:  time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC),
		FileAction: "move",
		SourcePath: "/source/photo.jpg",
		DestPath:   "/dest/photo.jpg",
		Status:     "success",
		Error:      "",
	})

	err := original.SaveAsJson(journalPath)
	if err != nil {
		t.Fatalf("failed to save journal: %v", err)
	}

	// Загружаем журнал
	loaded := NewJournal()
	err = loaded.LoadFromJson(journalPath)
	if err != nil {
		t.Fatalf("failed to load journal: %v", err)
	}

	// Проверяем что данные совпадают
	if len(loaded.Logs) != len(original.Logs) {
		t.Fatalf("expected %d entries, got %d", len(original.Logs), len(loaded.Logs))
	}

	if loaded.Logs[0].FileAction != original.Logs[0].FileAction {
		t.Errorf("expected FileAction %s, got %s", original.Logs[0].FileAction, loaded.Logs[0].FileAction)
	}

	if loaded.Logs[0].SourcePath != original.Logs[0].SourcePath {
		t.Errorf("expected SourcePath %s, got %s", original.Logs[0].SourcePath, loaded.Logs[0].SourcePath)
	}
}

func TestLoadFromJsonNonExistent(t *testing.T) {
	jrn := NewJournal()

	err := jrn.LoadFromJson("/path/that/does/not/exist.json")
	if err == nil {
		t.Error("expected error when loading non-existent file")
	}
}

func TestSaveAndLoadMultipleEntries(t *testing.T) {
	tmpDir := t.TempDir()
	journalPath := filepath.Join(tmpDir, "journal.json")

	original := NewJournal()

	entries := []LogEntry{
		{
			Timestamp:  time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC),
			FileAction: "copy",
			SourcePath: "/source/file1.txt",
			DestPath:   "/dest/file1.txt",
			Status:     "success",
			Error:      "",
		},
		{
			Timestamp:  time.Date(2026, 2, 18, 12, 1, 0, 0, time.UTC),
			FileAction: "move",
			SourcePath: "/source/file2.txt",
			DestPath:   "/dest/file2.txt",
			Status:     "failed",
			Error:      "permission denied",
		},
		{
			Timestamp:  time.Date(2026, 2, 18, 12, 2, 0, 0, time.UTC),
			FileAction: "copy",
			SourcePath: "/source/file3.txt",
			DestPath:   "/dest/file3.txt",
			Status:     "success",
			Error:      "",
		},
	}

	for _, entry := range entries {
		original.AppendLogEntry(entry)
	}

	err := original.SaveAsJson(journalPath)
	if err != nil {
		t.Fatalf("failed to save journal: %v", err)
	}

	loaded := NewJournal()
	err = loaded.LoadFromJson(journalPath)
	if err != nil {
		t.Fatalf("failed to load journal: %v", err)
	}

	if len(loaded.Logs) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(loaded.Logs))
	}

	for i, entry := range entries {
		if loaded.Logs[i].FileAction != entry.FileAction {
			t.Errorf("entry %d: expected FileAction %s, got %s", i, entry.FileAction, loaded.Logs[i].FileAction)
		}
		if loaded.Logs[i].Status != entry.Status {
			t.Errorf("entry %d: expected Status %s, got %s", i, entry.Status, loaded.Logs[i].Status)
		}
		if loaded.Logs[i].Error != entry.Error {
			t.Errorf("entry %d: expected Error %s, got %s", i, entry.Error, loaded.Logs[i].Error)
		}
	}
}

func TestConcurrentAppend(t *testing.T) {
	jrn := NewJournal()

	const numGoroutines = 10
	const entriesPerGoroutine = 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < entriesPerGoroutine; j++ {
				entry := LogEntry{
					Timestamp:  time.Now(),
					FileAction: "copy",
					SourcePath: "/source/file.txt",
					DestPath:   "/dest/file.txt",
					Status:     "success",
					Error:      "",
				}
				jrn.AppendLogEntry(entry)
			}
			done <- true
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	expectedCount := numGoroutines * entriesPerGoroutine
	if len(jrn.Logs) != expectedCount {
		t.Errorf("expected %d entries, got %d", expectedCount, len(jrn.Logs))
	}
}
