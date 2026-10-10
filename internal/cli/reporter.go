package cli

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/lookinalx/fiona/internal/sorter"
	"github.com/lookinalx/fiona/internal/types"

	"github.com/schollz/progressbar/v3"
)

const separator = "================================================"

// Reporter shows the progress of a run in the terminal. It implements sorter.Reporter.
type Reporter struct {
	out  io.Writer
	undo bool

	mu       sync.Mutex
	bar      *progressbar.ProgressBar
	problems []string
}

func NewSortReporter(out io.Writer) *Reporter {
	return &Reporter{out: out}
}

func NewUndoReporter(out io.Writer) *Reporter {
	return &Reporter{out: out, undo: true}
}

func (r *Reporter) Start(total int) {
	fmt.Fprintln(r.out, separator)
	if r.undo {
		fmt.Fprintln(r.out, "Starting undo process...")
	} else {
		fmt.Fprintln(r.out, "Starting execution...")
	}

	if total > 0 {
		r.bar = progressbar.Default(int64(total))
	}
}

func (r *Reporter) Done(a types.Action, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err != nil {
		path := a.SourcePath
		if r.undo {
			path = a.TargetPath
		}
		mark := "✗"
		if errors.Is(err, sorter.ErrUndoSkipped) {
			mark = "⊘"
		}
		r.problems = append(r.problems, fmt.Sprintf("%s %s: %v", mark, path, err))
	}

	if r.bar != nil {
		r.bar.Add(1)
	}
}

func (r *Reporter) Finish(s sorter.Summary) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.bar != nil && !s.Interrupted {
		r.bar.Finish()
	}
	fmt.Fprintln(r.out)

	for _, problem := range r.problems {
		fmt.Fprintln(r.out, problem)
	}

	if r.undo {
		r.finishUndo(s)
	} else {
		r.finishSort(s)
	}
}

func (r *Reporter) finishSort(s sorter.Summary) {
	if s.Interrupted {
		fmt.Fprintln(r.out, "Execution is interrupted.")
	} else {
		fmt.Fprintln(r.out, "Execution is ended.")
	}

	fmt.Fprintf(r.out, "  moved: %d\n", s.Moved)
	fmt.Fprintf(r.out, "  copied: %d\n", s.Copied)
	fmt.Fprintf(r.out, "  renamed: %d\n", s.Renamed)
	fmt.Fprintf(r.out, "  replaced: %d\n", s.Replaced)
	fmt.Fprintf(r.out, "  skipped: %d\n", s.Skipped)
	fmt.Fprintf(r.out, "  already in place: %d\n", s.Noop)
	fmt.Fprintf(r.out, "  rejected: %d\n", s.Rejected)
	fmt.Fprintf(r.out, "  errors: %d\n", s.Failed)
}

func (r *Reporter) finishUndo(s sorter.Summary) {
	if s.Interrupted {
		fmt.Fprintln(r.out, "Undo process is interrupted.")
	} else {
		fmt.Fprintln(r.out, "Undo process is ended.")
	}

	fmt.Fprintf(r.out, "  moved back: %d\n", s.Restored)
	fmt.Fprintf(r.out, "  copies removed: %d\n", s.Removed)
	fmt.Fprintf(r.out, "  restored from trash: %d\n", s.TrashRestored)
	fmt.Fprintf(r.out, "  empty directories removed: %d\n", s.DirsRemoved)
	fmt.Fprintf(r.out, "  skipped: %d\n", s.Skipped)
	fmt.Fprintf(r.out, "  errors: %d\n", s.Failed)
}
