package history

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/types"
)

// writeRun writes a finished journal and returns its history entry.
func writeRun(t *testing.T, runID string, started time.Time) Entry {
	t.Helper()

	dest := t.TempDir()
	w, err := journal.Create(filepath.Join(dest, journal.StateDir, journal.RunsDir), runID)
	if err != nil {
		t.Fatal(err)
	}
	header := journal.Header{RunID: runID, StartedAt: started, Source: dest, Dest: dest, FileAction: "move", OnConflict: "rename"}
	if err := w.Begin(header, nil, []types.Action{}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(journal.Summary{}); err != nil {
		t.Fatal(err)
	}

	return Entry{RunID: runID, StartedAt: started, Source: dest, Dest: dest, Journal: w.Path(), Action: "move", Planned: 0}
}

func TestAppendAndRead(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fiona")
	old := Entry{RunID: "20261010-100000-aaaa", StartedAt: time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC), Journal: "/x/a.jsonl"}
	recent := Entry{RunID: "20261011-100000-bbbb", StartedAt: time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC), Journal: "/x/b.jsonl"}

	for _, e := range []Entry{old, recent} {
		if err := Append(dir, e); err != nil {
			t.Fatal(err)
		}
	}
	// a torn line from a crash does not break the index
	f, err := os.OpenFile(filepath.Join(dir, FileName), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"run_id":"20261012`)
	f.Close()

	entries, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].RunID != recent.RunID || entries[1].RunID != old.RunID {
		t.Errorf("entries = %+v, want newest first", entries)
	}

	info, err := os.Stat(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("permissions = %o, want 600", perm)
	}
}

func TestRead_NoHistory(t *testing.T) {
	entries, err := Read(t.TempDir())
	if err != nil || entries != nil {
		t.Errorf("Read() = %v, %v", entries, err)
	}
}

func TestRecordsAndLatestUndoable(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)

	first := writeRun(t, "20261010-100000-aaaa", base)
	second := writeRun(t, "20261010-110000-bbbb", base.Add(time.Hour))
	missing := Entry{RunID: "20261010-120000-cccc", StartedAt: base.Add(2 * time.Hour), Journal: filepath.Join(t.TempDir(), "gone.jsonl")}
	for _, e := range []Entry{first, second, missing} {
		if err := Append(dir, e); err != nil {
			t.Fatal(err)
		}
	}

	records, err := Records(dir)
	if err != nil {
		t.Fatal(err)
	}
	if records[0].State != StateMissing || records[1].State != StateDone || records[2].State != StateDone {
		t.Errorf("states = %v, %v, %v", records[0].State, records[1].State, records[2].State)
	}

	latest, err := LatestUndoable(dir)
	if err != nil || latest == nil || latest.RunID != second.RunID {
		t.Fatalf("LatestUndoable() = %+v, %v; want %s", latest, err, second.RunID)
	}

	// like Ctrl+Z: once the newest run is undone, the one before it is next
	if err := journal.MarkUndone(second.Journal, time.Now()); err != nil {
		t.Fatal(err)
	}
	latest, err = LatestUndoable(dir)
	if err != nil || latest == nil || latest.RunID != first.RunID {
		t.Fatalf("LatestUndoable() = %+v, %v; want %s", latest, err, first.RunID)
	}

	if err := journal.MarkUndone(first.Journal, time.Now()); err != nil {
		t.Fatal(err)
	}
	if latest, err = LatestUndoable(dir); err != nil || latest != nil {
		t.Errorf("LatestUndoable() = %+v, %v; want nothing", latest, err)
	}
}

func TestFind(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
	a := writeRun(t, "20261010-100000-aaaa", base)
	b := writeRun(t, "20261010-100000-abbb", base.Add(time.Second))
	for _, e := range []Entry{a, b} {
		if err := Append(dir, e); err != nil {
			t.Fatal(err)
		}
	}

	if r, err := Find(dir, "20261010-100000-aa"); err != nil || r.RunID != a.RunID || r.Run == nil {
		t.Errorf("Find(prefix) = %+v, %v", r, err)
	}
	if _, err := Find(dir, "20261010-100000-a"); !errors.Is(err, ErrAmbiguous) {
		t.Errorf("Find(ambiguous) error = %v", err)
	}
	if _, err := Find(dir, "2025"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Find(unknown) error = %v", err)
	}
}
