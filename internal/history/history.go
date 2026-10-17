// Package history keeps a global index of runs, so that a run can be undone
// without knowing where its journal is. The state of a run is read from the
// journal itself; the index does not duplicate it.
package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lookinalx/fiona/internal/journal"
)

const FileName = "history.jsonl"

var (
	ErrNotFound  = errors.New("no such run in the history")
	ErrAmbiguous = errors.New("run id is ambiguous")
)

type Entry struct {
	RunID     string    `json:"run_id"`
	StartedAt time.Time `json:"started_at"`
	Source    string    `json:"source"`
	Dest      string    `json:"dest"`
	Journal   string    `json:"journal"`
	Action    string    `json:"action"`
	Planned   int       `json:"planned"`
}

type State string

const (
	StateDone        State = "done"
	StateFailed      State = "done with errors"
	StateInterrupted State = "interrupted"
	StateUndone      State = "undone"
	StateMissing     State = "journal missing"
	StateBroken      State = "journal unreadable"
)

// Record is an entry with the state read from its journal.
type Record struct {
	Entry
	State State
	Run   *journal.Run // nil if the journal cannot be read
}

// Undoable reports whether the run can still be undone.
func (r Record) Undoable() bool {
	return r.Run != nil && r.Run.UndoneAt == nil
}

// Append adds a line to the index. The file is readable by the owner only:
// it lists the folders the user sorts.
func Append(dir string, e Entry) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(filepath.Join(dir, FileName), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Read returns the entries, newest first. Lines that cannot be parsed are skipped.
func Read(dir string) ([]Entry, error) {
	f, err := os.Open(filepath.Join(dir, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.RunID == "" || e.Journal == "" {
			continue
		}
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(entries, func(i, j int) bool { return entries[i].StartedAt.After(entries[j].StartedAt) })
	return entries, nil
}

// Records reads the entries and the state of each run, newest first.
func Records(dir string) ([]Record, error) {
	entries, err := Read(dir)
	if err != nil {
		return nil, err
	}
	records := make([]Record, len(entries))
	for i, e := range entries {
		records[i] = load(e)
	}
	return records, nil
}

func load(e Entry) Record {
	r := Record{Entry: e}

	run, err := journal.Read(e.Journal)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		r.State = StateMissing
		return r
	case err != nil || run.Header.RunID != e.RunID:
		r.State = StateBroken
		return r
	}
	r.Run = run

	switch {
	case run.UndoneAt != nil:
		r.State = StateUndone
	case run.Footer == nil:
		r.State = StateInterrupted
	case run.Footer.Failed > 0:
		r.State = StateFailed
	default:
		r.State = StateDone
	}
	return r
}

// LatestUndoable is the newest run that is not undone and whose journal can be read.
func LatestUndoable(dir string) (*Record, error) {
	records, err := Records(dir)
	if err != nil {
		return nil, err
	}
	for i := range records {
		if records[i].Undoable() {
			return &records[i], nil
		}
	}
	return nil, nil
}

// Find looks a run up by a prefix of its id.
func Find(dir, prefix string) (*Record, error) {
	entries, err := Read(dir)
	if err != nil {
		return nil, err
	}

	var matches []Entry
	for _, e := range entries {
		if e.RunID == prefix {
			matches = []Entry{e}
			break
		}
		if strings.HasPrefix(e.RunID, prefix) {
			matches = append(matches, e)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("%w: %s", ErrNotFound, prefix)
	case 1:
		r := load(matches[0])
		return &r, nil
	default:
		ids := make([]string, len(matches))
		for i, m := range matches {
			ids[i] = m.RunID
		}
		return nil, fmt.Errorf("%w: %s matches %s", ErrAmbiguous, prefix, strings.Join(ids, ", "))
	}
}
