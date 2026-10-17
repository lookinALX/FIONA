// Package app is the sorting pipeline shared by the CLI and the TUI: scan,
// classify, plan, execute, undo.
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lookinalx/fiona/internal/history"
	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/rules"
	"github.com/lookinalx/fiona/internal/scanner"
	"github.com/lookinalx/fiona/internal/sorter"
)

// notImage is what the classifier answers for files that are not images.
const notImage = "__not_image__"

type Request struct {
	Source, Dest string // absolute, resolved
	Rules        *rules.Set
	Action       string // "move" or "copy"
	OnConflict   string
	Recursive    bool
	Hidden       bool
}

// Scan reads the source folder. Dest is not scanned when it lies inside source.
func Scan(req Request) (*scanner.Result, error) {
	return ScanContext(context.Background(), req)
}

// ScanContext is Scan that stops early when ctx is cancelled.
func ScanContext(ctx context.Context, req Request) (*scanner.Result, error) {
	opt := scanner.Options{Recursive: req.Recursive, IncludeHidden: req.Hidden, Context: ctx}
	if isInside(req.Source, req.Dest) {
		opt.Exclude = []string{req.Dest}
	}
	return scanner.Scan(req.Source, opt)
}

// NeedsML returns the files whose matching rule uses {ml} and that have no tag yet.
func NeedsML(scanned *scanner.Result, set *rules.Set) []*scanner.FileInfo {
	if !set.Uses(rules.PhML) {
		return nil
	}
	var files []*scanner.FileInfo
	for _, fi := range scanned.Files {
		if fi.Tag != "" {
			continue
		}
		if i, ok := set.Match(fi); ok && set.RuleUses(i, rules.PhML) {
			files = append(files, fi)
		}
	}
	return files
}

// Classify tags the files. A file the classifier does not see as an image
// gets its category as the tag.
func Classify(ctx context.Context, files []*scanner.FileInfo, c Classifier) error {
	if len(files) == 0 {
		return nil
	}
	paths := make([]string, len(files))
	for i, fi := range files {
		paths[i] = fi.Path
	}

	tags, err := c.Classify(ctx, paths)
	if err != nil {
		return err
	}
	for _, fi := range files {
		tag, ok := tags[fi.Path]
		if !ok || tag == notImage || tag == "" {
			tag = fi.Category()
		}
		fi.Tag = tag
	}
	return nil
}

// Plan decides what happens to every file. Nothing on disk is changed.
func Plan(scanned *scanner.Result, req Request, runID string) *sorter.Plan {
	return sorter.NewPlan(scanned, req.Rules, sorter.PlanParams{
		Source:     req.Source,
		Dest:       req.Dest,
		FileAction: req.Action,
		OnConflict: req.OnConflict,
		RunID:      runID,
	})
}

type ExecOptions struct {
	Workers    int
	LogDir     string // where the journal goes; default <dest>/.fiona/runs
	Version    string
	HistoryDir string // where the run index lives; empty means no index
}

type Outcome struct {
	Summary     sorter.Summary
	JournalPath string
	InHistory   bool
	HistoryErr  error // the run goes on without the index
	TrashErr    error
	Elapsed     time.Duration
}

// Execute creates the journal, adds the run to the history and carries out the plan.
func Execute(ctx context.Context, plan *sorter.Plan, opts ExecOptions, rep sorter.Reporter) (Outcome, error) {
	var out Outcome
	start := time.Now()

	if err := os.MkdirAll(plan.BaseDirDest, 0755); err != nil {
		return out, fmt.Errorf("cannot create destination directory: %w", err)
	}

	logDir := opts.LogDir
	if logDir == "" {
		logDir = filepath.Join(plan.BaseDirDest, journal.StateDir, journal.RunsDir)
	}
	jrn, err := journal.Create(logDir, plan.RunID)
	if err != nil {
		return out, err
	}
	out.JournalPath = jrn.Path()

	if opts.HistoryDir != "" {
		out.HistoryErr = history.Append(opts.HistoryDir, history.Entry{
			RunID:     plan.RunID,
			StartedAt: start.UTC().Truncate(time.Second),
			Source:    plan.BaseDirSource,
			Dest:      plan.BaseDirDest,
			Journal:   jrn.Path(),
			Action:    plan.FileAction,
			Planned:   plan.Stats().Files,
		})
		out.InHistory = out.HistoryErr == nil
	}

	_, out.TrashErr = sorter.CleanTrash(plan.BaseDirDest, start)

	out.Summary, err = sorter.NewExecutor(plan, jrn, opts.Workers, opts.Version).Run(ctx, rep)
	out.Elapsed = time.Since(start)
	return out, err
}

// isInside reports whether path lies strictly inside root. Both must be resolved.
func isInside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && filepath.IsLocal(rel)
}
