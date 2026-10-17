package tui

import (
	"context"
	"errors"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lookinalx/fiona/internal/app"
	"github.com/lookinalx/fiona/internal/history"
	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/report"
	"github.com/lookinalx/fiona/internal/rules"
	"github.com/lookinalx/fiona/internal/scanner"
	"github.com/lookinalx/fiona/internal/sorter"
	"github.com/lookinalx/fiona/internal/types"
)

// Background work runs in commands; the results come back as messages.
// Scans, plans and classification carry the scan sequence number, so that a
// result for a folder that is no longer selected is dropped.

type debounceMsg struct{ seq int }

type scanDoneMsg struct {
	seq    int
	result *scanner.Result
	err    error
}

type planDoneMsg struct {
	scanSeq int
	plan    *sorter.Plan
	ml      int // files that wait for the classifier
}

type classifyDoneMsg struct {
	scanSeq int
	err     error
}

type tickMsg struct{}

type runDoneMsg struct {
	undo     bool
	plan     *sorter.Plan
	undoPlan *sorter.UndoPlan
	outcome  app.Outcome
	summary  sorter.Summary
	problems []report.Problem
	err      error
}

type historyMsg struct {
	records []history.Record
	err     error
}

type undoReadyMsg struct {
	plan *sorter.UndoPlan
	err  error
}

// tickInterval is how often the progress of a run is redrawn.
const tickInterval = 100 * time.Millisecond

func scanCmd(ctx context.Context, seq int, req app.Request) tea.Cmd {
	return func() tea.Msg {
		res, err := app.ScanContext(ctx, req)
		if err == nil {
			// categories may read the files: better here than while planning
			for _, fi := range res.Files {
				if ctx.Err() != nil {
					break
				}
				fi.Category()
			}
		}
		return scanDoneMsg{seq: seq, result: res, err: err}
	}
}

func planCmd(scanSeq int, scanned *scanner.Result, req app.Request) tea.Cmd {
	return func() tea.Msg {
		plan := app.Plan(scanned, req, journal.NewRunID(time.Now()))
		return planDoneMsg{scanSeq: scanSeq, plan: plan, ml: len(app.NeedsML(scanned, req.Rules))}
	}
}

func classifyCmd(scanSeq int, files []*scanner.FileInfo, c app.Classifier) tea.Cmd {
	return func() tea.Msg {
		return classifyDoneMsg{scanSeq: scanSeq, err: app.Classify(context.Background(), files, c)}
	}
}

func tick() tea.Cmd {
	return tea.Tick(tickInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

func runCmd(ctx context.Context, plan *sorter.Plan, opts app.ExecOptions, rep *progress) tea.Cmd {
	return func() tea.Msg {
		out, err := app.Execute(ctx, plan, opts, rep)
		return runDoneMsg{plan: plan, outcome: out, summary: out.Summary, problems: rep.Problems(), err: err}
	}
}

func undoCmd(ctx context.Context, plan *sorter.UndoPlan, workers int, rep *progress) tea.Cmd {
	return func() tea.Msg {
		summary, err := app.ExecuteUndo(ctx, plan, workers, rep)
		return runDoneMsg{undo: true, undoPlan: plan, summary: summary, problems: rep.Problems(), err: err}
	}
}

func historyCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		records, err := history.Records(dir)
		return historyMsg{records: records, err: err}
	}
}

// undoPlanCmd reads a journal and decides what its undo would do.
func undoPlanCmd(run *journal.Run, path string) tea.Cmd {
	return func() tea.Msg {
		if run == nil {
			var err error
			if run, err = journal.Read(path); err != nil {
				return undoReadyMsg{err: err}
			}
		}
		plan, err := sorter.NewUndoPlan(run)
		return undoReadyMsg{plan: plan, err: err}
	}
}

// progress is the sorter.Reporter of the TUI: workers write to it, the
// screen reads it on every tick.
type progress struct {
	undo bool
	root string

	mu       sync.Mutex
	total    int
	done     int
	current  string
	problems []report.Problem
}

func (p *progress) Start(total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.total = total
}

func (p *progress) Done(a types.Action, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done++
	path := a.SourcePath
	if p.undo {
		path = a.TargetPath
	}
	p.current = report.ProblemPath(p.root, path)
	if err != nil {
		p.problems = append(p.problems, report.Problem{
			Path:    p.current,
			Err:     err.Error(),
			Skipped: errors.Is(err, sorter.ErrUndoSkipped),
		})
	}
}

func (p *progress) Finish(sorter.Summary) {}

func (p *progress) snapshot() (done, total int, current string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.done, p.total, p.current
}

func (p *progress) Problems() []report.Problem {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]report.Problem(nil), p.problems...)
}

// compileTemplate checks the template of the custom preset.
func compileTemplate(to string) (*rules.Set, error) {
	set, err := rules.Compile([]rules.Rule{{To: to}})
	if err != nil {
		return nil, errors.New(trimRulePrefix(err.Error()))
	}
	return set, nil
}

func trimRulePrefix(s string) string {
	const prefix = "rule 1: "
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}
