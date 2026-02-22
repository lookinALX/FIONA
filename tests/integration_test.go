package tests

import (
	"FIONA/internal/cli"
	"FIONA/internal/journal"
	"FIONA/internal/scanner"
	"FIONA/internal/sorter"
	"FIONA/internal/types"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// ── SORT ───────────────────────────────────────────────────────────────

func createTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("file doesn't exists: %s", path)
	}
}

func TestEndToEnd_CopyByTypeThenExtension(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	extensions := []string{".jpg", ".pdf", ".png", ".jpg", ".txt"}

	for i := range 5 {
		filename := "file" + strconv.Itoa(i) + extensions[i]
		createTestFile(t, filepath.Join(srcDir, filename), []byte("fake content"))
	}

	opts := cli.Opts{
		SourcePath:       srcDir,
		DestPath:         destDir,
		Sort:             cli.SortOption{Primary: cli.CritMIMEType, Secondary: cli.CritExtension},
		FileAction:       "copy",
		Force:            "yes",
		DryRun:           false,
		ConflictStrategy: cli.ConflictSkip,
		Workers:          runtime.NumCPU(),
	}

	rls, err := opts.ParseSortFlagsToRules()
	if err != nil {
		t.Fatal(err)
	}

	sc := scanner.NewScanner()
	files, err := sc.Scan(opts.SourcePath)
	if err != nil {
		t.Fatal(err)
	}

	plan := sorter.NewPlan(&opts)
	for _, f := range files {
		action := types.NewAction(f, rls, opts.DestPath)
		plan.AddAction(action)
	}

	executor := sorter.NewExecutor(&plan, &opts)
	executor.Execute()

	assertFileExists(t, filepath.Join(destDir, "images", "jpg", "file0.jpg"))
	assertFileExists(t, filepath.Join(destDir, "documents", "pdf", "file1.pdf"))
	assertFileExists(t, filepath.Join(destDir, "images", "png", "file2.png"))
	assertFileExists(t, filepath.Join(destDir, "images", "jpg", "file3.jpg"))
	assertFileExists(t, filepath.Join(destDir, "documents", "txt", "file4.txt"))

	assertFileExists(t, filepath.Join(srcDir, "file0.jpg"))
}

func TestEndToEnd_JournalAutoSave(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	journalPath := t.TempDir()

	createTestFile(t, filepath.Join(srcDir, "file1.txt"), []byte("content1"))
	createTestFile(t, filepath.Join(srcDir, "file2.jpg"), []byte("content2"))
	createTestFile(t, filepath.Join(srcDir, "file3.pdf"), []byte("content3"))

	opts := cli.Opts{
		SourcePath:       srcDir,
		DestPath:         destDir,
		Sort:             cli.SortOption{Primary: cli.CritMIMEType},
		FileAction:       "copy",
		Force:            "yes",
		DryRun:           false,
		ConflictStrategy: cli.ConflictSkip,
		Workers:          2,
		LogPath:          journalPath,
	}

	rls, _ := opts.ParseSortFlagsToRules()
	sc := scanner.NewScanner()
	files, _ := sc.Scan(opts.SourcePath)

	plan := sorter.NewPlan(&opts)
	for _, f := range files {
		action := types.NewAction(f, rls, opts.DestPath)
		plan.AddAction(action)
	}

	executor := sorter.NewExecutor(&plan, &opts)
	executor.Execute()

	// verify journal file was created
	if _, err := os.Stat(filepath.Join(journalPath, "fiona_logs.json")); os.IsNotExist(err) {
		t.Fatal("journal file was not auto-saved")
	}

	// verify file is not empty
	data, err := os.ReadFile(filepath.Join(journalPath, "fiona_logs.json"))
	if err != nil {
		t.Fatalf("failed to read journal file: %v", err)
	}

	if len(data) == 0 {
		t.Error("journal file is empty")
	}

	// load and verify content
	newJournal := journal.NewJournal(journalPath, "copy")
	err = newJournal.LoadFromJson(filepath.Join(journalPath, "fiona_logs.json"))
	if err != nil {
		t.Fatalf("failed to load saved journal: %v", err)
	}

	if newJournal.Count() != 3 {
		t.Errorf("expected 3 entries in saved journal, got %d", newJournal.Count())
	}

	// verify all entries are complete
	for i := 0; i < 3; i++ {
		entry, exists := newJournal.GetEntry(i)
		if !exists {
			t.Errorf("entry %d not found in saved journal", i)
			continue
		}

		if entry.SourcePath == "" {
			t.Errorf("entry %d: SourcePath is empty", i)
		}

		if entry.DestPath == "" {
			t.Errorf("entry %d: DestPath is empty", i)
		}

		if entry.Status == "" {
			t.Errorf("entry %d: Status is empty", i)
		}

		if entry.Timestamp.IsZero() {
			t.Errorf("entry %d: Timestamp is zero", i)
		}
	}
}

func TestEndToEnd_JournalAutoSaveWithConflicts(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	journalPath := t.TempDir()

	createTestFile(t, filepath.Join(srcDir, "file1.txt"), []byte("new1"))
	createTestFile(t, filepath.Join(srcDir, "file2.txt"), []byte("new2"))

	os.MkdirAll(filepath.Join(destDir, "documents", "txt"), 0755)
	createTestFile(t, filepath.Join(destDir, "documents", "txt", "file1.txt"), []byte("old1"))

	opts := cli.Opts{
		SourcePath:       srcDir,
		DestPath:         destDir,
		Sort:             cli.SortOption{Primary: cli.CritMIMEType, Secondary: cli.CritExtension},
		FileAction:       "copy",
		Force:            "yes",
		DryRun:           false,
		ConflictStrategy: cli.ConflictRename,
		Workers:          1,
		LogPath:          journalPath,
	}

	rls, _ := opts.ParseSortFlagsToRules()
	sc := scanner.NewScanner()
	files, _ := sc.Scan(opts.SourcePath)

	plan := sorter.NewPlan(&opts)
	for _, f := range files {
		action := types.NewAction(f, rls, opts.DestPath)
		plan.AddAction(action)
	}

	executor := sorter.NewExecutor(&plan, &opts)
	executor.Execute()

	// load saved journal
	savedJournal := journal.NewJournal(filepath.Join(journalPath, "fiona_logs.json"), "copy")
	err := savedJournal.LoadFromJson(filepath.Join(journalPath, "fiona_logs.json"))
	if err != nil {
		t.Fatalf("failed to load saved journal: %v", err)
	}

	// find entry with conflict resolution
	foundConflict := false
	for i := 0; i < savedJournal.Count(); i++ {
		entry, exists := savedJournal.GetEntry(i)
		if !exists {
			continue
		}

		if strings.Contains(entry.ConflictResolution, "Conflict resolved") {
			foundConflict = true

			if entry.Status != "succeeded" {
				t.Error("conflict resolution should result in success")
			}

			if !strings.Contains(entry.ConflictResolution, "file1_1.txt") {
				t.Errorf("expected renamed file in message, got: %s", entry.ConflictResolution)
			}
			break
		}
	}

	if !foundConflict {
		t.Error("expected to find conflict resolution in saved journal")
	}
}

func TestEndToEnd_JournalAutoSaveWithFailures(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	journalPath := t.TempDir()

	createTestFile(t, filepath.Join(srcDir, "valid.txt"), []byte("content"))

	opts := cli.Opts{
		SourcePath:       srcDir,
		DestPath:         destDir,
		Sort:             cli.SortOption{Primary: cli.CritMIMEType},
		FileAction:       "copy",
		Force:            "yes",
		DryRun:           false,
		ConflictStrategy: cli.ConflictSkip,
		Workers:          1,
		LogPath:          journalPath,
	}

	rls, _ := opts.ParseSortFlagsToRules()
	sc := scanner.NewScanner()
	files, _ := sc.Scan(opts.SourcePath)

	plan := sorter.NewPlan(&opts)
	for _, f := range files {
		action := types.NewAction(f, rls, opts.DestPath)
		plan.AddAction(action)
	}

	plan.AddAction(types.Action{
		SourcePath: "/nonexistent/missing.txt",
		DestPath:   destDir,
	})

	executor := sorter.NewExecutor(&plan, &opts)
	executor.Execute()

	savedJournal := journal.NewJournal(journalPath, "copy")
	err := savedJournal.LoadFromJson(filepath.Join(journalPath, "fiona_logs.json"))
	if err != nil {
		t.Fatalf("failed to load saved journal: %v", err)
	}

	if savedJournal.Count() != 2 {
		t.Errorf("expected 2 entries in saved journal, got %d", savedJournal.Count())
	}

	// find failed entry
	foundFailed := false
	for i := 0; i < savedJournal.Count(); i++ {
		entry, exists := savedJournal.GetEntry(i)
		if !exists {
			continue
		}

		if entry.Status == "failed" {
			foundFailed = true

			if entry.Error == "" {
				t.Error("failed entry should have error message in saved journal")
			}

			if !strings.Contains(entry.Error, "no such file") &&
				!strings.Contains(entry.Error, "does not exist") {
				t.Errorf("expected file not found error, got: %s", entry.Error)
			}
			break
		}
	}

	if !foundFailed {
		t.Error("expected to find failed entry in saved journal")
	}
}

func TestEndToEnd_JournalAutoSaveConcurrent(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	journalPath := t.TempDir()

	// create many files
	const numFiles = 50
	for i := 0; i < numFiles; i++ {
		ext := ".txt"
		if i%3 == 0 {
			ext = ".jpg"
		} else if i%3 == 1 {
			ext = ".pdf"
		}

		filename := fmt.Sprintf("file%d%s", i, ext)
		createTestFile(t, filepath.Join(srcDir, filename), []byte(fmt.Sprintf("content%d", i)))
	}

	opts := cli.Opts{
		SourcePath:       srcDir,
		DestPath:         destDir,
		Sort:             cli.SortOption{Primary: cli.CritMIMEType},
		FileAction:       "copy",
		Force:            "yes",
		DryRun:           false,
		ConflictStrategy: cli.ConflictSkip,
		Workers:          8,
		LogPath:          journalPath,
	}

	rls, _ := opts.ParseSortFlagsToRules()
	sc := scanner.NewScanner()
	files, _ := sc.Scan(opts.SourcePath)

	plan := sorter.NewPlan(&opts)
	for _, f := range files {
		action := types.NewAction(f, rls, opts.DestPath)
		plan.AddAction(action)
	}

	executor := sorter.NewExecutor(&plan, &opts)
	executor.Execute()

	// verify journal was saved
	if _, err := os.Stat(journalPath); os.IsNotExist(err) {
		t.Fatal("journal file was not auto-saved after concurrent execution")
	}

	// load and verify
	savedJournal := journal.NewJournal(journalPath, "copy")
	err := savedJournal.LoadFromJson(filepath.Join(journalPath, "fiona_logs.json"))
	if err != nil {
		t.Fatalf("failed to load saved journal after concurrent execution: %v", err)
	}

	if savedJournal.Count() != numFiles {
		t.Errorf("expected %d entries in saved journal, got %d", numFiles, savedJournal.Count())
	}

	// verify all entries succeeded
	successCount := 0
	for i := 0; i < numFiles; i++ {
		entry, exists := savedJournal.GetEntry(i)
		if !exists {
			continue
		}

		if entry.Status == "succeeded" {
			successCount++
		}
	}

	if successCount != numFiles {
		t.Errorf("expected all %d operations to succeed, got %d", numFiles, successCount)
	}
}

func TestEndToEnd_JournalSaveFailureHandling(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// use invalid path (directory that doesn't exist and can't be created)
	journalPath := "/root/restricted/"

	// create test file
	createTestFile(t, filepath.Join(srcDir, "file.txt"), []byte("content"))

	opts := cli.Opts{
		SourcePath:       srcDir,
		DestPath:         destDir,
		Sort:             cli.SortOption{Primary: cli.CritMIMEType},
		FileAction:       "copy",
		Force:            "yes",
		DryRun:           false,
		ConflictStrategy: cli.ConflictSkip,
		Workers:          1,
		LogPath:          journalPath,
	}

	rls, _ := opts.ParseSortFlagsToRules()
	sc := scanner.NewScanner()
	files, _ := sc.Scan(opts.SourcePath)

	plan := sorter.NewPlan(&opts)
	for _, f := range files {
		action := types.NewAction(f, rls, opts.DestPath)
		plan.AddAction(action)
	}

	executor := sorter.NewExecutor(&plan, &opts)

	// this should not panic, just print error
	// execution should complete even if journal save fails
	executor.Execute()

	// verify files were still processed despite journal save failure
	// walk destination to find the file
	fileFound := false
	filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() == "file.txt" {
			fileFound = true
		}
		return nil
	})

	if !fileFound {
		t.Fatal("Expected file.txt to be processed and copied, but it was not found in destination")
	}
}

// ── UNDO ───────────────────────────────────────────────────────────────

// runSort is a helper that runs a full sort operation and returns the saved journal path
func runSort(t *testing.T, srcDir, destDir, logDir, fileAction string) string {
	t.Helper()

	opts := cli.Opts{
		SourcePath:       srcDir,
		DestPath:         destDir,
		Sort:             cli.SortOption{Primary: cli.CritMIMEType},
		FileAction:       fileAction,
		Force:            "yes",
		DryRun:           false,
		ConflictStrategy: cli.ConflictSkip,
		Workers:          runtime.NumCPU(),
		LogPath:          logDir,
	}

	rls, err := opts.ParseSortFlagsToRules()
	if err != nil {
		t.Fatal(err)
	}

	sc := scanner.NewScanner()
	files, err := sc.Scan(opts.SourcePath)
	if err != nil {
		t.Fatal(err)
	}

	plan := sorter.NewPlan(&opts)
	for _, f := range files {
		action := types.NewAction(f, rls, opts.DestPath)
		plan.AddAction(action)
	}

	executor := sorter.NewExecutor(&plan, &opts)
	executor.Execute()

	return filepath.Join(logDir, "fiona_logs.json")
}

// runUndo is a helper that loads a journal and runs the undo operation
func runUndo(t *testing.T, logPath string, workers int) {
	t.Helper()

	jrn := journal.NewJournal("", "")
	if err := jrn.LoadFromJson(logPath); err != nil {
		t.Fatalf("failed to load journal: %v", err)
	}

	rv := sorter.NewReverter(sorter.NewUndoPlan(&jrn), workers)
	rv.RunUndo()
}

// TestEndToEnd_UndoCopy verifies that after sort+copy, undo removes the copied files
// but leaves the originals in source untouched
func TestEndToEnd_UndoCopy(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	logDir := t.TempDir()

	createTestFile(t, filepath.Join(srcDir, "photo.jpg"), []byte("jpg content"))
	createTestFile(t, filepath.Join(srcDir, "doc.pdf"), []byte("pdf content"))
	createTestFile(t, filepath.Join(srcDir, "note.txt"), []byte("txt content"))

	logPath := runSort(t, srcDir, destDir, logDir, "copy")

	// verify files were copied to dest
	fileFound := false
	filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() == "photo.jpg" {
			fileFound = true
		}
		return nil
	})
	if !fileFound {
		t.Fatal("sort did not copy files, test precondition failed")
	}

	runUndo(t, logPath, runtime.NumCPU())

	// after undo: originals must still exist in src
	assertFileExists(t, filepath.Join(srcDir, "photo.jpg"))
	assertFileExists(t, filepath.Join(srcDir, "doc.pdf"))
	assertFileExists(t, filepath.Join(srcDir, "note.txt"))

	// after undo: copied files must be removed from dest
	assertFileNotExists(t, destDir, "photo.jpg")
	assertFileNotExists(t, destDir, "doc.pdf")
	assertFileNotExists(t, destDir, "note.txt")
}

// TestEndToEnd_UndoMove verifies that after sort+move, undo moves the files back to source
func TestEndToEnd_UndoMove(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	logDir := t.TempDir()

	createTestFile(t, filepath.Join(srcDir, "photo.jpg"), []byte("jpg content"))
	createTestFile(t, filepath.Join(srcDir, "doc.pdf"), []byte("pdf content"))

	logPath := runSort(t, srcDir, destDir, logDir, "move")

	// verify files were moved out of src
	if _, err := os.Stat(filepath.Join(srcDir, "photo.jpg")); !os.IsNotExist(err) {
		t.Fatal("sort did not move files, test precondition failed")
	}

	runUndo(t, logPath, runtime.NumCPU())

	// after undo: files must be back in src
	assertFileExists(t, filepath.Join(srcDir, "photo.jpg"))
	assertFileExists(t, filepath.Join(srcDir, "doc.pdf"))

	// after undo: files must be gone from dest
	assertFileNotExists(t, destDir, "photo.jpg")
	assertFileNotExists(t, destDir, "doc.pdf")
}

// TestEndToEnd_UndoSkipsFailedEntries verifies that failed log entries are not undone
func TestEndToEnd_UndoSkipsFailedEntries(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	logDir := t.TempDir()

	createTestFile(t, filepath.Join(srcDir, "valid.txt"), []byte("content"))

	opts := cli.Opts{
		SourcePath:       srcDir,
		DestPath:         destDir,
		Sort:             cli.SortOption{Primary: cli.CritMIMEType},
		FileAction:       "copy",
		Force:            "yes",
		DryRun:           false,
		ConflictStrategy: cli.ConflictSkip,
		Workers:          1,
		LogPath:          logDir,
	}

	rls, _ := opts.ParseSortFlagsToRules()
	sc := scanner.NewScanner()
	files, _ := sc.Scan(opts.SourcePath)

	plan := sorter.NewPlan(&opts)
	for _, f := range files {
		action := types.NewAction(f, rls, opts.DestPath)
		plan.AddAction(action)
	}
	// inject a failing action
	plan.AddAction(types.Action{
		SourcePath: "/nonexistent/ghost.txt",
		DestPath:   destDir,
	})

	executor := sorter.NewExecutor(&plan, &opts)
	executor.Execute()

	logPath := filepath.Join(logDir, "fiona_logs.json")

	// undo should not panic and should complete normally
	runUndo(t, logPath, 1)

	// valid.txt was copied then undone — should be gone from dest
	assertFileNotExists(t, destDir, "valid.txt")

	// original in src must be untouched
	assertFileExists(t, filepath.Join(srcDir, "valid.txt"))
}

// TestEndToEnd_UndoConcurrent verifies undo works correctly with multiple workers
func TestEndToEnd_UndoConcurrent(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	logDir := t.TempDir()

	const numFiles = 20
	for i := 0; i < numFiles; i++ {
		name := filepath.Join(srcDir, filepath.Join(srcDir, "file"+string(rune('0'+i))+".txt"))
		createTestFile(t, name, []byte("content"))
	}

	logPath := runSort(t, srcDir, destDir, logDir, "copy")
	runUndo(t, logPath, 8)

	// all copied files must be removed from dest
	count := 0
	filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			count++
		}
		return nil
	})

	if count != 0 {
		t.Errorf("expected 0 files in dest after undo, got %d", count)
	}
}

// assertFileNotExists walks the given root and fails if a file with the given name is found
func assertFileNotExists(t *testing.T, root, name string) {
	t.Helper()
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() == name {
			t.Errorf("file should not exist after undo but was found: %s", path)
		}
		return nil
	})
}
