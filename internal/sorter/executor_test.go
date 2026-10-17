package sorter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/scanner"
	"github.com/lookinalx/fiona/internal/types"
)

// Helper: create test file with content
func createTestFile(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
}

// Helper: check if file exists
func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// Helper: read file content
func readFileContent(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	return content
}

func assertFileContent(t *testing.T, path string, expected []byte) {
	t.Helper()
	actual := readFileContent(t, path)
	if !bytes.Equal(actual, expected) {
		t.Errorf("content mismatch in %s: expected %q, got %q", path, expected, actual)
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
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Mode().Perm() != expectedMode.Perm() {
		t.Errorf("file mode mismatch: expected %v, got %v", expectedMode.Perm(), info.Mode().Perm())
	}
}

// assertNoTempFiles fails if a run left .part files anywhere under root.
func assertNoTempFiles(t *testing.T, root string) {
	t.Helper()
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(d.Name(), ".part") {
			t.Errorf("temporary file is left behind: %s", path)
		}
		return nil
	})
}

// recorder is a Reporter that remembers what it was told.
type recorder struct {
	mu       sync.Mutex
	total    int
	done     []types.Action
	errs     map[int]error
	finished *Summary
	onDone   func()
}

func (r *recorder) Start(total int) {
	r.total = total
}

func (r *recorder) Done(a types.Action, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.done = append(r.done, a)
	if err != nil {
		if r.errs == nil {
			r.errs = make(map[int]error)
		}
		r.errs[a.Seq] = err
	}
	if r.onDone != nil {
		r.onDone()
	}
}

func (r *recorder) Finish(s Summary) {
	r.finished = &s
}

// scanPlan scans source and plans sorting by type into dest.
func scanPlan(t *testing.T, source, dest, fileAction, onConflict string) *Plan {
	t.Helper()

	scanned, err := scanner.Scan(source, scanner.Options{Recursive: true})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	return NewPlan(scanned, byCategory(), PlanParams{
		Source:     source,
		Dest:       dest,
		FileAction: fileAction,
		OnConflict: onConflict,
		RunID:      journal.NewRunID(time.Now()),
	})
}

// execute runs the plan and returns the summary and the journal that was written.
func execute(t *testing.T, ctx context.Context, plan *Plan, workers int, rep Reporter) (Summary, *journal.Run) {
	t.Helper()

	jrn, err := journal.Create(t.TempDir(), plan.RunID)
	if err != nil {
		t.Fatalf("journal.Create() error = %v", err)
	}

	summary, err := NewExecutor(plan, jrn, workers, "test").Run(ctx, rep)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	run, err := journal.Read(jrn.Path())
	if err != nil {
		t.Fatalf("journal.Read() error = %v", err)
	}

	return summary, run
}

// ─── copyFileAtomic ──────────────────────────────────────────────────────────

func TestCopyFileAtomic_Content(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source.txt")
	dest := filepath.Join(tmpDir, "dest.txt")
	content := []byte("Hello, World! This is test content.")

	createTestFile(t, source, content, 0644)

	if err := copyFileAtomic(source, dest, dest+".part"); err != nil {
		t.Fatalf("copyFileAtomic failed: %v", err)
	}

	assertFileContent(t, dest, content)
	assertFileContent(t, source, content)
	assertNoTempFiles(t, tmpDir)
}

func TestCopyFileAtomic_Permissions(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source.txt")
	dest := filepath.Join(tmpDir, "dest.txt")

	createTestFile(t, source, []byte("test"), 0600)

	if err := copyFileAtomic(source, dest, dest+".part"); err != nil {
		t.Fatalf("copyFileAtomic failed: %v", err)
	}

	assertFileMode(t, dest, 0600)
}

func TestCopyFileAtomic_ModTime(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source.txt")
	dest := filepath.Join(tmpDir, "dest.txt")

	createTestFile(t, source, []byte("test"), 0644)

	modTime := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(source, modTime, modTime); err != nil {
		t.Fatalf("failed to set mod time: %v", err)
	}

	if err := copyFileAtomic(source, dest, dest+".part"); err != nil {
		t.Fatalf("copyFileAtomic failed: %v", err)
	}

	destInfo, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("failed to stat dest: %v", err)
	}
	if !destInfo.ModTime().Equal(modTime) {
		t.Errorf("mod time not preserved: expected %v, got %v", modTime, destInfo.ModTime())
	}
}

func TestCopyFileAtomic_SourceNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "dest.txt")

	if err := copyFileAtomic(filepath.Join(tmpDir, "nonexistent.txt"), dest, dest+".part"); err == nil {
		t.Error("expected error when source doesn't exist")
	}

	assertFileNotExists(t, dest)
	assertNoTempFiles(t, tmpDir)
}

func TestCopyFileAtomic_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "large.bin")
	dest := filepath.Join(tmpDir, "large_copy.bin")

	// create 1MB file
	content := make([]byte, 1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	createTestFile(t, source, content, 0644)

	if err := copyFileAtomic(source, dest, dest+".part"); err != nil {
		t.Fatalf("copyFileAtomic failed: %v", err)
	}

	assertFileContent(t, dest, content)
}

func TestCopyFileAtomic_NeverOverwrites(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source.txt")
	dest := filepath.Join(tmpDir, "dest.txt")

	createTestFile(t, source, []byte("new"), 0644)
	createTestFile(t, dest, []byte("old"), 0644)

	if err := copyFileAtomic(source, dest, dest+".part"); !errors.Is(err, ErrTargetAppeared) {
		t.Errorf("error = %v, want %v", err, ErrTargetAppeared)
	}

	assertFileContent(t, dest, []byte("old"))
	assertNoTempFiles(t, tmpDir)
}

func TestCopyFileAtomic_DoesNotWriteThroughSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source.txt")
	outside := filepath.Join(t.TempDir(), "victim.txt")
	createTestFile(t, source, []byte("new"), 0644)
	createTestFile(t, outside, []byte("victim"), 0644)

	for _, link := range []string{"dest.txt", "dest.txt.part"} {
		t.Run(link, func(t *testing.T) {
			path := filepath.Join(tmpDir, link)
			if err := os.Symlink(outside, path); err != nil {
				t.Skipf("cannot create symlink: %v", err)
			}
			defer os.Remove(path)

			dest := filepath.Join(tmpDir, "dest.txt")
			if err := copyFileAtomic(source, dest, dest+".part"); err == nil {
				t.Error("expected error when a symlink is in the way")
			}

			assertFileContent(t, outside, []byte("victim"))
		})
	}
}

// ─── moveFile ────────────────────────────────────────────────────────────────

func TestMoveFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source.txt")
	dest := filepath.Join(tmpDir, "dest.txt")
	content := []byte("move me")

	createTestFile(t, source, content, 0644)

	if err := moveFile(source, dest, dest+".part"); err != nil {
		t.Fatalf("moveFile failed: %v", err)
	}

	assertFileNotExists(t, source)
	assertFileContent(t, dest, content)
}

func TestMoveFile_SourceNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "dest.txt")

	if err := moveFile(filepath.Join(tmpDir, "nonexistent.txt"), dest, dest+".part"); err == nil {
		t.Error("expected error when source doesn't exist")
	}
}

func TestMoveFile_NeverOverwrites(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source.txt")
	dest := filepath.Join(tmpDir, "dest.txt")

	createTestFile(t, source, []byte("new"), 0644)
	createTestFile(t, dest, []byte("old"), 0644)

	if err := moveFile(source, dest, dest+".part"); !errors.Is(err, ErrTargetAppeared) {
		t.Errorf("error = %v, want %v", err, ErrTargetAppeared)
	}

	assertFileContent(t, source, []byte("new"))
	assertFileContent(t, dest, []byte("old"))
}

// ─── Executor ────────────────────────────────────────────────────────────────

func TestExecutor_Copy(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("photo"), 0600)
	createTestFile(t, filepath.Join(source, "sub", "report.pdf"), []byte("report"), 0644)

	plan := scanPlan(t, source, dest, "copy", types.OnConflictRename)
	rep := &recorder{}
	summary, run := execute(t, context.Background(), plan, 4, rep)

	assertFileContent(t, filepath.Join(dest, "images", "photo.jpg"), []byte("photo"))
	assertFileContent(t, filepath.Join(dest, "documents", "report.pdf"), []byte("report"))
	assertFileMode(t, filepath.Join(dest, "images", "photo.jpg"), 0600)
	assertFileExists(t, filepath.Join(source, "photo.jpg"))
	assertFileExists(t, filepath.Join(source, "sub", "report.pdf"))
	assertNoTempFiles(t, dest)

	if summary.Copied != 2 || summary.Moved != 0 || summary.Failed != 0 || summary.Interrupted {
		t.Errorf("summary = %+v", summary)
	}
	if summary.ExitCode() != 0 {
		t.Errorf("ExitCode() = %d, want 0", summary.ExitCode())
	}
	if rep.total != 2 || len(rep.done) != 2 || rep.finished == nil || *rep.finished != summary {
		t.Errorf("reporter: total = %d, done = %d, finished = %+v", rep.total, len(rep.done), rep.finished)
	}

	if run.Footer == nil || run.Footer.OK != 2 || run.Footer.Failed != 0 {
		t.Errorf("footer = %+v", run.Footer)
	}
	for _, op := range run.Ops {
		info, err := os.Lstat(op.TargetPath)
		if err != nil {
			t.Fatal(err)
		}
		if op.Result == nil || op.Result.Status != types.StatusOK || op.Result.Size != info.Size() || !op.Result.ModTime.Equal(info.ModTime()) {
			t.Errorf("op %d: result %+v does not match the file on disk", op.Seq, op.Result)
		}
	}
}

func TestExecutor_Move(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("photo"), 0644)
	createTestFile(t, filepath.Join(source, "report.pdf"), []byte("report"), 0644)

	plan := scanPlan(t, source, dest, "move", types.OnConflictRename)
	summary, _ := execute(t, context.Background(), plan, 2, &recorder{})

	assertFileContent(t, filepath.Join(dest, "images", "photo.jpg"), []byte("photo"))
	assertFileContent(t, filepath.Join(dest, "documents", "report.pdf"), []byte("report"))
	assertFileNotExists(t, filepath.Join(source, "photo.jpg"))
	assertFileNotExists(t, filepath.Join(source, "report.pdf"))

	if summary.Moved != 2 || summary.Copied != 0 || summary.Failed != 0 {
		t.Errorf("summary = %+v", summary)
	}
}

func TestExecutor_SpecialFiles(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content []byte
	}{
		{"empty file", "empty.txt", []byte{}},
		{"binary file", "binary.bin", []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}},
		{"unicode filename", "тест_файл_日本語.txt", []byte("Unicode content: 你好世界")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, dest := t.TempDir(), t.TempDir()
			createTestFile(t, filepath.Join(source, tt.file), tt.content, 0644)

			plan := scanPlan(t, source, dest, "copy", types.OnConflictRename)
			summary, _ := execute(t, context.Background(), plan, 1, &recorder{})

			if summary.Copied != 1 || summary.Failed != 0 {
				t.Fatalf("summary = %+v", summary)
			}
			assertFileContent(t, plan.Actions[0].TargetPath, tt.content)
		})
	}
}

func TestExecutor_RenameKeepsEveryFile(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()

	const numFiles = 100
	for i := 0; i < numFiles; i++ {
		createTestFile(t, filepath.Join(source, fmt.Sprintf("dir%03d", i), "same.txt"), []byte(fmt.Sprintf("content %d", i)), 0644)
	}

	plan := scanPlan(t, source, dest, "move", types.OnConflictRename)
	summary, _ := execute(t, context.Background(), plan, 16, &recorder{})

	if summary.Moved != numFiles || summary.Renamed != numFiles-1 || summary.Failed != 0 {
		t.Fatalf("summary = %+v", summary)
	}

	contents := make(map[string]struct{})
	entries, err := os.ReadDir(filepath.Join(dest, "documents"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		contents[string(readFileContent(t, filepath.Join(dest, "documents", entry.Name())))] = struct{}{}
	}
	if len(entries) != numFiles || len(contents) != numFiles {
		t.Errorf("dest has %d files with %d different contents, want %d", len(entries), len(contents), numFiles)
	}
}

func TestExecutor_SkipLeavesSourceInPlace(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("new"), 0644)
	createTestFile(t, filepath.Join(dest, "images", "photo.jpg"), []byte("old"), 0644)

	plan := scanPlan(t, source, dest, "move", types.OnConflictSkip)
	rep := &recorder{}
	summary, run := execute(t, context.Background(), plan, 1, rep)

	assertFileContent(t, filepath.Join(source, "photo.jpg"), []byte("new"))
	assertFileContent(t, filepath.Join(dest, "images", "photo.jpg"), []byte("old"))

	if summary.Skipped != 1 || summary.Moved != 0 || summary.Failed != 0 {
		t.Errorf("summary = %+v", summary)
	}
	if rep.total != 0 || len(rep.done) != 0 {
		t.Errorf("skipped operation was reported as executed")
	}
	if run.Ops[0].Kind != types.OpSkip || run.Ops[0].Result != nil || run.Footer == nil || run.Footer.Skipped != 1 {
		t.Errorf("journal: op = %+v, result = %+v, footer = %+v", run.Ops[0].Action, run.Ops[0].Result, run.Footer)
	}
}

func TestExecutor_ReplaceMovesOldFileToTrash(t *testing.T) {
	for _, fileAction := range []string{"move", "copy"} {
		t.Run(fileAction, func(t *testing.T) {
			source, dest := t.TempDir(), t.TempDir()
			createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("new"), 0644)
			createTestFile(t, filepath.Join(dest, "images", "photo.jpg"), []byte("old"), 0644)

			plan := scanPlan(t, source, dest, fileAction, types.OnConflictReplace)
			summary, run := execute(t, context.Background(), plan, 1, &recorder{})

			if summary.Replaced != 1 || summary.Failed != 0 {
				t.Fatalf("summary = %+v", summary)
			}

			assertFileContent(t, filepath.Join(dest, "images", "photo.jpg"), []byte("new"))
			assertFileContent(t, filepath.Join(dest, ".fiona", "trash", plan.RunID, "images", "photo.jpg"), []byte("old"))
			if run.Ops[0].TrashPath != plan.Actions[0].TrashPath || run.Ops[0].Conflict != types.ConflictReplaced {
				t.Errorf("journal op = %+v", run.Ops[0].Action)
			}
		})
	}
}

func TestExecutor_ReplaceRestoresOldFileWhenOperationFails(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("new"), 0644)
	createTestFile(t, filepath.Join(dest, "images", "photo.jpg"), []byte("old"), 0644)

	plan := scanPlan(t, source, dest, "move", types.OnConflictReplace)
	// the source disappears after planning
	if err := os.Remove(filepath.Join(source, "photo.jpg")); err != nil {
		t.Fatal(err)
	}

	summary, _ := execute(t, context.Background(), plan, 1, &recorder{})

	if summary.Failed != 1 || summary.Replaced != 0 {
		t.Errorf("summary = %+v", summary)
	}
	assertFileContent(t, filepath.Join(dest, "images", "photo.jpg"), []byte("old"))
	assertFileNotExists(t, plan.Actions[0].TrashPath)
}

func TestExecutor_GuardFailsWhenTargetAppearsAfterPlanning(t *testing.T) {
	for _, fileAction := range []string{"move", "copy"} {
		t.Run(fileAction, func(t *testing.T) {
			source, dest := t.TempDir(), t.TempDir()
			createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("new"), 0644)

			plan := scanPlan(t, source, dest, fileAction, types.OnConflictRename)
			createTestFile(t, filepath.Join(dest, "images", "photo.jpg"), []byte("appeared"), 0644)

			rep := &recorder{}
			summary, run := execute(t, context.Background(), plan, 1, rep)

			if summary.Failed != 1 || summary.Moved != 0 || summary.Copied != 0 {
				t.Errorf("summary = %+v", summary)
			}
			if summary.ExitCode() != 1 {
				t.Errorf("ExitCode() = %d, want 1", summary.ExitCode())
			}
			if !errors.Is(rep.errs[0], ErrTargetAppeared) {
				t.Errorf("error = %v, want %v", rep.errs[0], ErrTargetAppeared)
			}

			assertFileContent(t, filepath.Join(source, "photo.jpg"), []byte("new"))
			assertFileContent(t, filepath.Join(dest, "images", "photo.jpg"), []byte("appeared"))
			assertNoTempFiles(t, dest)

			result := run.Ops[0].Result
			if result == nil || result.Status != types.StatusFailed || result.Error == "" {
				t.Errorf("journal result = %+v", result)
			}
			if run.Footer == nil || run.Footer.Failed != 1 || run.Footer.OK != 0 {
				t.Errorf("footer = %+v", run.Footer)
			}
		})
	}
}

func TestExecutor_NoJournalNoRun(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("photo"), 0644)

	plan := scanPlan(t, source, dest, "move", types.OnConflictRename)

	jrn, err := journal.Create(t.TempDir(), plan.RunID)
	if err != nil {
		t.Fatal(err)
	}
	// a journal that cannot be written to
	jrn.Close(journal.Summary{Interrupted: true})

	rep := &recorder{}
	if _, err := NewExecutor(plan, jrn, 1, "test").Run(context.Background(), rep); err == nil {
		t.Fatal("expected error when the journal cannot be written")
	}

	assertFileExists(t, filepath.Join(source, "photo.jpg"))
	assertFileNotExists(t, filepath.Join(dest, "images"))
	if len(rep.done) != 0 {
		t.Error("operations were executed without a journal")
	}
}

func TestExecutor_CancelStopsAfterCurrentFile(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()

	const numFiles = 50
	for i := 0; i < numFiles; i++ {
		createTestFile(t, filepath.Join(source, fmt.Sprintf("file%02d.txt", i)), []byte(fmt.Sprintf("content %d", i)), 0644)
	}

	plan := scanPlan(t, source, dest, "move", types.OnConflictRename)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rep := &recorder{}
	rep.onDone = func() {
		if len(rep.done) == 5 {
			cancel()
		}
	}

	summary, run := execute(t, ctx, plan, 1, rep)

	if !summary.Interrupted || summary.ExitCode() != 1 {
		t.Errorf("summary = %+v, want interrupted with exit code 1", summary)
	}
	if summary.Moved < 5 || summary.Moved >= numFiles {
		t.Errorf("moved %d files, want at least 5 and fewer than %d", summary.Moved, numFiles)
	}
	if run.Footer != nil {
		t.Error("interrupted run has a footer")
	}

	// the journal stays consistent: a file is at its target exactly when it has a result
	for _, op := range run.Ops {
		moved := op.Result != nil
		if fileExists(op.TargetPath) != moved || fileExists(op.SourcePath) == moved {
			t.Errorf("op %d: result = %+v, target exists = %v, source exists = %v",
				op.Seq, op.Result, fileExists(op.TargetPath), fileExists(op.SourcePath))
		}
	}
}

func TestExecutor_CreatesDestinationDirectories(t *testing.T) {
	source := t.TempDir()
	dest := filepath.Join(t.TempDir(), "a", "b", "c")
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("photo"), 0644)

	plan := scanPlan(t, source, dest, "copy", types.OnConflictRename)
	if fileExists(dest) {
		t.Fatal("planning created the destination directory")
	}

	summary, _ := execute(t, context.Background(), plan, 1, &recorder{})

	if summary.Copied != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	assertFileExists(t, filepath.Join(dest, "images", "photo.jpg"))
}

// ─── Trash retention ─────────────────────────────────────────────────────────

func TestCleanTrash(t *testing.T) {
	dest := t.TempDir()
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	trash := filepath.Join(dest, ".fiona", "trash")

	expired := journal.NewRunID(now.Add(-TrashRetention - time.Hour))
	fresh := journal.NewRunID(now.Add(-TrashRetention + time.Hour))

	createTestFile(t, filepath.Join(trash, expired, "images", "old.jpg"), []byte("old"), 0644)
	createTestFile(t, filepath.Join(trash, fresh, "images", "recent.jpg"), []byte("recent"), 0644)
	createTestFile(t, filepath.Join(trash, "not-a-run", "keep.txt"), []byte("keep"), 0644)
	createTestFile(t, filepath.Join(trash, "20200101-000000-aaaa.txt"), []byte("a file, not a run folder"), 0644)
	createTestFile(t, filepath.Join(dest, ".fiona", "runs", expired+".jsonl"), []byte("journal"), 0644)

	removed, err := CleanTrash(dest, now)
	if err != nil {
		t.Fatalf("CleanTrash() error = %v", err)
	}
	if removed != 1 {
		t.Errorf("removed %d trash folders, want 1", removed)
	}

	assertFileNotExists(t, filepath.Join(trash, expired))
	assertFileExists(t, filepath.Join(trash, fresh, "images", "recent.jpg"))
	assertFileExists(t, filepath.Join(trash, "not-a-run", "keep.txt"))
	assertFileExists(t, filepath.Join(trash, "20200101-000000-aaaa.txt"))
	assertFileExists(t, filepath.Join(dest, ".fiona", "runs", expired+".jsonl"))
}

func TestCleanTrash_NothingToClean(t *testing.T) {
	if removed, err := CleanTrash(t.TempDir(), time.Now()); removed != 0 || err != nil {
		t.Errorf("CleanTrash() = %d, %v; want 0, nil", removed, err)
	}
}

func TestCleanTrash_DoesNotFollowSymlinkedTrash(t *testing.T) {
	dest, outside := t.TempDir(), t.TempDir()
	now := time.Now()

	expired := journal.NewRunID(now.Add(-2 * TrashRetention))
	createTestFile(t, filepath.Join(outside, expired, "victim.txt"), []byte("victim"), 0644)
	if err := os.MkdirAll(filepath.Join(dest, ".fiona"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dest, ".fiona", "trash")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	if removed, err := CleanTrash(dest, now); removed != 0 || err != nil {
		t.Errorf("CleanTrash() = %d, %v; want 0, nil", removed, err)
	}
	assertFileExists(t, filepath.Join(outside, expired, "victim.txt"))
}
