package cli

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"

	"github.com/lookinalx/fiona/internal/report"
	"github.com/lookinalx/fiona/internal/sorter"
	"github.com/lookinalx/fiona/internal/types"
)

// progressDelay keeps quick runs free of a flashing progress bar.
const progressDelay = 500 * time.Millisecond

// Reporter shows a progress bar in a terminal and collects the files that
// failed. It implements sorter.Reporter; the result itself is printed by the caller.
type Reporter struct {
	out        io.Writer
	terminal   bool
	undo       bool
	root       string // problems are shown relative to it
	totalBytes int64  // progress in bytes instead of files, for copies

	mu       sync.Mutex
	start    time.Time
	total    int
	files    int
	bytes    int64
	bar      *progressbar.ProgressBar
	problems []report.Problem
}

// NewSortReporter shows progress in files, or in bytes when totalBytes is set.
func NewSortReporter(out io.Writer, terminal bool, source string, totalBytes int64) *Reporter {
	return &Reporter{out: out, terminal: terminal, root: source, totalBytes: totalBytes}
}

func NewUndoReporter(out io.Writer, terminal bool, dest string) *Reporter {
	return &Reporter{out: out, terminal: terminal, undo: true, root: dest}
}

func (r *Reporter) Start(total int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.start = time.Now()
	r.total = total
}

func (r *Reporter) Done(a types.Action, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err != nil {
		path := a.SourcePath
		if r.undo {
			path = a.TargetPath
		}
		r.problems = append(r.problems, report.Problem{
			Path:    report.ProblemPath(r.root, path),
			Err:     err.Error(),
			Skipped: errors.Is(err, sorter.ErrUndoSkipped),
		})
	}

	r.files++
	if err == nil {
		r.bytes += a.Size
	}

	if r.bar == nil && r.terminal && r.total > 1 && time.Since(r.start) > progressDelay {
		r.bar = r.newBar()
	}
	if r.bar != nil {
		if r.totalBytes > 0 {
			r.bar.Set64(r.bytes)
		} else {
			r.bar.Set(r.files)
		}
	}
}

func (r *Reporter) newBar() *progressbar.ProgressBar {
	max := int64(r.total)
	if r.totalBytes > 0 {
		max = r.totalBytes
	}
	return progressbar.NewOptions64(max,
		progressbar.OptionSetWriter(r.out),
		progressbar.OptionSetWidth(30),
		progressbar.OptionShowCount(),
		progressbar.OptionShowBytes(r.totalBytes > 0),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionClearOnFinish(),
	)
}

func (r *Reporter) Finish(sorter.Summary) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.bar != nil {
		r.bar.Clear()
		r.bar = nil
	}
}

// Problems are the files that failed or were skipped, in the order they happened.
func (r *Reporter) Problems() []report.Problem {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]report.Problem(nil), r.problems...)
}
