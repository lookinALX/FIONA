package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/lookinalx/fiona/internal/history"
	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/report"
	"github.com/lookinalx/fiona/internal/sorter"
)

// UndoTarget says which run to undo. At most one of LogPath, Dest and RunID is set;
// with none of them the newest run of the history is taken.
type UndoTarget struct {
	LogPath    string // the journal itself
	Dest       string // the newest run of this destination
	RunID      string // a prefix of the run id, looked up in the history
	HistoryDir string
	Cwd        string // runs of the current folder, when the history has none
}

// NothingToUndoError is not a failure: every run is undone already.
type NothingToUndoError struct {
	Where string       // the destination looked at; empty for the whole history
	Last  *journal.Run // the newest run, already undone; nil if there are no runs
	Now   time.Time
}

func (e *NothingToUndoError) Error() string {
	if e.Last == nil {
		if e.Where != "" {
			return "Nothing to undo: no runs in " + report.Path(e.Where) + "."
		}
		return "Nothing to undo: no runs yet."
	}
	h := e.Last.Header
	return fmt.Sprintf("Nothing to undo: the last run in %s (%s) is already undone. See fiona history.",
		report.Path(h.Dest), report.When(h.StartedAt, e.Now))
}

// FindRun reads the journal of the run to undo.
func FindRun(t UndoTarget) (*journal.Run, error) {
	now := time.Now()

	switch {
	case t.LogPath != "":
		return journal.Read(t.LogPath)

	case t.RunID != "":
		rec, err := history.Find(t.HistoryDir, t.RunID)
		if err != nil {
			return nil, err
		}
		if rec.Run == nil {
			return nil, fmt.Errorf("run %s: %s: %s", rec.RunID, rec.State, rec.Journal)
		}
		return rec.Run, nil

	case t.Dest != "":
		return latestIn(t.Dest, now)
	}

	rec, err := history.LatestUndoable(t.HistoryDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read the history: %w", err)
	}
	if rec != nil {
		return rec.Run, nil
	}

	// runs made before the history existed, or while it could not be written
	if t.Cwd != "" {
		run, err := latestIn(t.Cwd, now)
		var nothing *NothingToUndoError
		if !errors.As(err, &nothing) {
			return run, err
		}
	}

	nothing := &NothingToUndoError{Now: now}
	if records, err := history.Records(t.HistoryDir); err == nil {
		for _, r := range records {
			if r.Run != nil {
				nothing.Last = r.Run
				break
			}
		}
	}
	return nil, nothing
}

// latestIn finds the newest run of dest that is not undone.
func latestIn(dest string, now time.Time) (*journal.Run, error) {
	run, err := journal.FindLatest(filepath.Join(dest, journal.StateDir, journal.RunsDir))
	var noRuns *journal.NoRunsError
	if errors.As(err, &noRuns) {
		return nil, &NothingToUndoError{Where: dest, Last: noRuns.Last, Now: now}
	}
	if err != nil {
		return nil, err
	}
	if run.Header.Dest != dest {
		return nil, fmt.Errorf("%w: %s belongs to dest %q, not to %q",
			journal.ErrRejected, filepath.Base(run.Path), run.Header.Dest, dest)
	}
	return run, nil
}

// ExecuteUndo reverts the run. The journal is marked as undone only after a
// pass without errors, so a failed undo can be repeated.
func ExecuteUndo(ctx context.Context, plan *sorter.UndoPlan, workers int, rep sorter.Reporter) (sorter.Summary, error) {
	return sorter.NewReverter(plan, workers).Run(ctx, rep)
}
