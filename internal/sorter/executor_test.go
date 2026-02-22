package sorter

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lookinalx/fiona/internal/cli"
	"github.com/lookinalx/fiona/internal/types"
)

// ─── Helper functions ────────────────────────────────────────────────────────

func createTestFile(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create parent dir: %v", err)
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readFileContent(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}
	return content
}

func assertFileContent(t *testing.T, path string, expected []byte) {
	t.Helper()
	got := readFileContent(t, path)
	if !bytes.Equal(got, expected) {
		t.Errorf("file content mismatch:\ngot:  %q\nwant: %q", got, expected)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if !fileExists(path) {
		t.Errorf("file should exist: %s", path)
	}
}

func assertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if fileExists(path) {
		t.Errorf("file should not exist: %s", path)
	}
}

func assertFileMode(t *testing.T, path string, expectedMode os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("cannot stat file: %v", err)
	}
	if info.Mode() != expectedMode {
		t.Errorf("mode mismatch: got %v, want %v", info.Mode(), expectedMode)
	}
}

// ─── generateNewName tests ───────────────────────────────────────────────────

func TestGenerateNewName_NoConflict(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "file.txt")

	// Create original file
	createTestFile(t, basePath, []byte("original"), 0644)

	got := generateNewName(basePath)
	want := filepath.Join(tmpDir, "file_1.txt")

	if got != want {
		t.Errorf("generateNewName() = %q, want %q", got, want)
	}
}

func TestGenerateNewName_MultipleConflicts(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "file.txt")

	// Create file.txt, file_1.txt, file_2.txt
	createTestFile(t, basePath, []byte("0"), 0644)
	createTestFile(t, filepath.Join(tmpDir, "file_1.txt"), []byte("1"), 0644)
	createTestFile(t, filepath.Join(tmpDir, "file_2.txt"), []byte("2"), 0644)

	got := generateNewName(basePath)
	want := filepath.Join(tmpDir, "file_3.txt")

	if got != want {
		t.Errorf("generateNewName() = %q, want %q", got, want)
	}
}

func TestGenerateNewName_NoExtension(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "Makefile")

	createTestFile(t, basePath, []byte("content"), 0644)

	got := generateNewName(basePath)
	want := filepath.Join(tmpDir, "Makefile_1")

	if got != want {
		t.Errorf("generateNewName() = %q, want %q", got, want)
	}
}

func TestGenerateNewName_MultipleDotsInName(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "my.file.name.txt")

	createTestFile(t, basePath, []byte("content"), 0644)

	got := generateNewName(basePath)
	want := filepath.Join(tmpDir, "my.file.name_1.txt")

	if got != want {
		t.Errorf("generateNewName() = %q, want %q", got, want)
	}
}

// ─── copyWithMetadata tests ──────────────────────────────────────────────────

func TestCopyWithMetadata_Content(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	content := []byte("test content with special chars: åäö\n")
	createTestFile(t, src, content, 0644)

	err := copyWithMetadata(src, dst)
	if err != nil {
		t.Fatalf("copyWithMetadata() error = %v", err)
	}

	assertFileContent(t, dst, content)
	assertFileExists(t, src) // Source should still exist
}

func TestCopyWithMetadata_Permissions(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	createTestFile(t, src, []byte("content"), 0755)

	err := copyWithMetadata(src, dst)
	if err != nil {
		t.Fatalf("copyWithMetadata() error = %v", err)
	}

	assertFileMode(t, dst, 0755)
}

func TestCopyWithMetadata_ModTime(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	createTestFile(t, src, []byte("content"), 0644)

	// Set specific mod time
	pastTime := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(src, pastTime, pastTime); err != nil {
		t.Fatalf("failed to set mod time: %v", err)
	}

	err := copyWithMetadata(src, dst)
	if err != nil {
		t.Fatalf("copyWithMetadata() error = %v", err)
	}

	srcInfo, _ := os.Stat(src)
	dstInfo, _ := os.Stat(dst)

	if !srcInfo.ModTime().Equal(dstInfo.ModTime()) {
		t.Errorf("mod time mismatch:\nsrc: %v\ndst: %v", srcInfo.ModTime(), dstInfo.ModTime())
	}
}

func TestCopyWithMetadata_SourceNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "nonexistent.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	err := copyWithMetadata(src, dst)
	if err == nil {
		t.Error("copyWithMetadata() should error for non-existent source")
	}
}

func TestCopyWithMetadata_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "large.bin")
	dst := filepath.Join(tmpDir, "large_copy.bin")

	// Create 1MB file
	largeContent := make([]byte, 1024*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	createTestFile(t, src, largeContent, 0644)

	err := copyWithMetadata(src, dst)
	if err != nil {
		t.Fatalf("copyWithMetadata() error = %v", err)
	}

	assertFileContent(t, dst, largeContent)
}

// ─── smartMoveFile tests ─────────────────────────────────────────────────────

func TestSmartMoveFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	content := []byte("content to move")
	createTestFile(t, src, content, 0644)

	err := smartMoveFile(src, dst)
	if err != nil {
		t.Fatalf("smartMoveFile() error = %v", err)
	}

	assertFileContent(t, dst, content)
	assertFileNotExists(t, src) // Source should be deleted after move
}

func TestSmartMoveFile_SourceNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "nonexistent.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	err := smartMoveFile(src, dst)
	if err == nil {
		t.Error("smartMoveFile() should error for non-existent source")
	}
}

// ─── ProcessFile tests ───────────────────────────────────────────────────────

func TestProcessFile_Copy_NoConflict(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	content := []byte("test content")
	createTestFile(t, src, content, 0644)

	ex := NewExecutor(nil,
		&cli.Opts{
			Sort:             cli.SortOption{Primary: "", Secondary: ""},
			SourcePath:       "",
			DestPath:         "",
			FileAction:       "copy",
			ConflictStrategy: cli.ConflictReplace,
			Force:            "",
			DryRun:           true,
			Workers:          10},
	)

	msg, err := ex.ProcessFile(src, dst)
	if err != nil {
		t.Fatalf("ProcessFile() error = %v", err)
	}

	if msg != "" {
		t.Errorf("ProcessFile() message = %v", msg)
	}

	assertFileContent(t, dst, content)
	assertFileExists(t, src) // Copy should preserve source
}

func TestProcessFile_Move_NoConflict(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	content := []byte("test content")
	createTestFile(t, src, content, 0644)

	ex := NewExecutor(nil,
		&cli.Opts{
			Sort:             cli.SortOption{Primary: "", Secondary: ""},
			SourcePath:       "",
			DestPath:         "",
			FileAction:       "move",
			ConflictStrategy: cli.ConflictReplace,
			Force:            "",
			DryRun:           true,
			Workers:          10},
	)

	msg, err := ex.ProcessFile(src, dst)
	if err != nil {
		t.Fatalf("ProcessFile() error = %v", err)
	}

	if msg != "" {
		t.Errorf("ProcessFile() message = %v", msg)
	}

	assertFileContent(t, dst, content)
	assertFileNotExists(t, src) // Move should delete source
}

func TestProcessFile_Copy_ConflictReplace(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	srcContent := []byte("new content")
	dstContent := []byte("old content")

	createTestFile(t, src, srcContent, 0644)
	createTestFile(t, dst, dstContent, 0644)

	ex := NewExecutor(nil,
		&cli.Opts{
			Sort:             cli.SortOption{Primary: "", Secondary: ""},
			SourcePath:       "",
			DestPath:         "",
			FileAction:       "copy",
			ConflictStrategy: cli.ConflictReplace,
			Force:            "",
			DryRun:           true,
			Workers:          10},
	)

	msg, err := ex.ProcessFile(src, dst)
	if err != nil {
		t.Fatalf("ProcessFile() error = %v", err)
	}

	if !strings.Contains(msg, "⟲ Replaced:") {
		t.Errorf("ProcessFile() message = %v", msg)
	}

	assertFileContent(t, dst, srcContent) // Should be replaced
	assertFileExists(t, src)
}

func TestProcessFile_Copy_ConflictSkip(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	srcContent := []byte("new content")
	dstContent := []byte("old content")

	createTestFile(t, src, srcContent, 0644)
	createTestFile(t, dst, dstContent, 0644)

	ex := NewExecutor(nil,
		&cli.Opts{
			Sort:             cli.SortOption{Primary: "", Secondary: ""},
			SourcePath:       "",
			DestPath:         "",
			FileAction:       "copy",
			ConflictStrategy: cli.ConflictSkip,
			Force:            "",
			DryRun:           true,
			Workers:          10},
	)

	msg, err := ex.ProcessFile(src, dst)
	if err != nil {
		t.Fatalf("ProcessFile() error = %v", err)
	}

	if !strings.Contains(msg, "⊘ Skipping (already exists):") {
		t.Errorf("ProcessFile() message = %v", msg)
	}

	assertFileContent(t, dst, dstContent) // Should remain unchanged
	assertFileExists(t, src)
}

func TestProcessFile_Copy_ConflictRename(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	srcContent := []byte("new content")
	dstContent := []byte("old content")

	createTestFile(t, src, srcContent, 0644)
	createTestFile(t, dst, dstContent, 0644)

	ex := NewExecutor(nil,
		&cli.Opts{
			Sort:             cli.SortOption{Primary: "", Secondary: ""},
			SourcePath:       "",
			DestPath:         "",
			FileAction:       "copy",
			ConflictStrategy: cli.ConflictRename,
			Force:            "",
			DryRun:           true,
			Workers:          10},
	)

	msg, err := ex.ProcessFile(src, dst)
	if err != nil {
		t.Fatalf("ProcessFile() error = %v", err)
	}

	if !strings.Contains(msg, "⚠ Conflict resolved: saving as") {
		t.Errorf("ProcessFile() message = %v", msg)
	}

	// Original dest should be unchanged
	assertFileContent(t, dst, dstContent)

	// Renamed file should exist with source content
	renamed := filepath.Join(tmpDir, "dest_1.txt")
	assertFileContent(t, renamed, srcContent)
	assertFileExists(t, src)
}

func TestProcessFile_Move_ConflictRename(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	srcContent := []byte("new content")
	dstContent := []byte("old content")

	createTestFile(t, src, srcContent, 0644)
	createTestFile(t, dst, dstContent, 0644)

	ex := NewExecutor(nil,
		&cli.Opts{
			Sort:             cli.SortOption{Primary: "", Secondary: ""},
			SourcePath:       "",
			DestPath:         "",
			FileAction:       "move",
			ConflictStrategy: cli.ConflictRename,
			Force:            "",
			DryRun:           true,
			Workers:          10},
	)

	msg, err := ex.ProcessFile(src, dst)
	if err != nil {
		t.Fatalf("ProcessFile() error = %v", err)
	}

	if !strings.Contains(msg, "⚠ Conflict resolved: saving as") {
		t.Errorf("ProcessFile() message = %v", msg)
	}

	assertFileContent(t, dst, dstContent)

	renamed := filepath.Join(tmpDir, "dest_1.txt")
	assertFileContent(t, renamed, srcContent)
	assertFileNotExists(t, src) // Move should delete source
}

func TestProcessFile_InvalidConflictStrategy(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	createTestFile(t, src, []byte("content"), 0644)
	createTestFile(t, dst, []byte("existing"), 0644)

	ex := NewExecutor(nil,
		&cli.Opts{
			Sort:             cli.SortOption{Primary: "", Secondary: ""},
			SourcePath:       "",
			DestPath:         "",
			FileAction:       "copy",
			ConflictStrategy: "invalid_strategy",
			Force:            "",
			DryRun:           true,
			Workers:          10},
	)

	msg, err := ex.ProcessFile(src, dst)
	if err == nil {
		t.Error("ProcessFile() should error for invalid conflict strategy")
	}

	if msg != "" {
		t.Errorf("ProcessFile() message = %v", msg)
	}
}

func TestProcessFile_SourceNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "nonexistent.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	ex := NewExecutor(nil,
		&cli.Opts{
			Sort:             cli.SortOption{Primary: "", Secondary: ""},
			SourcePath:       "",
			DestPath:         "",
			FileAction:       "copy",
			ConflictStrategy: cli.ConflictReplace,
			Force:            "",
			DryRun:           true,
			Workers:          10},
	)

	msg, err := ex.ProcessFile(src, dst)
	if err == nil {
		t.Error("ProcessFile() should error for non-existent source")
	}

	if msg != "" {
		t.Errorf("ProcessFile() message = %v", msg)
	}
}

func TestProcessFile_Copy_PreservesMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	createTestFile(t, src, []byte("content"), 0755)

	pastTime := time.Date(2020, 6, 15, 10, 30, 0, 0, time.UTC)
	if err := os.Chtimes(src, pastTime, pastTime); err != nil {
		t.Fatalf("failed to set mod time: %v", err)
	}

	ex := NewExecutor(nil,
		&cli.Opts{
			Sort:             cli.SortOption{Primary: "", Secondary: ""},
			SourcePath:       "",
			DestPath:         "",
			FileAction:       "copy",
			ConflictStrategy: cli.ConflictReplace,
			Force:            "",
			DryRun:           true,
			Workers:          10},
	)

	msg, err := ex.ProcessFile(src, dst)
	if err != nil {
		t.Fatalf("ProcessFile() error = %v", err)
	}

	if msg != "" {
		t.Errorf("ProcessFile() message = %v", msg)
	}

	srcInfo, _ := os.Stat(src)
	dstInfo, _ := os.Stat(dst)

	if srcInfo.Mode() != dstInfo.Mode() {
		t.Errorf("permissions not preserved")
	}

	if !srcInfo.ModTime().Equal(dstInfo.ModTime()) {
		t.Errorf("mod time not preserved")
	}
}

// ─── executeAction tests ─────────────────────────────────────────────────────

func TestExecuteAction_CreatesDestinationDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source
	srcDir := filepath.Join(tmpDir, "source")
	srcFile := filepath.Join(srcDir, "file.txt")
	createTestFile(t, srcFile, []byte("content"), 0644)

	// Destination directory doesn't exist yet
	destDir := filepath.Join(tmpDir, "dest", "nested", "deep")

	action := types.Action{
		SourcePath: srcFile,
		DestPath:   destDir,
	}

	opts := &cli.Opts{
		FileAction:       "copy",
		ConflictStrategy: cli.ConflictReplace,
	}

	plan := &Plan{
		Actions: []types.Action{action},
	}

	executor := NewExecutor(plan, opts)

	msg, err := executor.executeAction(action)
	if err != nil {
		t.Fatalf("executeAction() error = %v", err)
	}

	if msg != "" {
		t.Errorf("ProcessFile() message = %v", msg)
	}

	// Check that directory was created
	if _, err := os.Stat(destDir); err != nil {
		t.Errorf("destination directory not created: %v", err)
	}

	// Check that file was copied
	finalPath := filepath.Join(destDir, "file.txt")
	assertFileContent(t, finalPath, []byte("content"))
}

func TestExecuteAction_Copy(t *testing.T) {
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "source", "file.txt")
	destDir := filepath.Join(tmpDir, "dest")

	createTestFile(t, srcFile, []byte("test"), 0644)

	action := types.Action{
		SourcePath: srcFile,
		DestPath:   destDir,
	}

	opts := &cli.Opts{
		FileAction:       "copy",
		ConflictStrategy: cli.ConflictReplace,
	}

	plan := &Plan{Actions: []types.Action{action}}
	executor := NewExecutor(plan, opts)

	msg, err := executor.executeAction(action)
	if err != nil {
		t.Fatalf("executeAction() error = %v", err)
	}

	if msg != "" {
		t.Errorf("ProcessFile() message = %v", msg)
	}

	finalPath := filepath.Join(destDir, "file.txt")
	assertFileContent(t, finalPath, []byte("test"))
	assertFileExists(t, srcFile) // Source should still exist
}

func TestExecuteAction_Move(t *testing.T) {
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "source", "file.txt")
	destDir := filepath.Join(tmpDir, "dest")

	createTestFile(t, srcFile, []byte("test"), 0644)

	action := types.Action{
		SourcePath: srcFile,
		DestPath:   destDir,
	}

	opts := &cli.Opts{
		FileAction:       "move",
		ConflictStrategy: cli.ConflictReplace,
	}

	plan := &Plan{Actions: []types.Action{action}}
	executor := NewExecutor(plan, opts)

	msg, err := executor.executeAction(action)
	if err != nil {
		t.Fatalf("executeAction() error = %v", err)
	}

	if msg != "" {
		t.Errorf("ProcessFile() message = %v", msg)
	}

	finalPath := filepath.Join(destDir, "file.txt")
	assertFileContent(t, finalPath, []byte("test"))
	assertFileNotExists(t, srcFile) // Source should be deleted
}

// ─── Edge cases ──────────────────────────────────────────────────────────────

func TestProcessFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "empty.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	createTestFile(t, src, []byte{}, 0644)

	ex := NewExecutor(nil,
		&cli.Opts{
			Sort:             cli.SortOption{Primary: "", Secondary: ""},
			SourcePath:       "",
			DestPath:         "",
			FileAction:       "copy",
			ConflictStrategy: cli.ConflictReplace,
			Force:            "",
			DryRun:           true,
			Workers:          10},
	)

	msg, err := ex.ProcessFile(src, dst)
	if err != nil {
		t.Fatalf("ProcessFile() error = %v", err)
	}

	if msg != "" {
		t.Errorf("ProcessFile() message = %v", msg)
	}

	assertFileContent(t, dst, []byte{})
}

func TestProcessFile_BinaryFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "binary.bin")
	dst := filepath.Join(tmpDir, "dest.bin")

	binaryContent := []byte{0x00, 0xFF, 0x42, 0xAA, 0xBB, 0xCC}
	createTestFile(t, src, binaryContent, 0644)

	ex := NewExecutor(nil,
		&cli.Opts{
			Sort:             cli.SortOption{Primary: "", Secondary: ""},
			SourcePath:       "",
			DestPath:         "",
			FileAction:       "copy",
			ConflictStrategy: cli.ConflictReplace,
			Force:            "",
			DryRun:           true,
			Workers:          10},
	)

	msg, err := ex.ProcessFile(src, dst)
	if err != nil {
		t.Fatalf("ProcessFile() error = %v", err)
	}

	if msg != "" {
		t.Errorf("ProcessFile() message = %v", msg)
	}

	assertFileContent(t, dst, binaryContent)
}

func TestProcessFile_UnicodeFilename(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "файл.txt")
	dst := filepath.Join(tmpDir, "目标.txt")

	content := []byte("unicode content")
	createTestFile(t, src, content, 0644)

	ex := NewExecutor(nil,
		&cli.Opts{
			Sort:             cli.SortOption{Primary: "", Secondary: ""},
			SourcePath:       "",
			DestPath:         "",
			FileAction:       "copy",
			ConflictStrategy: cli.ConflictReplace,
			Force:            "",
			DryRun:           true,
			Workers:          10},
	)

	msg, err := ex.ProcessFile(src, dst)
	if err != nil {
		t.Fatalf("ProcessFile() error = %v", err)
	}

	if msg != "" {
		t.Errorf("ProcessFile() message = %v", msg)
	}

	assertFileContent(t, dst, content)
}

func TestGenerateNewName_HighNumberConflicts(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "file.txt")

	// Create file.txt and file_1.txt through file_10.txt
	createTestFile(t, basePath, []byte("0"), 0644)
	for i := 1; i <= 10; i++ {
		path := filepath.Join(tmpDir, filepath.Base(basePath[:len(basePath)-4])+"_"+string(rune('0'+i))+".txt")
		if i < 10 {
			path = filepath.Join(tmpDir, "file_"+string(rune('0'+i))+".txt")
		} else {
			path = filepath.Join(tmpDir, "file_10.txt")
		}
		createTestFile(t, path, []byte{byte(i)}, 0644)
	}

	got := generateNewName(basePath)
	want := filepath.Join(tmpDir, "file_11.txt")

	if got != want {
		t.Errorf("generateNewName() = %q, want %q", got, want)
	}
}
