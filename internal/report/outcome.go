package report

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/lookinalx/fiona/internal/sorter"
	"github.com/lookinalx/fiona/internal/types"
)

const maxOutcomeProblems = 20

// Problem is a file that failed or was skipped during a run.
type Problem struct {
	Path    string
	Err     string
	Skipped bool
}

type SortResult struct {
	Plan     *sorter.Plan
	Summary  sorter.Summary
	Elapsed  time.Duration
	Problems []Problem
	UndoHint string // the command that undoes the run
}

// SortOutcome is the result of a run: one line, and the problems under it.
func SortOutcome(r SortResult, st Styles) []string {
	s := r.Summary
	planned := r.Plan.Stats().Files
	done := s.Moved + s.Copied
	verb := "Moved"
	if r.Plan.FileAction == string(types.OpCopy) {
		verb = "Copied"
	}
	undo := ""
	if r.UndoHint != "" {
		undo = " Undo: " + r.UndoHint
	}

	var lines []string
	switch {
	case s.Interrupted:
		lines = append(lines, st.warn("Stopped")+fmt.Sprintf(" after %d of %s.%s", done+s.Failed, Count(planned, "file"), undo))
		lines = append(lines, outcomeProblems(r.Problems, st)...)
	case s.Failed > 0:
		lines = append(lines, st.warn("!")+fmt.Sprintf(" %s %d of %s, %d failed:", verb, done, Count(planned, "file"), s.Failed))
		lines = append(lines, outcomeProblems(r.Problems, st)...)
		if undo != "" {
			lines = append(lines, " "+undo)
		}
	default:
		lines = append(lines, st.ok("✓")+fmt.Sprintf(" %s %s in %s.%s", verb, Count(done, "file"), Duration(r.Elapsed), undo))
	}
	return lines
}

func outcomeProblems(problems []Problem, st Styles) []string {
	var lines []string
	for i, p := range problems {
		if i == maxOutcomeProblems {
			lines = append(lines, st.dim(fmt.Sprintf("  … and %d more", len(problems)-maxOutcomeProblems)))
			break
		}
		mark := st.bad("✗")
		if p.Skipped {
			mark = st.warn("⊘")
		}
		lines = append(lines, fmt.Sprintf("  %s %s: %s", mark, p.Path, p.Err))
	}
	return lines
}

// RunDescription tells what a run did: "moved 214 files in ~/Downloads".
func RunDescription(p *sorter.UndoPlan) string {
	h := p.Run.Header
	done := 0
	for _, op := range p.Run.Ops {
		if op.Executable() && op.Result != nil && op.Result.Status == types.StatusOK {
			done++
		}
	}

	verb := "moved"
	if h.FileAction == string(types.OpCopy) {
		verb = "copied"
	}
	where := "in " + Path(h.Dest)
	if h.Source != h.Dest {
		where = "from " + Path(h.Source) + " to " + Path(h.Dest)
	}
	return fmt.Sprintf("%s %s %s", verb, Count(done, "file"), where)
}

// UndoPreview returns the details worth knowing before an undo and the question.
func UndoPreview(p *sorter.UndoPlan, now time.Time, st Styles) (lines []string, question string) {
	copied := p.Run.Header.FileAction == string(types.OpCopy)

	if p.Run.Footer == nil {
		what := "files it really moved are returned"
		if copied {
			what = "copies it really made are removed"
		}
		lines = append(lines, st.warn("  The run was interrupted:")+" only "+what+".")
	}
	if n := p.Stats.TrashRestore; n > 0 {
		lines = append(lines, fmt.Sprintf("  Restore from trash: %d", n))
	}
	if n := p.Stats.Skipped; n > 0 {
		lines = append(lines, fmt.Sprintf("  Skip, changed after the run: %d", n))
		var skipped []string
		for _, a := range p.Actions {
			if a.Kind == types.UndoSkip {
				skipped = append(skipped, fmt.Sprintf("%s: %s", relTo(p.Run.Header.Dest, a.Op.TargetPath), a.Note))
			}
		}
		lines = append(lines, problemLines(skipped, st)...)
	}

	question = fmt.Sprintf("Undo the run from %s (%s)?", When(p.Run.Header.StartedAt, now), RunDescription(p))
	return lines, question
}

type UndoResult struct {
	Plan     *sorter.UndoPlan
	Summary  sorter.Summary
	Problems []Problem
}

// UndoOutcome is the result of an undo in one line, and the problems under it.
func UndoOutcome(r UndoResult, st Styles) []string {
	s := r.Summary
	copied := r.Plan.Run.Header.FileAction == string(types.OpCopy)
	planned := r.Plan.Stats.MoveBack + r.Plan.Stats.RemoveCopy + r.Plan.Stats.Skipped

	main := "Restored " + Count(s.Restored, "file")
	if copied {
		main = "Removed " + Count(s.Removed, "copy")
	}

	if s.Interrupted {
		lines := []string{st.warn("Stopped") + fmt.Sprintf(" after %d of %s. Run the same undo again to finish.", s.Restored+s.Removed+s.Failed+s.Skipped, Count(planned, "file"))}
		return append(lines, outcomeProblems(r.Problems, st)...)
	}

	if s.Failed > 0 || s.Skipped > 0 {
		done := s.Restored
		noun := "file"
		verb := "Restored"
		if copied {
			done, noun, verb = s.Removed, "copy", "Removed"
		}
		var counts []string
		if s.Skipped > 0 {
			counts = append(counts, fmt.Sprintf("%d skipped", s.Skipped))
		}
		if s.Failed > 0 {
			counts = append(counts, fmt.Sprintf("%d failed", s.Failed))
		}
		lines := []string{st.warn("!") + fmt.Sprintf(" %s %d of %s, %s:", verb, done, Count(planned, noun), strings.Join(counts, ", "))}
		return append(lines, outcomeProblems(r.Problems, st)...)
	}

	parts := []string{main}
	if s.TrashRestored > 0 {
		parts = append(parts, fmt.Sprintf("restored %d from trash", s.TrashRestored))
	}
	if s.DirsRemoved > 0 {
		parts = append(parts, "removed "+Count(s.DirsRemoved, "empty folder"))
	}
	return []string{st.ok("✓") + " " + strings.Join(parts, ", ") + "."}
}

// ProblemPath shows a problem file relative to the folder it belongs to.
func ProblemPath(root, path string) string {
	return relTo(root, filepath.Clean(path))
}
