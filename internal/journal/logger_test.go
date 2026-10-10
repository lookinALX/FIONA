package journal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lookinalx/fiona/internal/types"
)

var (
	testSource = filepath.Join(string(filepath.Separator), "data", "source")
	testDest   = filepath.Join(string(filepath.Separator), "data", "dest")
)

func testHeader(runID string) Header {
	return Header{
		RunID:        runID,
		StartedAt:    time.Date(2026, 10, 10, 15, 30, 12, 0, time.UTC),
		Source:       testSource,
		Dest:         testDest,
		FileAction:   "move",
		OnConflict:   "rename",
		FionaVersion: "0.4.0",
	}
}

func testActions() []types.Action {
	mtime := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	return []types.Action{
		{
			Seq: 0, Kind: types.OpMove,
			SourcePath: filepath.Join(testSource, "a.jpg"),
			TargetPath: filepath.Join(testDest, "images", "a.jpg"),
			Size:       2048, ModTime: mtime,
		},
		{
			Seq: 1, Kind: types.OpMove,
			SourcePath: filepath.Join(testSource, "b.jpg"),
			TargetPath: filepath.Join(testDest, "images", "b_1.jpg"),
			Size:       0, ModTime: mtime,
			Conflict: types.ConflictRenamed,
		},
		{
			Seq: 2, Kind: types.OpSkip,
			SourcePath: filepath.Join(testSource, "c.pdf"),
			TargetPath: filepath.Join(testDest, "documents", "c.pdf"),
			Note:       "already exists",
		},
		{
			Seq: 3, Kind: types.OpMove,
			SourcePath: filepath.Join(testSource, "d.txt"),
			TargetPath: filepath.Join(testDest, "documents", "d.txt"),
			Size:       7, ModTime: mtime,
			Conflict:  types.ConflictReplaced,
			TrashPath: filepath.Join(testDest, StateDir, TrashDir, "run", "documents", "d.txt"),
		},
	}
}

func testMkdirs() []string {
	return []string{filepath.Join(testDest, "images"), filepath.Join(testDest, "documents")}
}

// writeJournal creates a journal with the standard header and plan.
func writeJournal(t *testing.T, dir, runID string) *Writer {
	t.Helper()

	w, err := Create(dir, runID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := w.Begin(testHeader(runID), testMkdirs(), testActions()); err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	return w
}

func writeRaw(t *testing.T, lines ...string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "run.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

const rawHeader = `{"type":"header","version":2,"run_id":"20261010-153012-a1b2","started_at":"2026-10-10T15:30:12Z","source":"/data/source","dest":"/data/dest","file_action":"move","on_conflict":"rename","fiona_version":"0.4.0"}`

func TestNewRunID(t *testing.T) {
	now := time.Date(2026, 10, 10, 18, 30, 12, 0, time.FixedZone("UTC+3", 3*60*60))

	id := NewRunID(now)

	if !strings.HasPrefix(id, "20261010-153012-") || len(id) != len("20261010-153012-a1b2") {
		t.Errorf("NewRunID() = %q, want 20261010-153012-xxxx", id)
	}

	started, ok := RunTime(id)
	if !ok || !started.Equal(now) {
		t.Errorf("RunTime(%q) = %v, %v; want %v", id, started, ok, now.UTC())
	}
}

func TestRunTime_Invalid(t *testing.T) {
	for _, id := range []string{"", "trash", "20261010-153012", "20261010-153012-zzzz", "2026101x-153012-a1b2", "20261010_153012-a1b2"} {
		if _, ok := RunTime(id); ok {
			t.Errorf("RunTime(%q) accepted an invalid identifier", id)
		}
	}
}

func TestCreate_RefusesExistingFile(t *testing.T) {
	dir := t.TempDir()

	w, err := Create(dir, "run")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer w.Close(Summary{})

	if _, err := Create(dir, "run"); err == nil {
		t.Error("expected error when the journal file already exists")
	}
}

func TestCreate_MakesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), StateDir, RunsDir)

	w, err := Create(dir, "run")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer w.Close(Summary{})

	if w.Path() != filepath.Join(dir, "run.jsonl") {
		t.Errorf("Path() = %q", w.Path())
	}
	if _, err := os.Stat(w.Path()); err != nil {
		t.Errorf("journal file was not created: %v", err)
	}
}

func TestWriteAndRead(t *testing.T) {
	w := writeJournal(t, t.TempDir(), "20261010-153012-a1b2")

	doneMtime := time.Date(2026, 9, 1, 10, 0, 0, 500, time.UTC)
	if err := w.Done(0, types.StatusOK, nil, 2048, doneMtime); err != nil {
		t.Fatalf("Done() error = %v", err)
	}
	if err := w.Done(1, types.StatusFailed, errors.New("permission denied"), 0, time.Time{}); err != nil {
		t.Fatalf("Done() error = %v", err)
	}
	if err := w.Done(3, types.StatusOK, nil, 7, doneMtime); err != nil {
		t.Fatalf("Done() error = %v", err)
	}
	if err := w.Close(Summary{OK: 2, Failed: 1, Skipped: 1}); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	run, err := Read(w.Path())
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if run.Header != testHeader("20261010-153012-a1b2") {
		t.Errorf("Header = %+v", run.Header)
	}
	if len(run.Mkdirs) != 2 || run.Mkdirs[0] != testMkdirs()[0] || run.Mkdirs[1] != testMkdirs()[1] {
		t.Errorf("Mkdirs = %v", run.Mkdirs)
	}
	if len(run.Ops) != 4 {
		t.Fatalf("got %d ops, want 4", len(run.Ops))
	}

	for i, want := range testActions() {
		got := run.Ops[i].Action
		if got.Seq != want.Seq || got.Kind != want.Kind || got.SourcePath != want.SourcePath ||
			got.TargetPath != want.TargetPath || got.Size != want.Size || got.Conflict != want.Conflict ||
			got.TrashPath != want.TrashPath || got.Note != want.Note || !got.ModTime.Equal(want.ModTime) {
			t.Errorf("op %d = %+v, want %+v", i, got, want)
		}
	}

	ok := run.Ops[0].Result
	if ok == nil || ok.Status != types.StatusOK || ok.Size != 2048 || !ok.ModTime.Equal(doneMtime) {
		t.Errorf("result of op 0 = %+v", ok)
	}
	failed := run.Ops[1].Result
	if failed == nil || failed.Status != types.StatusFailed || failed.Error != "permission denied" {
		t.Errorf("result of op 1 = %+v", failed)
	}
	if run.Ops[2].Result != nil {
		t.Errorf("skip has a result: %+v", run.Ops[2].Result)
	}

	if run.Footer == nil || run.Footer.OK != 2 || run.Footer.Failed != 1 || run.Footer.Skipped != 1 {
		t.Errorf("Footer = %+v", run.Footer)
	}
	if run.UndoneAt != nil {
		t.Errorf("UndoneAt = %v, want nil", run.UndoneAt)
	}
	if err := run.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestBegin_PlanIsOnDiskBeforeFirstOperation(t *testing.T) {
	w := writeJournal(t, t.TempDir(), "run")
	defer w.Close(Summary{})

	run, err := Read(w.Path())
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if len(run.Ops) != len(testActions()) || len(run.Mkdirs) != len(testMkdirs()) {
		t.Errorf("plan is not complete: %d ops, %d mkdirs", len(run.Ops), len(run.Mkdirs))
	}
	if run.Footer != nil {
		t.Error("unfinished run has a footer")
	}
}

func TestClose_InterruptedRunHasNoFooter(t *testing.T) {
	w := writeJournal(t, t.TempDir(), "run")
	if err := w.Done(0, types.StatusOK, nil, 2048, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(Summary{OK: 1, Interrupted: true}); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	run, err := Read(w.Path())
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if run.Footer != nil {
		t.Error("interrupted run has a footer")
	}
	if run.Ops[0].Result == nil || run.Ops[1].Result != nil {
		t.Error("results are not read correctly")
	}
}

func TestDone_Concurrent(t *testing.T) {
	const numOps = 200

	actions := make([]types.Action, numOps)
	for i := range actions {
		actions[i] = types.Action{
			Seq: i, Kind: types.OpCopy,
			SourcePath: filepath.Join(testSource, "f", strings.Repeat("x", i+1)),
			TargetPath: filepath.Join(testDest, "f", strings.Repeat("x", i+1)),
		}
	}

	w, err := Create(t.TempDir(), "run")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Begin(testHeader("run"), nil, actions); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(seq int) {
			defer wg.Done()
			if err := w.Done(seq, types.StatusOK, nil, int64(seq), time.Now()); err != nil {
				t.Errorf("Done() error = %v", err)
			}
		}(i)
	}
	wg.Wait()

	if err := w.Close(Summary{OK: numOps}); err != nil {
		t.Fatal(err)
	}

	run, err := Read(w.Path())
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	for i, op := range run.Ops {
		if op.Seq != i || op.Result == nil || op.Result.Size != int64(i) {
			t.Fatalf("op %d = %+v with result %+v", i, op.Action, op.Result)
		}
	}
}

func TestRead_IgnoresTornLastLine(t *testing.T) {
	path := writeRaw(t,
		rawHeader,
		`{"type":"op","seq":0,"kind":"move","src":"/data/source/a.jpg","dst":"/data/dest/images/a.jpg","size":1}`,
		`{"type":"op","seq":1,"kind":"move","src":"/data/source/b.jpg","dst":"/data/dest/images/b.jpg","size":1}`,
		`{"type":"done","seq":0,"status":"ok","size":1}`,
		`{"type":"done","seq":1,"sta`,
	)

	run, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if run.Ops[0].Result == nil {
		t.Error("complete done line is lost")
	}
	if run.Ops[1].Result != nil {
		t.Error("torn done line is not ignored")
	}
	if run.Footer != nil {
		t.Error("torn journal has a footer")
	}
}

func TestRead_CorruptJournals(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
	}{
		{"empty file", nil},
		{"no header", []string{`{"type":"op","seq":0,"kind":"move","src":"/a","dst":"/b"}`, ""}},
		{"garbage", []string{"not a journal", ""}},
		{"broken line in the middle", []string{
			rawHeader,
			`{"type":"op","seq":0,"ki`,
			`{"type":"op","seq":1,"kind":"move","src":"/data/source/b.jpg","dst":"/data/dest/b.jpg"}`,
			"",
		}},
		{"done for unknown op", []string{rawHeader, `{"type":"done","seq":5,"status":"ok"}`, ""}},
		{"duplicate op", []string{
			rawHeader,
			`{"type":"op","seq":0,"kind":"move","src":"/data/source/a","dst":"/data/dest/a"}`,
			`{"type":"op","seq":0,"kind":"move","src":"/data/source/b","dst":"/data/dest/b"}`,
			"",
		}},
		{"duplicate done", []string{
			rawHeader,
			`{"type":"op","seq":0,"kind":"move","src":"/data/source/a","dst":"/data/dest/a"}`,
			`{"type":"done","seq":0,"status":"ok"}`,
			`{"type":"done","seq":0,"status":"ok"}`,
			"",
		}},
		{"unknown status", []string{
			rawHeader,
			`{"type":"op","seq":0,"kind":"move","src":"/data/source/a","dst":"/data/dest/a"}`,
			`{"type":"done","seq":0,"status":"maybe"}`,
			"",
		}},
		{"unknown record type", []string{rawHeader, `{"type":"exec","cmd":"rm"}`, ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Read(writeRaw(t, tt.lines...))
			if !errors.Is(err, ErrCorrupt) {
				t.Errorf("Read() error = %v, want %v", err, ErrCorrupt)
			}
		})
	}
}

func TestRead_OldFormat(t *testing.T) {
	path := writeRaw(t, `{
  "file_action": "move",
  "logs": {
    "0": {"SourcePath": "/data/source/a.jpg", "DestPath": "/data/dest/images", "Status": "succeeded"}
  }
}`)

	if _, err := Read(path); !errors.Is(err, ErrOldFormat) {
		t.Errorf("Read() error = %v, want %v", err, ErrOldFormat)
	}
}

func TestRead_UnknownVersion(t *testing.T) {
	for _, header := range []string{
		strings.Replace(rawHeader, `"version":2`, `"version":3`, 1),
		strings.Replace(rawHeader, `"version":2,`, ``, 1),
	} {
		if _, err := Read(writeRaw(t, header, "")); !errors.Is(err, ErrUnknownVersion) {
			t.Errorf("Read() error = %v, want %v", err, ErrUnknownVersion)
		}
	}
}

func TestRead_MissingFile(t *testing.T) {
	if _, err := Read(filepath.Join(t.TempDir(), "missing.jsonl")); err == nil {
		t.Error("expected error for a missing file")
	}
}

func TestValidate(t *testing.T) {
	op := func(src, dst, extra string) string {
		return `{"type":"op","seq":0,"kind":"move","src":"` + src + `","dst":"` + dst + `"` + extra + `}`
	}

	tests := []struct {
		name    string
		lines   []string
		wantErr bool
	}{
		{"valid", []string{rawHeader, op("/data/source/a", "/data/dest/x/a", "")}, false},
		{"valid with trash", []string{rawHeader, op("/data/source/a", "/data/dest/a", `,"trash":"/data/dest/.fiona/trash/r/a"`)}, false},
		{"valid mkdir", []string{rawHeader, `{"type":"mkdir","path":"/data/dest/images"}`}, false},
		{"noop inside both roots", []string{
			strings.Replace(rawHeader, `"source":"/data/source"`, `"source":"/data/dest"`, 1),
			`{"type":"op","seq":0,"kind":"noop","src":"/data/dest/x/a","dst":"/data/dest/x/a"}`,
		}, false},
		{"reject without dst", []string{rawHeader, `{"type":"op","seq":0,"kind":"reject","src":"/data/source/a","note":"unsafe"}`}, false},
		{"relative source", []string{strings.Replace(rawHeader, `"source":"/data/source"`, `"source":"data/source"`, 1)}, true},
		{"relative dest", []string{strings.Replace(rawHeader, `"dest":"/data/dest"`, `"dest":"dest"`, 1)}, true},
		{"dst outside dest", []string{rawHeader, op("/data/source/a", "/data/other/a", "")}, true},
		{"dst escapes with dots", []string{rawHeader, op("/data/source/a", "/data/dest/../other/a", "")}, true},
		{"dst is dest itself", []string{rawHeader, op("/data/source/a", "/data/dest", "")}, true},
		{"dst relative", []string{rawHeader, op("/data/source/a", "images/a", "")}, true},
		{"dst missing", []string{rawHeader, `{"type":"op","seq":0,"kind":"move","src":"/data/source/a"}`}, true},
		{"src outside source", []string{rawHeader, op("/etc/passwd", "/data/dest/passwd", "")}, true},
		{"src is not clean", []string{rawHeader, op("/data/source/x/../../../etc/passwd", "/data/dest/passwd", "")}, true},
		{"trash outside dest", []string{rawHeader, op("/data/source/a", "/data/dest/a", `,"trash":"/data/elsewhere/a"`)}, true},
		{"mkdir outside dest", []string{rawHeader, `{"type":"mkdir","path":"/data/other"}`}, true},
		{"mkdir is dest itself", []string{rawHeader, `{"type":"mkdir","path":"/data/dest"}`}, true},
		{"unknown kind", []string{rawHeader, `{"type":"op","seq":0,"kind":"delete","src":"/data/source/a","dst":"/data/dest/a"}`}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if filepath.Separator != '/' {
				t.Skip("paths in this test are written for unix")
			}

			run, err := Read(writeRaw(t, append(tt.lines, "")...))
			if err != nil {
				t.Fatalf("Read() error = %v", err)
			}

			err = run.Validate()
			if tt.wantErr && !errors.Is(err, ErrRejected) {
				t.Errorf("Validate() error = %v, want %v", err, ErrRejected)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestMarkUndone(t *testing.T) {
	w := writeJournal(t, t.TempDir(), "run")
	if err := w.Close(Summary{}); err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	if err := MarkUndone(w.Path(), at); err != nil {
		t.Fatalf("MarkUndone() error = %v", err)
	}

	run, err := Read(w.Path())
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if run.UndoneAt == nil || !run.UndoneAt.Equal(at) {
		t.Errorf("UndoneAt = %v, want %v", run.UndoneAt, at)
	}
	if len(run.Ops) != len(testActions()) || run.Footer == nil {
		t.Error("the rest of the journal is damaged")
	}
}

func TestMarkUndone_AfterTornLine(t *testing.T) {
	path := writeRaw(t,
		rawHeader,
		`{"type":"op","seq":0,"kind":"move","src":"/data/source/a.jpg","dst":"/data/dest/images/a.jpg","size":1}`,
		`{"type":"done","seq":0,"sta`,
	)

	if err := MarkUndone(path, time.Now()); err != nil {
		t.Fatalf("MarkUndone() error = %v", err)
	}

	run, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if run.UndoneAt == nil {
		t.Error("undo mark is lost after a torn line")
	}
	if len(run.Ops) != 1 || run.Ops[0].Result != nil {
		t.Errorf("ops = %+v", run.Ops)
	}
}

func TestFindLatest(t *testing.T) {
	dir := t.TempDir()

	for _, runID := range []string{"20261010-100000-aaaa", "20261010-120000-bbbb", "20261010-110000-cccc"} {
		w := writeJournal(t, dir, runID)
		if err := w.Close(Summary{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a journal"), 0644); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"20261010-120000-bbbb", "20261010-110000-cccc", "20261010-100000-aaaa"} {
		run, err := FindLatest(dir)
		if err != nil {
			t.Fatalf("FindLatest() error = %v", err)
		}
		if run.Header.RunID != want {
			t.Fatalf("FindLatest() = %s, want %s", run.Header.RunID, want)
		}
		if err := MarkUndone(run.Path, time.Now()); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := FindLatest(dir); !errors.Is(err, ErrNoRuns) || !strings.Contains(err.Error(), "already undone") {
		t.Errorf("FindLatest() error = %v, want %v mentioning the undone run", err, ErrNoRuns)
	}
}

func TestFindLatest_NoJournals(t *testing.T) {
	if _, err := FindLatest(t.TempDir()); !errors.Is(err, ErrNoRuns) {
		t.Errorf("FindLatest() error = %v, want %v", err, ErrNoRuns)
	}
	if _, err := FindLatest(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, ErrNoRuns) {
		t.Errorf("FindLatest() error = %v, want %v", err, ErrNoRuns)
	}
}

func TestFindLatest_BrokenLatestJournalIsNotSkipped(t *testing.T) {
	dir := t.TempDir()

	w := writeJournal(t, dir, "20261010-100000-aaaa")
	if err := w.Close(Summary{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "20261010-120000-bbbb.jsonl"), []byte("garbage\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := FindLatest(dir); !errors.Is(err, ErrCorrupt) {
		t.Errorf("FindLatest() error = %v, want %v", err, ErrCorrupt)
	}
}
