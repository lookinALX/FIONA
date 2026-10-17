package main

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/lookinalx/fiona/internal/app"
	"github.com/lookinalx/fiona/internal/cli"
	"github.com/lookinalx/fiona/internal/config"
	"github.com/lookinalx/fiona/internal/history"
	"github.com/lookinalx/fiona/internal/report"
	"github.com/lookinalx/fiona/internal/sorter"
)

// historyLimit is how many runs fiona history shows without --all.
const historyLimit = 20

func (e env) runUndo(args []string) int {
	opts, err := cli.ParseUndo(args)
	if err != nil {
		return e.fail(err)
	}
	historyDir, err := config.Dir()
	if err != nil {
		return e.fail(err)
	}

	run, err := app.FindRun(app.UndoTarget{
		LogPath:    opts.LogPath,
		Dest:       opts.Dest,
		RunID:      opts.RunID,
		HistoryDir: historyDir,
		Cwd:        resolvedCwd(),
	})
	var nothing *app.NothingToUndoError
	if errors.As(err, &nothing) {
		e.println(nothing.Error())
		return ExitSuccess
	}
	if err != nil {
		return e.fail(err)
	}

	plan, err := sorter.NewUndoPlan(run)
	if err != nil {
		return e.fail(err)
	}

	details, question := report.UndoPreview(plan, time.Now(), e.styles)
	e.println(details...)

	if opts.DryRun {
		e.println("Would "+strings.ToLower(question[:1])+strings.TrimSuffix(question[1:], "?")+".", "Dry run: nothing is changed.")
		return ExitSuccess
	}
	if !opts.Yes {
		switch e.prompt().Ask(question, false) {
		case cli.AnswerYes:
		case cli.AnswerNoInput:
			e.println("No answer: nothing is changed. Use --yes to undo without confirmation.")
			return ExitSuccess
		default:
			e.println("Cancelled: nothing is changed.")
			return ExitSuccess
		}
	}

	ctx, stop := notifyContext()
	defer stop()

	rep := cli.NewUndoReporter(e.stdout, e.terminal, run.Header.Dest)
	summary, err := app.ExecuteUndo(ctx, plan, opts.Workers, rep)
	e.println(report.UndoOutcome(report.UndoResult{Plan: plan, Summary: summary, Problems: rep.Problems()}, e.styles)...)
	if err != nil {
		return e.fail(err)
	}
	return summary.ExitCode()
}

func (e env) runHistory(args []string) int {
	opts, err := cli.ParseHistory(args)
	if err != nil {
		return e.fail(err)
	}
	dir, err := config.Dir()
	if err != nil {
		return e.fail(err)
	}
	records, err := history.Records(dir)
	if err != nil {
		return e.fail(fmt.Errorf("cannot read the history: %w", err))
	}
	if len(records) == 0 {
		e.println("No runs yet.")
		return ExitSuccess
	}

	shown := records
	if !opts.All && len(shown) > historyLimit {
		shown = shown[:historyLimit]
	}

	now := time.Now()
	w := tabwriter.NewWriter(e.stdout, 0, 0, 2, ' ', 0)
	// only the last column may be styled: tabwriter counts escape codes as width
	fmt.Fprintln(w, "  RUN\tWHEN\tACTION\tFOLDER\tFILES\tSTATE")
	for _, r := range shown {
		where := report.Path(r.Dest)
		if r.Source != r.Dest {
			where = report.Path(r.Source) + " → " + report.Path(r.Dest)
		}
		state := string(r.State)
		switch r.State {
		case history.StateUndone:
			state = e.styles.Dim.Render(state)
		case history.StateDone:
		default:
			state = e.styles.Warn.Render(state)
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%d\t%s\n", r.RunID, report.When(r.StartedAt, now), r.Action, where, r.Planned, state)
	}
	w.Flush()

	if hidden := len(records) - len(shown); hidden > 0 {
		e.println(fmt.Sprintf("  … and %s more: fiona history --all", report.Count(hidden, "older run")))
	}
	return ExitSuccess
}
