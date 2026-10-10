package sorter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/types"
)

// mtimeTolerance covers file systems with a coarse modification time.
const mtimeTolerance = 2 * time.Second

const (
	noteChanged       = "changed after the run"
	noteMissing       = "file is missing"
	noteNoAccess      = "file is not accessible"
	noteSourceTaken   = "original location is occupied"
	noteUnsafeDirPath = "path goes through something that is not a directory"
)

var (
	ErrAlreadyUndone = errors.New("already undone")
	// ErrUndoSkipped marks a file that failed verification and was left untouched.
	ErrUndoSkipped = errors.New("skipped")
)

type UndoStats struct {
	MoveBack     int
	RemoveCopy   int
	TrashRestore int
	EmptyDirs    int
	Skipped      int
}

type UndoPlan struct {
	Run     *journal.Run
	Actions []types.UndoAction // in reverse order of seq
	Stats   UndoStats
}

// NewUndoPlan validates the journal and decides, without touching the disk,
// what an undo of the run would do.
func NewUndoPlan(run *journal.Run) (*UndoPlan, error) {
	if run.UndoneAt != nil {
		return nil, fmt.Errorf("run %s is %w", run.Header.RunID, ErrAlreadyUndone)
	}
	if err := run.Validate(); err != nil {
		return nil, err
	}

	plan := &UndoPlan{Run: run}

	// paths that will be gone after the undo, to predict which directories become empty
	gone := make(map[string]struct{})

	for i := len(run.Ops) - 1; i >= 0; i-- {
		action := decideUndo(run, run.Ops[i])

		switch action.Kind {
		case types.UndoMoveBack:
			plan.Stats.MoveBack++
		case types.UndoRemoveCopy:
			plan.Stats.RemoveCopy++
		case types.UndoSkip:
			plan.Stats.Skipped++
		}

		if action.RestoreTrash {
			plan.Stats.TrashRestore++
			gone[action.Op.TrashPath] = struct{}{}
		} else if action.Kind == types.UndoMoveBack || action.Kind == types.UndoRemoveCopy {
			gone[action.Op.TargetPath] = struct{}{}
		}

		if action.Kind != types.UndoNothing || action.RestoreTrash {
			plan.Actions = append(plan.Actions, action)
		}
	}

	for _, dir := range deepestFirst(run.Mkdirs) {
		if willBeEmpty(dir, gone) {
			plan.Stats.EmptyDirs++
			gone[dir] = struct{}{}
		}
	}

	return plan, nil
}

// decideUndo looks at the disk and decides how one operation is reverted.
func decideUndo(run *journal.Run, op types.LogEntry) types.UndoAction {
	action := types.UndoAction{Seq: op.Seq, Kind: types.UndoNothing, Op: op.Action}

	if !op.Executable() {
		return action
	}

	if !hasOnlyDirsBetween(run.Header.Dest, op.TargetPath) || !hasOnlyDirsBetween(run.Header.Source, op.SourcePath) {
		action.Kind = types.UndoSkip
		action.Note = noteUnsafeDirPath
		return action
	}

	target, targetErr := os.Lstat(op.TargetPath)
	_, sourceErr := os.Lstat(op.SourcePath)
	targetExists := targetErr == nil
	targetFree := errors.Is(targetErr, fs.ErrNotExist)
	sourceFree := errors.Is(sourceErr, fs.ErrNotExist)

	trashExists := false
	if op.TrashPath != "" && hasOnlyDirsBetween(run.Header.Dest, op.TrashPath) {
		_, err := os.Lstat(op.TrashPath)
		trashExists = err == nil
	}

	switch {
	case op.Result == nil:
		// the run was interrupted before this operation reported its result
		sizeMatches := targetExists && target.Mode().IsRegular() && target.Size() == op.Size
		switch {
		case op.Kind == types.OpMove && sizeMatches && sourceFree:
			action.Kind = types.UndoMoveBack
		// without the displaced file in trash the target is still the old file, not our copy
		case op.Kind == types.OpCopy && sizeMatches && (op.TrashPath == "" || trashExists):
			action.Kind = types.UndoRemoveCopy
		}

	case op.Result.Status != types.StatusOK:
		// a failed operation left the source in place

	case targetFree:
		// already reverted by an earlier, interrupted undo
		if op.Kind == types.OpMove && sourceFree {
			action.Kind = types.UndoSkip
			action.Note = noteMissing
		}

	case !targetExists:
		action.Kind = types.UndoSkip
		action.Note = noteNoAccess

	case !target.Mode().IsRegular() || target.Size() != op.Result.Size || !sameMtime(target.ModTime(), op.Result.ModTime):
		action.Kind = types.UndoSkip
		action.Note = noteChanged

	case op.Kind == types.OpCopy:
		action.Kind = types.UndoRemoveCopy

	case !sourceFree:
		action.Kind = types.UndoSkip
		action.Note = noteSourceTaken

	default:
		action.Kind = types.UndoMoveBack
	}

	// the displaced file returns to the target if the target ends up free
	if trashExists {
		targetFreed := action.Kind == types.UndoMoveBack || action.Kind == types.UndoRemoveCopy
		action.RestoreTrash = targetFreed || targetFree
	}

	return action
}

func sameMtime(a, b time.Time) bool {
	diff := a.Sub(b)
	return diff >= -mtimeTolerance && diff <= mtimeTolerance
}

// hasOnlyDirsBetween reports whether every existing path component between
// root and path is a real directory, not a symlink leading somewhere else.
func hasOnlyDirsBetween(root, path string) bool {
	rel, err := filepath.Rel(root, filepath.Dir(path))
	if err != nil || rel == "." {
		return err == nil
	}
	if !filepath.IsLocal(rel) {
		return false
	}

	dir := root
	for _, component := range strings.Split(rel, string(filepath.Separator)) {
		dir = filepath.Join(dir, component)
		info, err := os.Lstat(dir)
		if errors.Is(err, fs.ErrNotExist) {
			return true
		}
		if err != nil || !info.IsDir() {
			return false
		}
	}

	return true
}

func deepestFirst(dirs []string) []string {
	sorted := make([]string, len(dirs))
	copy(sorted, dirs)
	sort.SliceStable(sorted, func(i, j int) bool {
		di := strings.Count(sorted[i], string(filepath.Separator))
		dj := strings.Count(sorted[j], string(filepath.Separator))
		return di > dj
	})
	return sorted
}

func willBeEmpty(dir string, gone map[string]struct{}) bool {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if _, ok := gone[filepath.Join(dir, entry.Name())]; !ok {
			return false
		}
	}
	return true
}

// Print shows what the undo is going to do, before the user confirms it.
func (p *UndoPlan) Print(w io.Writer) {
	header := p.Run.Header

	fmt.Fprintf(w, "Undo of run %s (%s, %s)\n",
		header.RunID, header.FileAction, header.StartedAt.Local().Format("02.01.2006 15:04"))
	if p.Run.Footer == nil {
		fmt.Fprintln(w, "  The run was interrupted: only operations that really happened are reverted.")
	}

	if header.FileAction == string(types.OpCopy) {
		fmt.Fprintf(w, "  remove copies from %s: %d %s\n", header.Dest, p.Stats.RemoveCopy, pluralize(p.Stats.RemoveCopy))
	} else {
		fmt.Fprintf(w, "  move back to %s: %d %s\n", header.Source, p.Stats.MoveBack, pluralize(p.Stats.MoveBack))
	}
	fmt.Fprintf(w, "  restore from trash: %d\n", p.Stats.TrashRestore)
	fmt.Fprintf(w, "  remove empty directories: %d\n", p.Stats.EmptyDirs)
	fmt.Fprintf(w, "  skip (changed after the run): %d\n", p.Stats.Skipped)

	for _, action := range p.Actions {
		if action.Kind == types.UndoSkip {
			fmt.Fprintf(w, "    %s: %s\n", action.Op.TargetPath, action.Note)
		}
	}
}

type Reverter struct {
	plan    *UndoPlan
	workers int
	entries map[int]types.LogEntry // by seq
}

func NewReverter(plan *UndoPlan, workers int) *Reverter {
	entries := make(map[int]types.LogEntry, len(plan.Run.Ops))
	for _, op := range plan.Run.Ops {
		entries[op.Seq] = op
	}

	return &Reverter{
		plan:    plan,
		workers: workers,
		entries: entries,
	}
}

// Run reverts the operations of the run. Every file is verified again right
// before it is touched; nothing is ever overwritten. The journal is marked as
// undone only after a pass without errors, so a failed undo can be repeated.
func (r *Reverter) Run(ctx context.Context, rep Reporter) (Summary, error) {
	var summary Summary
	run := r.plan.Run

	rep.Start(len(r.plan.Actions))

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		executed int
	)

	jobs := make(chan types.UndoAction)
	for i := 0; i < r.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for planned := range jobs {
				if ctx.Err() != nil {
					continue
				}

				result, err := r.revert(planned)

				mu.Lock()
				executed++
				summary.Restored += result.Restored
				summary.Removed += result.Removed
				summary.TrashRestored += result.TrashRestored
				switch {
				case errors.Is(err, ErrUndoSkipped):
					summary.Skipped++
				case err != nil:
					summary.Failed++
				}
				mu.Unlock()

				rep.Done(planned.Op, err)
			}
		}()
	}

feed:
	for _, action := range r.plan.Actions {
		select {
		case jobs <- action:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)

	wg.Wait()

	summary.Interrupted = executed < len(r.plan.Actions)
	if summary.Interrupted {
		rep.Finish(summary)
		return summary, nil
	}

	r.removeTempFiles()
	summary.DirsRemoved = removeEmptyDirs(run.Header.Dest, run.Mkdirs)

	var err error
	if summary.Failed == 0 {
		err = journal.MarkUndone(run.Path, time.Now())
	}

	rep.Finish(summary)

	return summary, err
}

func (r *Reverter) revert(planned types.UndoAction) (Summary, error) {
	var result Summary
	run := r.plan.Run

	// the disk may have changed since the preview
	action := decideUndo(run, r.entries[planned.Seq])
	op := action.Op

	var err error
	switch action.Kind {
	case types.UndoMoveBack:
		if err = os.MkdirAll(filepath.Dir(op.SourcePath), 0755); err == nil {
			err = moveFile(op.TargetPath, op.SourcePath, undoTempPath(op.SourcePath, run.Header.RunID, op.Seq))
		}
		if err == nil {
			result.Restored++
		}

	case types.UndoRemoveCopy:
		if err = os.Remove(op.TargetPath); err == nil {
			result.Removed++
		}

	case types.UndoSkip:
		err = fmt.Errorf("%w: %s", ErrUndoSkipped, action.Note)
	}

	if action.RestoreTrash && guard(op.TargetPath) == nil {
		if trashErr := os.Rename(op.TrashPath, op.TargetPath); trashErr != nil {
			err = errors.Join(err, trashErr)
		} else {
			result.TrashRestored++
		}
	}

	return result, err
}

func undoTempPath(path, runID string, seq int) string {
	return filepath.Join(filepath.Dir(path), fmt.Sprintf(".fiona-%s-undo-%d.part", runID, seq))
}

// removeTempFiles deletes .fiona-<run-id>-*.part files left by a killed run.
func (r *Reverter) removeTempFiles() {
	run := r.plan.Run
	prefix := fmt.Sprintf(".fiona-%s-", run.Header.RunID)

	dirs := make(map[string]struct{})
	for _, op := range run.Ops {
		if !op.Executable() {
			continue
		}
		if hasOnlyDirsBetween(run.Header.Dest, op.TargetPath) {
			dirs[filepath.Dir(op.TargetPath)] = struct{}{}
		}
		if hasOnlyDirsBetween(run.Header.Source, op.SourcePath) {
			dirs[filepath.Dir(op.SourcePath)] = struct{}{}
		}
	}

	for dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.Type().IsRegular() && strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".part") {
				os.Remove(filepath.Join(dir, name))
			}
		}
	}
}

// removeEmptyDirs removes directories created by the run if they are empty,
// deepest first. Dest itself and everything above it is never in the list.
func removeEmptyDirs(dest string, dirs []string) int {
	removed := 0
	for _, dir := range deepestFirst(dirs) {
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || !hasOnlyDirsBetween(dest, dir) {
			continue
		}
		// os.Remove refuses to delete a directory that is not empty
		if os.Remove(dir) == nil {
			removed++
		}
	}
	return removed
}
