package main

import (
	"fmt"
	"time"

	"github.com/lookinalx/fiona/internal/app"
	"github.com/lookinalx/fiona/internal/cli"
	"github.com/lookinalx/fiona/internal/config"
	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/report"
	"github.com/lookinalx/fiona/internal/types"
)

func (e env) runSort(args []string) int {
	opts, err := cli.ParseSort(args)
	if err != nil {
		return e.fail(err)
	}
	cfg, err := config.Load()
	if err != nil {
		return e.fail(err)
	}
	opts.ApplyConfig(cfg)

	set, ruleSource, err := opts.Rules(cfg)
	if err != nil {
		return e.fail(err)
	}

	req := app.Request{
		Source:     opts.Source,
		Dest:       opts.Dest,
		Rules:      set,
		Action:     opts.Action,
		OnConflict: opts.OnConflict,
		Recursive:  opts.Recursive,
		Hidden:     opts.Hidden,
	}

	scanned, err := app.Scan(req)
	if err != nil {
		return e.fail(err)
	}

	if files := app.NeedsML(scanned, set); len(files) > 0 {
		fmt.Fprintf(e.stdout, "Classifying %s with ML…\n", report.Count(len(files), "file"))
		classifier := app.NewSidecar(opts.MLConfigPath)
		defer classifier.Close()

		ctx, stop := notifyContext()
		err := app.Classify(ctx, files, classifier)
		stop()
		if err != nil {
			return e.fail(err)
		}
	}

	plan := app.Plan(scanned, req, journal.NewRunID(time.Now()))
	view := report.PlanView{Plan: plan, RuleSource: ruleSource, Recursive: opts.Recursive}

	if report.Empty(plan) {
		e.println(report.NothingToSort(view))
		return ExitSuccess
	}

	e.println(report.Header(view))
	if overview := report.Overview(view, e.styles); len(overview) > 0 {
		e.println("")
		e.println(overview...)
	}
	if opts.List {
		e.println("")
		e.println(report.FileList(plan, e.styles)...)
	}
	e.println("")

	if opts.DryRun {
		e.println("Dry run: nothing is changed.")
		return ExitSuccess
	}
	if plan.Stats().Files == 0 {
		e.println("Nothing to do.")
		return ExitSuccess
	}

	if !opts.Yes {
		prompt := e.prompt()
	ask:
		for {
			switch prompt.Ask(report.Question(plan), true) {
			case cli.AnswerYes:
				break ask
			case cli.AnswerList:
				e.println(report.FileList(plan, e.styles)...)
				e.println("")
			case cli.AnswerNoInput:
				e.println("No answer: nothing is changed. Use --yes to run without confirmation.")
				return ExitSuccess
			default:
				e.println("Cancelled: nothing is changed.")
				return ExitSuccess
			}
		}
	}

	historyDir, err := config.Dir()
	if err != nil {
		e.warn("the run is not added to the history: %v", err)
		historyDir = ""
	}

	var totalBytes int64
	if plan.FileAction == string(types.OpCopy) {
		totalBytes = plan.Stats().Size
	}
	rep := cli.NewSortReporter(e.stdout, e.terminal, plan.BaseDirSource, totalBytes)

	ctx, stop := notifyContext()
	defer stop()

	out, err := app.Execute(ctx, plan, app.ExecOptions{
		Workers:    opts.Workers,
		LogDir:     opts.LogPath,
		Version:    Version,
		HistoryDir: historyDir,
	}, rep)
	if out.JournalPath == "" {
		// nothing was touched
		return e.fail(err)
	}

	e.println(report.SortOutcome(report.SortResult{
		Plan:     plan,
		Summary:  out.Summary,
		Elapsed:  out.Elapsed,
		Problems: rep.Problems(),
		UndoHint: undoHint(opts, out),
	}, e.styles)...)

	if out.HistoryErr != nil {
		e.warn("the run is not added to the history: %v", out.HistoryErr)
	}
	if out.TrashErr != nil {
		e.warn("cannot clean old trash: %v", out.TrashErr)
	}
	if err != nil {
		return e.fail(err)
	}
	return out.Summary.ExitCode()
}

// undoHint is the shortest command that undoes this run.
func undoHint(opts *cli.SortOpts, out app.Outcome) string {
	switch {
	case out.InHistory:
		return "fiona undo"
	case opts.LogPath == "":
		return "fiona undo --dest " + report.ShellQuote(opts.Dest)
	default:
		return "fiona undo --log " + report.ShellQuote(out.JournalPath)
	}
}
