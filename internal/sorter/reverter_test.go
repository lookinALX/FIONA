package sorter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/types"
)

// sortAndRead runs a full sort and returns the journal of the run.
func sortAndRead(t *testing.T, source, dest, fileAction, onConflict string) *journal.Run {
	t.Helper()

	plan := scanPlan(t, source, dest, fileAction, onConflict)
	summary, run := execute(t, context.Background(), plan, 4, &recorder{})
	if summary.Failed != 0 || summary.Interrupted {
		t.Fatalf("sort failed: %+v", summary)
	}
	return run
}

// undo plans and runs the undo of a journal file.
func undo(t *testing.T, journalPath string, workers int) (Summary, *recorder) {
	t.Helper()

	run, err := journal.Read(journalPath)
	if err != nil {
		t.Fatalf("journal.Read() error = %v", err)
	}
	plan, err := NewUndoPlan(run)
	if err != nil {
		t.Fatalf("NewUndoPlan() error = %v", err)
	}

	rep := &recorder{}
	summary, err := NewReverter(plan, workers).Run(context.Background(), rep)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return summary, rep
}

func isUndone(t *testing.T, journalPath string) bool {
	t.Helper()

	run, err := journal.Read(journalPath)
	if err != nil {
		t.Fatalf("journal.Read() error = %v", err)
	}
	return run.UndoneAt != nil
}

// snapshot maps every file under root to its content.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()

	files := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			rel, _ := filepath.Rel(root, path)
			files[rel] = string(readFileContent(t, path))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func assertSnapshot(t *testing.T, root string, want map[string]string) {
	t.Helper()

	got := snapshot(t, root)
	if len(got) != len(want) {
		t.Errorf("%s has %d files, want %d: %v", root, len(got), len(want), got)
	}
	for name, content := range want {
		if got[name] != content {
			t.Errorf("%s: content = %q, want %q", filepath.Join(root, name), got[name], content)
		}
	}
}

func TestUndo_Move(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("jpg content"), 0644)
	createTestFile(t, filepath.Join(source, "sub", "doc.pdf"), []byte("pdf content"), 0644)
	before := snapshot(t, source)

	run := sortAndRead(t, source, dest, "move", types.OnConflictRename)
	assertSnapshot(t, source, map[string]string{})

	summary, _ := undo(t, run.Path, 4)

	if summary.Restored != 2 || summary.Failed != 0 || summary.Skipped != 0 || summary.DirsRemoved != 2 {
		t.Errorf("summary = %+v", summary)
	}
	assertSnapshot(t, source, before)
	assertSnapshot(t, dest, map[string]string{})
	assertFileNotExists(t, filepath.Join(dest, "images"))
	assertFileNotExists(t, filepath.Join(dest, "documents"))
	assertFileExists(t, dest)

	if !isUndone(t, run.Path) {
		t.Error("journal is not marked as undone")
	}
}

func TestUndo_Copy(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("jpg content"), 0644)
	createTestFile(t, filepath.Join(source, "doc.pdf"), []byte("pdf content"), 0644)
	before := snapshot(t, source)

	run := sortAndRead(t, source, dest, "copy", types.OnConflictRename)

	summary, _ := undo(t, run.Path, 4)

	if summary.Removed != 2 || summary.Restored != 0 || summary.Failed != 0 {
		t.Errorf("summary = %+v", summary)
	}
	assertSnapshot(t, source, before)
	assertSnapshot(t, dest, map[string]string{})
}

func TestUndo_RecreatesSourceDirectory(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "sub", "deep", "photo.jpg"), []byte("jpg content"), 0644)

	run := sortAndRead(t, source, dest, "move", types.OnConflictRename)
	if err := os.RemoveAll(filepath.Join(source, "sub")); err != nil {
		t.Fatal(err)
	}

	summary, _ := undo(t, run.Path, 1)

	if summary.Restored != 1 || summary.Failed != 0 {
		t.Errorf("summary = %+v", summary)
	}
	assertFileContent(t, filepath.Join(source, "sub", "deep", "photo.jpg"), []byte("jpg content"))
}

func TestUndo_DoesNotTouchFilesThatExistedBeforeTheRun(t *testing.T) {
	for _, fileAction := range []string{"move", "copy"} {
		for _, strategy := range []string{types.OnConflictSkip, types.OnConflictRename} {
			t.Run(fileAction+"/"+strategy, func(t *testing.T) {
				source, dest := t.TempDir(), t.TempDir()
				createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("new photo"), 0644)
				createTestFile(t, filepath.Join(source, "doc.pdf"), []byte("new doc"), 0644)
				createTestFile(t, filepath.Join(dest, "images", "photo.jpg"), []byte("old photo"), 0644)
				createTestFile(t, filepath.Join(dest, "images", "other.png"), []byte("old other"), 0644)
				sourceBefore, destBefore := snapshot(t, source), snapshot(t, dest)

				run := sortAndRead(t, source, dest, fileAction, strategy)
				summary, _ := undo(t, run.Path, 2)

				if summary.Failed != 0 || summary.Skipped != 0 {
					t.Errorf("summary = %+v", summary)
				}
				assertSnapshot(t, source, sourceBefore)
				assertSnapshot(t, dest, destBefore)
			})
		}
	}
}

func TestUndo_Replace(t *testing.T) {
	for _, fileAction := range []string{"move", "copy"} {
		t.Run(fileAction, func(t *testing.T) {
			source, dest := t.TempDir(), t.TempDir()
			createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("new photo"), 0644)
			createTestFile(t, filepath.Join(dest, "images", "photo.jpg"), []byte("old photo"), 0644)
			sourceBefore, destBefore := snapshot(t, source), snapshot(t, dest)

			run := sortAndRead(t, source, dest, fileAction, types.OnConflictReplace)
			assertFileContent(t, filepath.Join(dest, "images", "photo.jpg"), []byte("new photo"))

			summary, _ := undo(t, run.Path, 2)

			if summary.TrashRestored != 1 || summary.Failed != 0 || summary.Skipped != 0 {
				t.Errorf("summary = %+v", summary)
			}
			assertSnapshot(t, source, sourceBefore)
			assertSnapshot(t, dest, destBefore)
			assertFileNotExists(t, filepath.Join(dest, ".fiona"))
		})
	}
}

func TestUndo_SkipsFilesChangedAfterTheRun(t *testing.T) {
	changes := map[string]func(t *testing.T, path string){
		"size": func(t *testing.T, path string) {
			createTestFile(t, path, []byte("edited after the run, different size"), 0644)
		},
		"mtime": func(t *testing.T, path string) {
			later := time.Now().Add(time.Hour)
			if err := os.Chtimes(path, later, later); err != nil {
				t.Fatal(err)
			}
		},
	}

	for _, fileAction := range []string{"move", "copy"} {
		for name, change := range changes {
			t.Run(fileAction+"/"+name, func(t *testing.T) {
				source, dest := t.TempDir(), t.TempDir()
				createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("jpg content"), 0644)
				createTestFile(t, filepath.Join(source, "doc.pdf"), []byte("pdf content"), 0644)

				run := sortAndRead(t, source, dest, fileAction, types.OnConflictRename)

				changed := filepath.Join(dest, "images", "photo.jpg")
				change(t, changed)
				changedContent := readFileContent(t, changed)

				plan, err := NewUndoPlan(run)
				if err != nil {
					t.Fatal(err)
				}
				if plan.Stats.Skipped != 1 {
					t.Errorf("preview: skipped = %d, want 1", plan.Stats.Skipped)
				}

				summary, rep := undo(t, run.Path, 2)

				if summary.Skipped != 1 || summary.Failed != 0 {
					t.Errorf("summary = %+v", summary)
				}
				var skipErr error
				for _, err := range rep.errs {
					skipErr = err
				}
				if !errors.Is(skipErr, ErrUndoSkipped) {
					t.Errorf("error = %v, want %v", skipErr, ErrUndoSkipped)
				}

				// the changed file is left where it is, the other one is reverted
				assertFileContent(t, changed, changedContent)
				assertFileNotExists(t, filepath.Join(dest, "documents", "doc.pdf"))
				assertFileExists(t, filepath.Join(source, "doc.pdf"))
				assertFileExists(t, filepath.Join(dest, "images"))
			})
		}
	}
}

func TestUndo_NeverOverwritesOccupiedSource(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("moved"), 0644)

	run := sortAndRead(t, source, dest, "move", types.OnConflictRename)
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("created later"), 0644)

	summary, _ := undo(t, run.Path, 1)

	if summary.Skipped != 1 || summary.Restored != 0 || summary.Failed != 0 {
		t.Errorf("summary = %+v", summary)
	}
	assertFileContent(t, filepath.Join(source, "photo.jpg"), []byte("created later"))
	assertFileContent(t, filepath.Join(dest, "images", "photo.jpg"), []byte("moved"))
}

func TestUndo_InterruptedRun(t *testing.T) {
	for _, fileAction := range []string{"move", "copy"} {
		t.Run(fileAction, func(t *testing.T) {
			source, dest := t.TempDir(), t.TempDir()

			const numFiles = 20
			for i := 0; i < numFiles; i++ {
				createTestFile(t, filepath.Join(source, fmt.Sprintf("file%02d.txt", i)), []byte(fmt.Sprintf("content %d", i)), 0644)
			}
			before := snapshot(t, source)

			run := sortAndRead(t, source, dest, fileAction, types.OnConflictRename)

			// as after kill -9: the plan is in the journal, results of the operations are not
			data, err := os.ReadFile(run.Path)
			if err != nil {
				t.Fatal(err)
			}
			var kept []string
			for _, line := range strings.SplitAfter(string(data), "\n") {
				if !strings.Contains(line, `"type":"done"`) && !strings.Contains(line, `"type":"footer"`) {
					kept = append(kept, line)
				}
			}
			if err := os.WriteFile(run.Path, []byte(strings.Join(kept, "")+`{"type":"done","seq":3,"sta`), 0644); err != nil {
				t.Fatal(err)
			}
			// and a temporary file of a copy that was never finished
			part := filepath.Join(dest, "documents", fmt.Sprintf(".fiona-%s-7.part", run.Header.RunID))
			createTestFile(t, part, []byte("half of a file"), 0600)

			summary, _ := undo(t, run.Path, 4)

			if summary.Failed != 0 || summary.Skipped != 0 || summary.Restored+summary.Removed != numFiles {
				t.Errorf("summary = %+v", summary)
			}
			assertSnapshot(t, source, before)
			assertSnapshot(t, dest, map[string]string{})
			assertFileNotExists(t, part)
			if !isUndone(t, run.Path) {
				t.Error("journal is not marked as undone")
			}
		})
	}
}

func TestUndo_InterruptedReplaceDoesNotRemoveOldFile(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("12345"), 0644)
	createTestFile(t, filepath.Join(dest, "images", "photo.jpg"), []byte("54321"), 0644)

	// the run was killed before the operation started: the old file of the
	// same size is still at the target and nothing is in trash
	plan := scanPlan(t, source, dest, "copy", types.OnConflictReplace)
	jrn, err := journal.Create(t.TempDir(), plan.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := jrn.Begin(journal.Header{RunID: plan.RunID, Source: source, Dest: dest, FileAction: "copy"}, plan.Mkdirs, plan.Actions); err != nil {
		t.Fatal(err)
	}
	jrn.Close(journal.Summary{Interrupted: true})

	summary, _ := undo(t, jrn.Path(), 1)

	if summary.Removed != 0 || summary.Failed != 0 {
		t.Errorf("summary = %+v", summary)
	}
	assertFileContent(t, filepath.Join(dest, "images", "photo.jpg"), []byte("54321"))
	assertFileContent(t, filepath.Join(source, "photo.jpg"), []byte("12345"))
}

func TestUndo_KeepsDirectoriesThatAreNotEmpty(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("jpg content"), 0644)
	createTestFile(t, filepath.Join(source, "doc.pdf"), []byte("pdf content"), 0644)

	run := sortAndRead(t, source, dest, "move", types.OnConflictRename)
	createTestFile(t, filepath.Join(dest, "images", "added_later.png"), []byte("mine"), 0644)

	plan, err := NewUndoPlan(run)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Stats.EmptyDirs != 1 {
		t.Errorf("preview: empty directories = %d, want 1", plan.Stats.EmptyDirs)
	}

	summary, _ := undo(t, run.Path, 2)

	if summary.DirsRemoved != 1 {
		t.Errorf("summary = %+v", summary)
	}
	assertFileContent(t, filepath.Join(dest, "images", "added_later.png"), []byte("mine"))
	assertFileNotExists(t, filepath.Join(dest, "documents"))
}

func TestUndo_DoesNotRemoveDirectoriesThatExistedBeforeTheRun(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("jpg content"), 0644)
	if err := os.MkdirAll(filepath.Join(dest, "images"), 0755); err != nil {
		t.Fatal(err)
	}

	run := sortAndRead(t, source, dest, "move", types.OnConflictRename)
	summary, _ := undo(t, run.Path, 1)

	if summary.DirsRemoved != 0 {
		t.Errorf("summary = %+v", summary)
	}
	assertFileExists(t, filepath.Join(dest, "images"))
}

func TestUndo_Twice(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("jpg content"), 0644)

	run := sortAndRead(t, source, dest, "move", types.OnConflictRename)
	undo(t, run.Path, 1)

	undone, err := journal.Read(run.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewUndoPlan(undone); !errors.Is(err, ErrAlreadyUndone) {
		t.Errorf("NewUndoPlan() error = %v, want %v", err, ErrAlreadyUndone)
	}
}

func TestUndo_FailureKeepsJournalUndoable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write to any directory")
	}

	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "sub", "photo.jpg"), []byte("jpg content"), 0644)
	createTestFile(t, filepath.Join(source, "doc.pdf"), []byte("pdf content"), 0644)
	before := snapshot(t, source)

	run := sortAndRead(t, source, dest, "move", types.OnConflictRename)

	locked := filepath.Join(source, "sub")
	if err := os.Chmod(locked, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0755)

	summary, _ := undo(t, run.Path, 1)

	if summary.Failed != 1 || summary.Restored != 1 || summary.ExitCode() != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	assertFileExists(t, filepath.Join(dest, "images", "photo.jpg"))
	if isUndone(t, run.Path) {
		t.Fatal("journal is marked as undone although a file was not restored")
	}

	// once the problem is fixed, the same undo finishes the job
	if err := os.Chmod(locked, 0755); err != nil {
		t.Fatal(err)
	}
	summary, _ = undo(t, run.Path, 1)

	if summary.Failed != 0 || summary.Restored != 1 || summary.Skipped != 0 {
		t.Errorf("second undo: summary = %+v", summary)
	}
	assertSnapshot(t, source, before)
	if !isUndone(t, run.Path) {
		t.Error("journal is not marked as undone")
	}
}

func TestUndo_SkipsFailedOperations(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("jpg content"), 0644)
	createTestFile(t, filepath.Join(source, "doc.pdf"), []byte("pdf content"), 0644)

	plan := scanPlan(t, source, dest, "copy", types.OnConflictRename)
	// someone else puts a file to the target after planning: the operation fails
	createTestFile(t, filepath.Join(dest, "images", "photo.jpg"), []byte("not ours"), 0644)

	summary, run := execute(t, context.Background(), plan, 1, &recorder{})
	if summary.Failed != 1 || summary.Copied != 1 {
		t.Fatalf("summary = %+v", summary)
	}

	undoSummary, _ := undo(t, run.Path, 1)

	if undoSummary.Removed != 1 || undoSummary.Failed != 0 || undoSummary.Skipped != 0 {
		t.Errorf("summary = %+v", undoSummary)
	}
	assertFileContent(t, filepath.Join(dest, "images", "photo.jpg"), []byte("not ours"))
	assertFileNotExists(t, filepath.Join(dest, "documents", "doc.pdf"))
}

func TestUndo_Concurrent(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()

	const numFiles = 100
	for i := 0; i < numFiles; i++ {
		createTestFile(t, filepath.Join(source, fmt.Sprintf("dir%03d", i), "same.txt"), []byte(fmt.Sprintf("content %d", i)), 0644)
	}
	before := snapshot(t, source)

	run := sortAndRead(t, source, dest, "move", types.OnConflictRename)
	summary, rep := undo(t, run.Path, 16)

	if summary.Restored != numFiles || summary.Failed != 0 || rep.total != numFiles {
		t.Errorf("summary = %+v", summary)
	}
	assertSnapshot(t, source, before)
	assertSnapshot(t, dest, map[string]string{})
}

func TestUndo_Cancel(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()

	const numFiles = 30
	for i := 0; i < numFiles; i++ {
		createTestFile(t, filepath.Join(source, fmt.Sprintf("file%02d.txt", i)), []byte(fmt.Sprintf("content %d", i)), 0644)
	}
	before := snapshot(t, source)

	run := sortAndRead(t, source, dest, "move", types.OnConflictRename)

	plan, err := NewUndoPlan(run)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rep := &recorder{}
	rep.onDone = func() {
		if len(rep.done) == 3 {
			cancel()
		}
	}

	summary, err := NewReverter(plan, 1).Run(ctx, rep)
	if err != nil {
		t.Fatal(err)
	}
	if !summary.Interrupted || summary.Restored >= numFiles || summary.ExitCode() != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if isUndone(t, run.Path) {
		t.Fatal("interrupted undo marked the journal as undone")
	}

	// the undo is idempotent: a repeated run skips what is already back
	summary, _ = undo(t, run.Path, 4)

	if summary.Failed != 0 || summary.Skipped != 0 || summary.Interrupted {
		t.Errorf("second undo: summary = %+v", summary)
	}
	assertSnapshot(t, source, before)
	assertSnapshot(t, dest, map[string]string{})
}

func TestNewUndoPlan_RejectsForgedJournal(t *testing.T) {
	victimDir := t.TempDir()
	victim := filepath.Join(victimDir, "victim.txt")
	createTestFile(t, victim, []byte("victim"), 0644)

	source, dest := t.TempDir(), t.TempDir()
	header := fmt.Sprintf(`{"type":"header","version":2,"run_id":"20261010-153012-a1b2","started_at":"2026-10-10T15:30:12Z","source":%q,"dest":%q,"file_action":"copy","on_conflict":"rename","fiona_version":"0.4.0"}`, source, dest)
	op := fmt.Sprintf(`{"type":"op","seq":0,"kind":"copy","src":%q,"dst":%q,"size":6}`, filepath.Join(source, "victim.txt"), victim)

	path := filepath.Join(t.TempDir(), "forged.jsonl")
	if err := os.WriteFile(path, []byte(header+"\n"+op+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	run, err := journal.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewUndoPlan(run); !errors.Is(err, journal.ErrRejected) {
		t.Errorf("NewUndoPlan() error = %v, want %v", err, journal.ErrRejected)
	}
	assertFileContent(t, victim, []byte("victim"))
}

func TestUndoPlan_Print(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(source, "photo.jpg"), []byte("new photo"), 0644)
	createTestFile(t, filepath.Join(source, "doc.pdf"), []byte("pdf content"), 0644)
	createTestFile(t, filepath.Join(source, "clip.mp4"), []byte("video"), 0644)
	createTestFile(t, filepath.Join(dest, "images", "photo.jpg"), []byte("old photo"), 0644)

	run := sortAndRead(t, source, dest, "move", types.OnConflictReplace)
	createTestFile(t, filepath.Join(dest, "videos", "clip.mp4"), []byte("edited after the run"), 0644)

	plan, err := NewUndoPlan(run)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	plan.Print(&out)
	text := out.String()

	for _, want := range []string{
		"Undo of run " + run.Header.RunID + " (move, ",
		"move back to " + source + ": 2 files",
		"restore from trash: 1",
		// documents and the four levels of .fiona/trash/<run-id>/images; videos keeps the changed file
		"remove empty directories: 5",
		"skip (changed after the run): 1",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("preview does not contain %q:\n%s", want, text)
		}
	}
}
