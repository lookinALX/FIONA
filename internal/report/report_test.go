package report

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lookinalx/fiona/internal/scanner"
	"github.com/lookinalx/fiona/internal/sorter"
	"github.com/lookinalx/fiona/internal/types"
)

type byCategory struct{}

func (byCategory) Route(fi *scanner.FileInfo) ([]string, bool) { return []string{fi.Category()}, true }

type onlyImages struct{}

func (onlyImages) Route(fi *scanner.FileInfo) ([]string, bool) {
	return []string{fi.Category()}, fi.Category() == scanner.CategoryImages
}

func writeFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		// binary content, so that unknown extensions are not taken for text
		if err := os.WriteFile(path, []byte{0, 1, 2, 3, 4, 5, 6, 7}, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func planFor(t *testing.T, dir string, router sorter.Router, action string) *sorter.Plan {
	t.Helper()
	scanned, err := scanner.Scan(dir, scanner.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return sorter.NewPlan(scanned, router, sorter.PlanParams{
		Source: dir, Dest: dir, FileAction: action, OnConflict: types.OnConflictRename, RunID: "20261010-153012-a1b2",
	})
}

func TestOverview(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "a.jpg", "b.png", "report.pdf", "x.torrent", "y.torrent", "z.ics", "w.bin", "inbox/old.txt")
	if err := os.MkdirAll(filepath.Join(dir, "documents"), 0755); err != nil {
		t.Fatal(err)
	}

	plan := planFor(t, dir, byCategory{}, "move")
	text := strings.Join(Overview(PlanView{Plan: plan}, Styles{}), "\n")

	want := strings.Join([]string{
		"  images/     2 files  16 B",
		"  documents/  1 file    8 B  exists",
		"  other/      4 files  32 B  .torrent 2, .bin 1, .ics 1",
		"",
		// documents/ is a target of the plan, not a folder left as is
		"  Left as is: 1 folder",
	}, "\n")
	if text != want {
		t.Errorf("overview:\n%s\nwant:\n%s", text, want)
	}

	if q := Question(plan); q != "Move 7 files (56 B) into 3 folders?" {
		t.Errorf("Question() = %q", q)
	}
}

func TestOverview_ZeroCountersAreHidden(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "a.jpg")

	lines := Overview(PlanView{Plan: planFor(t, dir, byCategory{}, "move")}, Styles{})
	for _, line := range lines {
		if strings.Contains(line, ": 0") {
			t.Errorf("zero counter in the overview: %q", line)
		}
	}
	if len(lines) != 1 {
		t.Errorf("overview = %q, want only the folder line", lines)
	}
}

func TestOverview_ManyFoldersAreCut(t *testing.T) {
	dir := t.TempDir()
	var names []string
	for i := 0; i < 15; i++ {
		names = append(names, fmt.Sprintf("f%02d.e%02d", i, i))
	}
	writeFiles(t, dir, names...)
	byExt := routerFunc(func(fi *scanner.FileInfo) ([]string, bool) { return []string{fi.Ext}, true })

	lines := Overview(PlanView{Plan: planFor(t, dir, byExt, "move")}, Styles{})
	if len(lines) != maxFolders+1 || lines[maxFolders] != "  … and 3 more folders, 3 files" {
		t.Errorf("overview = %q", lines)
	}
}

type routerFunc func(fi *scanner.FileInfo) ([]string, bool)

func (f routerFunc) Route(fi *scanner.FileInfo) ([]string, bool) { return f(fi) }

func TestNotes_Unmatched(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "a.jpg", "report.pdf", "notes.txt")

	plan := planFor(t, dir, onlyImages{}, "copy")
	notes := Notes(plan, Styles{})
	if len(notes) != 1 || notes[0] != "  Left as is: 2 files that match no rule" {
		t.Errorf("notes = %q", notes)
	}
	if q := Question(plan); q != "Copy 1 file (8 B) into 1 folder?" {
		t.Errorf("Question() = %q", q)
	}

	list := strings.Join(FileList(plan, Styles{}), "\n")
	for _, want := range []string{"a.jpg → images/a.jpg", "report.pdf: matches no rule, stays"} {
		if !strings.Contains(list, want) {
			t.Errorf("list does not contain %q:\n%s", want, list)
		}
	}
}

func TestNothingToSort(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "inbox/a.txt", "old/b.txt")

	plan := planFor(t, dir, byCategory{}, "move")
	if !Empty(plan) {
		t.Fatal("plan is not empty")
	}
	msg := NothingToSort(PlanView{Plan: plan})
	if !strings.HasSuffix(msg, ": no files at the top level, only 2 folders. Use -r to include the files inside them.") {
		t.Errorf("NothingToSort() = %q", msg)
	}
}

func TestSortOutcome(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "a.jpg", "b.jpg", "c.pdf")
	plan := planFor(t, dir, byCategory{}, "move")

	ok := SortOutcome(SortResult{Plan: plan, Summary: sorter.Summary{Moved: 3}, Elapsed: 1200 * time.Millisecond, UndoHint: "fiona undo"}, Styles{})
	if len(ok) != 1 || ok[0] != "✓ Moved 3 files in 1.2s. Undo: fiona undo" {
		t.Errorf("ok = %q", ok)
	}

	failed := SortOutcome(SortResult{
		Plan:     plan,
		Summary:  sorter.Summary{Moved: 2, Failed: 1},
		Problems: []Problem{{Path: "c.pdf", Err: "permission denied"}},
		UndoHint: "fiona undo",
	}, Styles{})
	want := []string{"! Moved 2 of 3 files, 1 failed:", "  ✗ c.pdf: permission denied", "  Undo: fiona undo"}
	if strings.Join(failed, "\n") != strings.Join(want, "\n") {
		t.Errorf("failed = %q, want %q", failed, want)
	}

	stopped := SortOutcome(SortResult{Plan: plan, Summary: sorter.Summary{Moved: 1, Interrupted: true}, UndoHint: "fiona undo"}, Styles{})
	if stopped[0] != "Stopped after 1 of 3 files. Undo: fiona undo" {
		t.Errorf("stopped = %q", stopped)
	}
}

func TestWhen(t *testing.T) {
	now := time.Date(2026, 10, 17, 15, 0, 0, 0, time.Local)
	tests := map[time.Time]string{
		time.Date(2026, 10, 17, 9, 5, 0, 0, time.Local):   "today 09:05",
		time.Date(2026, 10, 16, 23, 59, 0, 0, time.Local): "yesterday 23:59",
		time.Date(2026, 3, 2, 15, 28, 0, 0, time.Local):   "2 Mar 15:28",
		time.Date(2025, 10, 12, 8, 0, 0, 0, time.Local):   "12 Oct 2025 08:00",
	}
	for at, want := range tests {
		if got := When(at, now); got != want {
			t.Errorf("When(%v) = %q, want %q", at, got, want)
		}
	}
}

func TestFormatting(t *testing.T) {
	sizes := map[int64]string{0: "0 B", 1023: "1023 B", 2458: "2.4 KB", 432013312: "412 MB", 1288490189: "1.2 GB"}
	for bytes, want := range sizes {
		if got := Size(bytes); got != want {
			t.Errorf("Size(%d) = %q, want %q", bytes, got, want)
		}
	}
	if Count(1, "file") != "1 file" || Count(2, "copy") != "2 copies" || Count(3, "folder") != "3 folders" {
		t.Error("Count pluralizes wrong")
	}
	if ShellQuote("/a/b") != "/a/b" || ShellQuote("/a/My Files") != "'/a/My Files'" {
		t.Error("ShellQuote quotes wrong")
	}
	if Duration(50*time.Millisecond) != "under 0.1s" || Duration(125*time.Second) != "2m05s" {
		t.Error("Duration formats wrong")
	}
}
