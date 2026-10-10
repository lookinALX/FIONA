package sorter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/types"
)

var (
	ErrTargetAppeared = errors.New("destination appeared after planning")
	ErrTargetChanged  = errors.New("destination changed after planning")
)

// Reporter receives the progress of a run. The sorter itself prints nothing.
type Reporter interface {
	Start(total int)
	Done(a types.Action, err error)
	Finish(s Summary)
}

type Summary struct {
	Moved       int
	Copied      int
	Renamed     int
	Replaced    int
	Skipped     int
	Noop        int
	Rejected    int
	Failed      int
	Interrupted bool

	// undo only
	Restored      int // files moved back to source
	Removed       int // copies deleted from dest
	TrashRestored int
	DirsRemoved   int
}

// ExitCode is 1 if any operation failed or the run was interrupted.
func (s Summary) ExitCode() int {
	if s.Failed > 0 || s.Interrupted {
		return 1
	}
	return 0
}

// Executor makes no decisions: it carries out the operations of a ready plan.
type Executor struct {
	plan         *Plan
	jrn          *journal.Writer
	workers      int
	fionaVersion string
}

func NewExecutor(plan *Plan, jrn *journal.Writer, workers int, fionaVersion string) *Executor {
	return &Executor{
		plan:         plan,
		jrn:          jrn,
		workers:      workers,
		fionaVersion: fionaVersion,
	}
}

// Run writes the whole plan to the journal and only then starts touching
// files: no journal, no run. On cancellation workers finish the current file and stop.
func (ex *Executor) Run(ctx context.Context, rep Reporter) (Summary, error) {
	var summary Summary

	err := ex.jrn.Begin(journal.Header{
		RunID:        ex.plan.RunID,
		StartedAt:    time.Now(),
		Source:       ex.plan.BaseDirSource,
		Dest:         ex.plan.BaseDirDest,
		FileAction:   ex.plan.FileAction,
		OnConflict:   ex.plan.OnConflict,
		FionaVersion: ex.fionaVersion,
	}, ex.plan.Mkdirs, ex.plan.Actions)
	if err != nil {
		ex.jrn.Close(journal.Summary{Interrupted: true})
		return summary, err
	}

	var todo []types.Action
	for _, action := range ex.plan.Actions {
		switch action.Kind {
		case types.OpSkip:
			summary.Skipped++
		case types.OpNoop:
			summary.Noop++
		case types.OpReject:
			summary.Rejected++
		default:
			todo = append(todo, action)
		}
	}

	rep.Start(len(todo))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu         sync.Mutex
		wg         sync.WaitGroup
		executed   int
		journalErr error
	)

	jobs := make(chan types.Action)
	for i := 0; i < ex.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for action := range jobs {
				if ctx.Err() != nil {
					continue
				}

				opErr := ex.executeAction(action)

				status := types.StatusOK
				var size int64
				var mtime time.Time
				if opErr != nil {
					status = types.StatusFailed
				} else if info, err := os.Lstat(action.TargetPath); err == nil {
					size, mtime = info.Size(), info.ModTime()
				}
				jrnErr := ex.jrn.Done(action.Seq, status, opErr, size, mtime)

				mu.Lock()
				executed++
				summary.count(action, opErr)
				if jrnErr != nil && journalErr == nil {
					// without a journal the run cannot be undone, so it stops here
					journalErr = jrnErr
					cancel()
				}
				mu.Unlock()

				rep.Done(action, opErr)
			}
		}()
	}

feed:
	for _, action := range todo {
		select {
		case jobs <- action:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)

	wg.Wait()

	summary.Interrupted = executed < len(todo)

	closeErr := ex.jrn.Close(journal.Summary{
		OK:          executed - summary.Failed,
		Failed:      summary.Failed,
		Skipped:     summary.Skipped,
		Interrupted: summary.Interrupted,
	})

	rep.Finish(summary)

	return summary, errors.Join(journalErr, closeErr)
}

func (s *Summary) count(action types.Action, err error) {
	if err != nil {
		s.Failed++
		return
	}

	if action.Kind == types.OpCopy {
		s.Copied++
	} else {
		s.Moved++
	}

	switch action.Conflict {
	case types.ConflictRenamed:
		s.Renamed++
	case types.ConflictReplaced:
		s.Replaced++
	}
}

func (ex *Executor) executeAction(action types.Action) error {
	if err := os.MkdirAll(filepath.Dir(action.TargetPath), 0755); err != nil {
		return err
	}

	trashed := false
	if action.TrashPath != "" {
		var err error
		if trashed, err = moveToTrash(action.TargetPath, action.TrashPath); err != nil {
			return err
		}
	}

	tmp := tempPath(action.TargetPath, ex.plan.RunID, action.Seq)

	var err error
	if action.Kind == types.OpCopy {
		err = copyFileAtomic(action.SourcePath, action.TargetPath, tmp)
	} else {
		err = moveFile(action.SourcePath, action.TargetPath, tmp)
	}

	if err != nil && trashed {
		// put the displaced file back, the operation did not take its place
		if guard(action.TargetPath) == nil {
			os.Rename(action.TrashPath, action.TargetPath)
		}
	}

	return err
}

// moveToTrash moves the file being replaced out of the way. The trash lives
// inside dest, so this is an atomic rename on the same file system.
func moveToTrash(target, trash string) (bool, error) {
	info, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, ErrTargetChanged
	}

	if err := os.MkdirAll(filepath.Dir(trash), 0755); err != nil {
		return false, err
	}
	if err := guard(trash); err != nil {
		return false, err
	}
	if err := os.Rename(target, trash); err != nil {
		return false, err
	}

	return true, nil
}

// guard fails if something occupies path. Lstat, so that a symlink counts too.
func guard(path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return fmt.Errorf("%w: %s", ErrTargetAppeared, path)
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func tempPath(target, runID string, seq int) string {
	return filepath.Join(filepath.Dir(target), fmt.Sprintf(".fiona-%s-%d.part", runID, seq))
}

// moveFile renames source to destination, which must be free. Across file
// systems it falls back to an atomic copy followed by removing the source.
func moveFile(source, destination, tmp string) error {
	if err := guard(destination); err != nil {
		return err
	}

	err := os.Rename(source, destination)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}

	if err = copyFileAtomic(source, destination, tmp); err != nil {
		return err
	}
	if err = os.Remove(source); err != nil {
		// the source stays where it was, so the copy must not stay too
		os.Remove(destination)
		return err
	}

	return nil
}

// copyFileAtomic writes the copy to a temporary file next to destination and
// renames it to the final name only when it is complete. Nothing is ever
// written through an existing file or symlink.
func copyFileAtomic(source, destination, tmp string) (err error) {
	if err := guard(destination); err != nil {
		return err
	}

	srcFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("source is not a regular file: %s", source)
	}

	tmpFile, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmpFile.Close()
			os.Remove(tmp)
		}
	}()

	if _, err = io.Copy(tmpFile, srcFile); err != nil {
		return err
	}
	if err = tmpFile.Sync(); err != nil {
		return err
	}
	if err = tmpFile.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp, srcInfo.Mode()); err != nil {
		return err
	}
	if err = os.Chtimes(tmp, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		return err
	}
	if err = guard(destination); err != nil {
		return err
	}

	return os.Rename(tmp, destination)
}
