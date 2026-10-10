package journal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lookinalx/fiona/internal/types"
)

var (
	ErrOldFormat      = errors.New("journal has the old format (fiona_logs.json): it does not record final file paths, so a safe undo is impossible")
	ErrUnknownVersion = errors.New("unknown journal version")
	ErrCorrupt        = errors.New("journal is corrupt")
	ErrRejected       = errors.New("journal rejected")
	ErrNoRuns         = errors.New("no runs to undo")
)

// Run is a journal read back from disk.
type Run struct {
	Path     string
	Header   Header
	Mkdirs   []string
	Ops      []types.LogEntry // ordered by seq
	Footer   *Footer          // nil means the run was interrupted
	UndoneAt *time.Time       // set if the run has already been undone
}

// Read parses a journal file. A torn last line is ignored.
func Read(path string) (*Run, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read journal file: %w", err)
	}

	lines := bytes.Split(data, []byte{'\n'})
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}

	var first anyRecord
	if len(lines) == 0 || json.Unmarshal(lines[0], &first) != nil || first.Type != typeHeader {
		if isOldFormat(data) {
			return nil, ErrOldFormat
		}
		return nil, fmt.Errorf("%w: no header", ErrCorrupt)
	}
	if first.Version == nil || *first.Version != Version {
		return nil, fmt.Errorf("%w: %v", ErrUnknownVersion, derefInt(first.Version))
	}

	run := &Run{
		Path: path,
		Header: Header{
			RunID:        first.RunID,
			StartedAt:    derefTime(first.StartedAt),
			Source:       first.Source,
			Dest:         first.Dest,
			FileAction:   first.FileAction,
			OnConflict:   first.OnConflict,
			FionaVersion: first.FionaVersion,
		},
	}

	bySeq := make(map[int]int)
	torn := false

	for i, line := range lines[1:] {
		lineNo := i + 2

		var rec anyRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			if torn {
				return nil, fmt.Errorf("%w: line %d", ErrCorrupt, lineNo)
			}
			torn = true
			continue
		}
		// only the undo mark may follow a torn line
		if torn && rec.Type != typeUndone {
			return nil, fmt.Errorf("%w: line %d", ErrCorrupt, lineNo-1)
		}

		switch rec.Type {
		case typeMkdir:
			run.Mkdirs = append(run.Mkdirs, rec.Path)

		case typeOp:
			if rec.Seq == nil {
				return nil, fmt.Errorf("%w: line %d: op without seq", ErrCorrupt, lineNo)
			}
			if _, exists := bySeq[*rec.Seq]; exists {
				return nil, fmt.Errorf("%w: line %d: duplicate op %d", ErrCorrupt, lineNo, *rec.Seq)
			}
			bySeq[*rec.Seq] = len(run.Ops)
			run.Ops = append(run.Ops, types.LogEntry{Action: types.Action{
				Seq:        *rec.Seq,
				Kind:       types.OpKind(rec.Kind),
				SourcePath: rec.Src,
				TargetPath: rec.Dst,
				Size:       derefInt64(rec.Size),
				ModTime:    derefTime(rec.Mtime),
				Conflict:   rec.Conflict,
				TrashPath:  rec.Trash,
				Note:       rec.Note,
			}})

		case typeDone:
			if rec.Seq == nil {
				return nil, fmt.Errorf("%w: line %d: done without seq", ErrCorrupt, lineNo)
			}
			idx, exists := bySeq[*rec.Seq]
			if !exists {
				return nil, fmt.Errorf("%w: line %d: done for unknown op %d", ErrCorrupt, lineNo, *rec.Seq)
			}
			if run.Ops[idx].Result != nil {
				return nil, fmt.Errorf("%w: line %d: duplicate done for op %d", ErrCorrupt, lineNo, *rec.Seq)
			}
			if rec.Status != types.StatusOK && rec.Status != types.StatusFailed {
				return nil, fmt.Errorf("%w: line %d: unknown status %q", ErrCorrupt, lineNo, rec.Status)
			}
			run.Ops[idx].Result = &types.LogResult{
				Status:  rec.Status,
				Error:   rec.Error,
				Size:    derefInt64(rec.Size),
				ModTime: derefTime(rec.Mtime),
			}

		case typeFooter:
			run.Footer = &Footer{
				FinishedAt: derefTime(rec.FinishedAt),
				OK:         rec.OK,
				Failed:     rec.Failed,
				Skipped:    rec.Skipped,
			}

		case typeUndone:
			at := derefTime(rec.At)
			run.UndoneAt = &at

		default:
			return nil, fmt.Errorf("%w: line %d: unknown record type %q", ErrCorrupt, lineNo, rec.Type)
		}
	}

	sort.SliceStable(run.Ops, func(i, j int) bool { return run.Ops[i].Seq < run.Ops[j].Seq })

	return run, nil
}

// Validate checks that the journal cannot make undo touch anything outside
// of its own source and dest. A journal breaking any rule is rejected as a whole.
func (r *Run) Validate() error {
	if !isCleanAbs(r.Header.Source) {
		return fmt.Errorf("%w: source %q is not an absolute path", ErrRejected, r.Header.Source)
	}
	if !isCleanAbs(r.Header.Dest) {
		return fmt.Errorf("%w: dest %q is not an absolute path", ErrRejected, r.Header.Dest)
	}

	for _, dir := range r.Mkdirs {
		if !isInside(r.Header.Dest, dir) {
			return fmt.Errorf("%w: mkdir %q is outside of dest %q", ErrRejected, dir, r.Header.Dest)
		}
	}

	for _, op := range r.Ops {
		switch op.Kind {
		case types.OpMove, types.OpCopy, types.OpSkip, types.OpNoop, types.OpReject:
		default:
			return fmt.Errorf("%w: op %d: unknown kind %q", ErrRejected, op.Seq, op.Kind)
		}
		if !isInside(r.Header.Source, op.SourcePath) {
			return fmt.Errorf("%w: op %d: src %q is outside of source %q", ErrRejected, op.Seq, op.SourcePath, r.Header.Source)
		}
		// a rejected operation may have no target at all
		if (op.TargetPath != "" || op.Kind != types.OpReject) && !isInside(r.Header.Dest, op.TargetPath) {
			return fmt.Errorf("%w: op %d: dst %q is outside of dest %q", ErrRejected, op.Seq, op.TargetPath, r.Header.Dest)
		}
		if op.TrashPath != "" && !isInside(r.Header.Dest, op.TrashPath) {
			return fmt.Errorf("%w: op %d: trash %q is outside of dest %q", ErrRejected, op.Seq, op.TrashPath, r.Header.Dest)
		}
	}

	return nil
}

// FindLatest returns the most recent run in dir that has not been undone yet.
func FindLatest(dir string) (*Run, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w in %s", ErrNoRuns, dir)
		}
		return nil, fmt.Errorf("failed to read journal directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), fileExt) {
			names = append(names, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))

	for _, name := range names {
		run, err := Read(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if run.UndoneAt == nil {
			return run, nil
		}
	}

	if len(names) > 0 {
		lastRun := strings.TrimSuffix(names[0], fileExt)
		return nil, fmt.Errorf("%w in %s: the last run %s is already undone", ErrNoRuns, dir, lastRun)
	}
	return nil, fmt.Errorf("%w in %s", ErrNoRuns, dir)
}

func isOldFormat(data []byte) bool {
	var old struct {
		Logs json.RawMessage `json:"logs"`
	}
	return json.Unmarshal(data, &old) == nil && old.Logs != nil
}

func isCleanAbs(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path
}

// isInside reports whether path lies strictly inside root. Paths that are not
// clean are refused: with symlinks, ".." may lead somewhere else than it reads.
func isInside(root, path string) bool {
	if !isCleanAbs(path) {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && filepath.IsLocal(rel)
}

func derefInt(p *int) any {
	if p == nil {
		return "missing"
	}
	return *p
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func derefTime(p *time.Time) time.Time {
	if p == nil {
		return time.Time{}
	}
	return *p
}
