package sorter

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/lookinALX/FIONA/internal/types"
)

func TestNewReverter(t *testing.T) {
	plan := &UndoPlan{
		FileAction:  "copy",
		UndoActions: []types.UndoAction{},
	}
	workers := 4

	reverter := NewReverter(plan, workers)

	if reverter.plan != plan {
		t.Error("plan not set correctly")
	}

	if reverter.workers != workers {
		t.Errorf("expected workers %d, got %d", workers, reverter.workers)
	}
}

func TestRemoveEmptyDirs(t *testing.T) {
	tmpDir := t.TempDir()

	deepDir := filepath.Join(tmpDir, "level1", "level2", "level3")
	err := os.MkdirAll(deepDir, 0755)
	if err != nil {
		t.Fatalf("failed to create test directories: %v", err)
	}

	err = removeEmptyDirs(deepDir)
	if err != nil {
		t.Fatalf("removeEmptyDirs failed: %v", err)
	}

	if _, err := os.Stat(deepDir); !os.IsNotExist(err) {
		t.Error("deepDir should be removed")
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "level1", "level2")); !os.IsNotExist(err) {
		t.Error("level2 should be removed")
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "level1")); !os.IsNotExist(err) {
		t.Error("level1 should be removed")
	}
}

func TestRemoveEmptyDirsWithFiles(t *testing.T) {
	tmpDir := t.TempDir()

	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir1", "dir2")
	err := os.MkdirAll(dir2, 0755)
	if err != nil {
		t.Fatalf("failed to create directories: %v", err)
	}

	testFile := filepath.Join(dir1, "file.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	err = removeEmptyDirs(dir2)
	if err != nil {
		t.Fatalf("removeEmptyDirs failed: %v", err)
	}

	if _, err := os.Stat(dir2); !os.IsNotExist(err) {
		t.Error("dir2 should be removed")
	}

	// dir1 contains file, should not be removed
	if _, err := os.Stat(dir1); os.IsNotExist(err) {
		t.Error("dir1 should NOT be removed (contains file)")
	}
}

func TestRunUndoCopyAction(t *testing.T) {
	tmpDir := t.TempDir()

	sourceDir := filepath.Join(tmpDir, "source")
	err := os.MkdirAll(sourceDir, 0755)
	if err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	file1 := filepath.Join(sourceDir, "file1.txt")
	file2 := filepath.Join(sourceDir, "file2.txt")

	err = os.WriteFile(file1, []byte("content1"), 0644)
	if err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}

	err = os.WriteFile(file2, []byte("content2"), 0644)
	if err != nil {
		t.Fatalf("failed to create file2: %v", err)
	}

	plan := &UndoPlan{
		FileAction: "copy",
		UndoActions: []types.UndoAction{
			{SourcePath: file1, DestPath: ""},
			{SourcePath: file2, DestPath: ""},
		},
	}

	reverter := NewReverter(plan, 2)
	reverter.RunUndo()

	if _, err := os.Stat(file1); !os.IsNotExist(err) {
		t.Error("file1 should be removed")
	}

	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Error("file2 should be removed")
	}

	if _, err := os.Stat(sourceDir); !os.IsNotExist(err) {
		t.Error("source directory should be removed (was empty)")
	}
}

func TestRunUndoMoveAction(t *testing.T) {
	tmpDir := t.TempDir()

	destDir := filepath.Join(tmpDir, "dest")
	sourceDir := filepath.Join(tmpDir, "source")

	err := os.MkdirAll(destDir, 0755)
	if err != nil {
		t.Fatalf("failed to create dest dir: %v", err)
	}

	err = os.MkdirAll(sourceDir, 0755)
	if err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	file1 := filepath.Join(destDir, "file1.txt")
	file2 := filepath.Join(destDir, "file2.txt")

	err = os.WriteFile(file1, []byte("content1"), 0644)
	if err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}

	err = os.WriteFile(file2, []byte("content2"), 0644)
	if err != nil {
		t.Fatalf("failed to create file2: %v", err)
	}

	plan := &UndoPlan{
		FileAction: "move",
		UndoActions: []types.UndoAction{
			{
				SourcePath: file1,
				DestPath:   filepath.Join(sourceDir, "file1.txt"),
			},
			{
				SourcePath: file2,
				DestPath:   filepath.Join(sourceDir, "file2.txt"),
			},
		},
	}

	reverter := NewReverter(plan, 2)
	reverter.RunUndo()

	if _, err := os.Stat(filepath.Join(sourceDir, "file1.txt")); os.IsNotExist(err) {
		t.Error("file1 should be moved back to source")
	}

	if _, err := os.Stat(filepath.Join(sourceDir, "file2.txt")); os.IsNotExist(err) {
		t.Error("file2 should be moved back to source")
	}

	if _, err := os.Stat(file1); !os.IsNotExist(err) {
		t.Error("file1 should not exist in dest")
	}

	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Error("file2 should not exist in dest")
	}
}

func TestRunUndoNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()

	plan := &UndoPlan{
		FileAction: "copy",
		UndoActions: []types.UndoAction{
			{SourcePath: filepath.Join(tmpDir, "nonexistent.txt"), DestPath: ""},
		},
	}

	reverter := NewReverter(plan, 1)
	reverter.RunUndo()
}

func TestRunUndoEmptyPlan(t *testing.T) {
	plan := &UndoPlan{
		FileAction:  "copy",
		UndoActions: []types.UndoAction{},
	}

	reverter := NewReverter(plan, 2)
	reverter.RunUndo()
}

func TestRunUndoConcurrency(t *testing.T) {
	tmpDir := t.TempDir()

	const numFiles = 50
	var undoActions []types.UndoAction

	for i := 0; i < numFiles; i++ {
		filePath := filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i))
		err := os.WriteFile(filePath, []byte("content"), 0644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		undoActions = append(undoActions, types.UndoAction{
			SourcePath: filePath,
			DestPath:   "",
		})
	}

	plan := &UndoPlan{
		FileAction:  "copy",
		UndoActions: undoActions,
	}

	reverter := NewReverter(plan, 4)
	reverter.RunUndo()

	for i := 0; i < numFiles; i++ {
		filePath := filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i))
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Errorf("file%d.txt should be removed", i)
		}
	}
}
