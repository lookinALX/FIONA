package journal

import (
	"FIONA/internal/types"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJournal_AppendLogEntry(t *testing.T) {
	tests := []struct {
		name        string
		input       types.LogEntry
		wantLogsLen int
	}{
		{"empty", types.LogEntry{}, 1},
		{"normal", types.LogEntry{Timestamp: time.Now()}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jrn := NewJournal("", "move")
			id := jrn.AppendLogEntry(tt.input)

			if id != 0 {
				t.Errorf("first entry should have id 0, got %d", id)
			}

			if jrn.Count() != tt.wantLogsLen {
				t.Errorf("Journal.AppendLogEntry() count = %v, want %v", jrn.Count(), tt.wantLogsLen)
			}
		})
	}
}

func TestAppendLogEntryFromAction(t *testing.T) {
	jrn := NewJournal("", "move")

	action := types.Action{
		SourcePath: "/source/photo.jpg",
		DestPath:   "/dest/photo.jpg",
	}

	id := jrn.AppendLogEntryFromAction(action, "success", "", "")

	if jrn.Count() != 1 {
		t.Fatalf("expected 1 entry, got %d", jrn.Count())
	}

	entry, exists := jrn.GetEntry(id)
	if !exists {
		t.Fatalf("entry with id %d not found", id)
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

func TestMultipleAppends(t *testing.T) {
	jrn := NewJournal("", "copy")

	id1 := jrn.AppendLogEntry(types.LogEntry{SourcePath: "/file1.txt"})
	id2 := jrn.AppendLogEntry(types.LogEntry{SourcePath: "/file2.txt"})
	id3 := jrn.AppendLogEntry(types.LogEntry{SourcePath: "/file3.txt"})

	if id1 != 0 || id2 != 1 || id3 != 2 {
		t.Errorf("expected sequential IDs 0,1,2 got %d,%d,%d", id1, id2, id3)
	}

	if jrn.Count() != 3 {
		t.Errorf("expected 3 entries, got %d", jrn.Count())
	}

	// Check that we can retrieve by ID
	entry1, exists := jrn.GetEntry(0)
	if !exists || entry1.SourcePath != "/file1.txt" {
		t.Error("failed to retrieve entry with id 0")
	}

	entry2, exists := jrn.GetEntry(1)
	if !exists || entry2.SourcePath != "/file2.txt" {
		t.Error("failed to retrieve entry with id 1")
	}
}

func TestGetEntry_NonExistent(t *testing.T) {
	jrn := NewJournal("", "move")

	_, exists := jrn.GetEntry(999)
	if exists {
		t.Error("expected GetEntry to return false for non-existent ID")
	}
}

func TestSaveAsJson(t *testing.T) {
	tmpDir := t.TempDir()
	journalPath := filepath.Join(tmpDir, "journal.json")

	jrn := NewJournal(journalPath, "move")
	jrn.AppendLogEntry(types.LogEntry{
		Timestamp:  time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC),
		SourcePath: "/source/file.txt",
		DestPath:   "/dest/file.txt",
		Status:     "success",
		Error:      "",
	})

	err := jrn.SaveAsJson()
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

	// Create and save journal
	original := NewJournal(journalPath, "move")
	id := original.AppendLogEntry(types.LogEntry{
		Timestamp:  time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC),
		SourcePath: "/source/photo.jpg",
		DestPath:   "/dest/photo.jpg",
		Status:     "success",
		Error:      "",
	})

	err := original.SaveAsJson()
	if err != nil {
		t.Fatalf("failed to save journal: %v", err)
	}

	// Load journal
	loaded := NewJournal(journalPath, "move")
	err = loaded.LoadFromJson(journalPath)
	if err != nil {
		t.Fatalf("failed to load journal: %v", err)
	}

	// Check data matches
	if loaded.Count() != original.Count() {
		t.Fatalf("expected %d entries, got %d", original.Count(), loaded.Count())
	}

	loadedEntry, exists := loaded.GetEntry(id)
	if !exists {
		t.Fatalf("expected entry with id %d to exist after loading", id)
	}

	originalEntry, _ := original.GetEntry(id)
	if loadedEntry.SourcePath != originalEntry.SourcePath {
		t.Errorf("expected SourcePath %s, got %s", originalEntry.SourcePath, loadedEntry.SourcePath)
	}
}

func TestLoadFromJsonNonExistent(t *testing.T) {
	jrn := NewJournal("", "move")

	err := jrn.LoadFromJson("/path/that/does/not/exist.json")
	if err == nil {
		t.Error("expected error when loading non-existent file")
	}
}

func TestSaveAndLoadMultipleEntries(t *testing.T) {
	tmpDir := t.TempDir()
	journalPath := filepath.Join(tmpDir, "journal.json")

	original := NewJournal(journalPath, "move")

	entries := []types.LogEntry{
		{
			Timestamp:  time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC),
			SourcePath: "/source/file1.txt",
			DestPath:   "/dest/file1.txt",
			Status:     "success",
			Error:      "",
		},
		{
			Timestamp:  time.Date(2026, 2, 18, 12, 1, 0, 0, time.UTC),
			SourcePath: "/source/file2.txt",
			DestPath:   "/dest/file2.txt",
			Status:     "failed",
			Error:      "permission denied",
		},
		{
			Timestamp:  time.Date(2026, 2, 18, 12, 2, 0, 0, time.UTC),
			SourcePath: "/source/file3.txt",
			DestPath:   "/dest/file3.txt",
			Status:     "success",
			Error:      "",
		},
	}

	ids := make([]int, len(entries))
	for i, entry := range entries {
		ids[i] = original.AppendLogEntry(entry)
	}

	err := original.SaveAsJson()
	if err != nil {
		t.Fatalf("failed to save journal: %v", err)
	}

	loaded := NewJournal("", "move")
	err = loaded.LoadFromJson(journalPath)
	if err != nil {
		t.Fatalf("failed to load journal: %v", err)
	}

	if loaded.Count() != 3 {
		t.Fatalf("expected 3 entries, got %d", loaded.Count())
	}

	// Verify each entry by ID
	for i, id := range ids {
		loadedEntry, exists := loaded.GetEntry(id)
		if !exists {
			t.Fatalf("entry with id %d not found after loading", id)
		}

		if loadedEntry.Status != entries[i].Status {
			t.Errorf("entry %d: expected Status %s, got %s", id, entries[i].Status, loadedEntry.Status)
		}
		if loadedEntry.Error != entries[i].Error {
			t.Errorf("entry %d: expected Error %s, got %s", id, entries[i].Error, loadedEntry.Error)
		}
	}
}

func TestConcurrentAppend(t *testing.T) {
	jrn := NewJournal("", "move")

	const numGoroutines = 10
	const entriesPerGoroutine = 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < entriesPerGoroutine; j++ {
				entry := types.LogEntry{
					Timestamp:  time.Now(),
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
	if jrn.Count() != expectedCount {
		t.Errorf("expected %d entries, got %d", expectedCount, jrn.Count())
	}
}

func TestNextIDPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	journalPath := filepath.Join(tmpDir, "journal.json")

	// Create journal with 3 entries (IDs: 0, 1, 2)
	original := NewJournal(journalPath, "move")
	original.AppendLogEntry(types.LogEntry{SourcePath: "/file1.txt"})
	original.AppendLogEntry(types.LogEntry{SourcePath: "/file2.txt"})
	original.AppendLogEntry(types.LogEntry{SourcePath: "/file3.txt"})

	err := original.SaveAsJson()
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Load and append new entry
	loaded := NewJournal("", "move")
	err = loaded.LoadFromJson(journalPath)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	// Next entry should have ID 3
	newID := loaded.AppendLogEntry(types.LogEntry{SourcePath: "/file4.txt"})
	if newID != 3 {
		t.Errorf("expected new entry to have ID 3, got %d", newID)
	}

	if loaded.Count() != 4 {
		t.Errorf("expected 4 entries after append, got %d", loaded.Count())
	}
}

func TestCount(t *testing.T) {
	jrn := NewJournal("", "copy")

	if jrn.Count() != 0 {
		t.Errorf("new journal should have count 0, got %d", jrn.Count())
	}

	jrn.AppendLogEntry(types.LogEntry{})
	if jrn.Count() != 1 {
		t.Errorf("after one append, count should be 1, got %d", jrn.Count())
	}

	jrn.AppendLogEntry(types.LogEntry{})
	jrn.AppendLogEntry(types.LogEntry{})
	if jrn.Count() != 3 {
		t.Errorf("after three appends, count should be 3, got %d", jrn.Count())
	}
}
