package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/lookinalx/fiona/internal/cli"
	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/scanner"
	"github.com/lookinalx/fiona/internal/sorter"
	"github.com/lookinalx/fiona/internal/tui"
	"github.com/lookinalx/fiona/ml"
)

const (
	ExitSuccess = 0
	ExitFailure = 1
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	for _, arg := range args {
		if arg == "--version" || arg == "-version" || arg == "-v" {
			fmt.Printf("FIONA %s\n", Version)
			return ExitSuccess
		}
		if arg == "--help" || arg == "-h" {
			cli.PrintUsage()
			return ExitSuccess
		}
	}

	if len(args) < 1 {
		cli.PrintShortUsage()
		return ExitFailure
	}

	switch args[0] {
	case "tui":
		result, err := tui.Run()
		if err != nil {
			return fail(err)
		}
		if result.Cancelled() {
			return ExitSuccess
		}
		// TODO: send result to sort/undo pipeline
		_ = result
		return ExitSuccess
	case "undo":
		return runUndo(args[1:])
	case "sort":
		return runSort(args[1:])
	default:
		fmt.Println("!!! ⚠ Unknown command ⚠ !!!")
		cli.PrintShortUsage()
		return ExitFailure
	}
}

func fail(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		cli.PrintUsage()
		return ExitSuccess
	}
	fmt.Fprintf(os.Stderr, "Something went wrong:\n%v\n", err)
	return ExitFailure
}

func runSort(args []string) int {
	var opts cli.Opts
	if err := opts.ParseSortFlags(args); err != nil {
		return fail(err)
	}

	rls, err := opts.ParseSortFlagsToRules()
	if err != nil {
		return fail(err)
	}

	scanOpts := scanner.Options{Recursive: opts.Recursive, IncludeHidden: opts.Hidden}
	if cli.IsInside(opts.SourcePath, opts.DestPath) {
		scanOpts.Exclude = []string{opts.DestPath}
	}

	scanned, err := scanner.Scan(opts.SourcePath, scanOpts)
	if err != nil {
		return fail(err)
	}
	files := scanned.Files

	if opts.Sort.Primary == "ml" || opts.Sort.Secondary == "ml" {
		server := ml.NewServer(opts.MlConfigPath)
		if err = server.Start(); err != nil {
			return fail(err)
		}
		defer server.Stop()

		client := ml.NewClient(server.BaseURL())

		paths := make([]string, 0, len(files))
		for _, file := range files {
			paths = append(paths, file.Path)
		}

		tags, mlErr := client.Classify(paths)
		if mlErr != nil {
			return fail(mlErr)
		}

		for i := range files {
			tag := tags[files[i].Path]
			if tag == "__not_image__" {
				files[i].Tag = files[i].Type
			} else {
				files[i].Tag = tag
			}
		}
	}

	plan := sorter.NewPlan(files, rls, scanned.Skipped, sorter.PlanParams{
		Source:     opts.SourcePath,
		Dest:       opts.DestPath,
		FileAction: opts.FileAction,
		OnConflict: opts.ConflictStrategy,
		RunID:      journal.NewRunID(time.Now()),
	})
	plan.Print(os.Stdout, true)

	if opts.DryRun {
		fmt.Println("Dry run: nothing is changed.")
		return ExitSuccess
	}
	if plan.Stats().Files == 0 {
		fmt.Println("Nothing to do.")
		return ExitSuccess
	}
	if !opts.Yes && !cli.Confirm(os.Stdin, os.Stdout, "Would you like to continue?") {
		fmt.Println("Cancelled: nothing is changed.")
		return ExitSuccess
	}

	// side effects start here
	if err = os.MkdirAll(opts.DestPath, 0755); err != nil {
		return fail(fmt.Errorf("cannot create destination directory: %w", err))
	}

	logDir := opts.LogPath
	if logDir == "" {
		logDir = filepath.Join(opts.DestPath, journal.StateDir, journal.RunsDir)
	}
	jrn, err := journal.Create(logDir, plan.RunID)
	if err != nil {
		return fail(err)
	}

	removed, err := sorter.CleanTrash(opts.DestPath, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "!!! ⚠ !!! Failed to clean old trash: %v\n", err)
	}
	if removed > 0 {
		fmt.Printf("Removed trash of %d runs older than %d days.\n", removed, int(sorter.TrashRetention.Hours()/24))
	}

	ctx, stop := notifyContext()
	defer stop()

	executor := sorter.NewExecutor(plan, jrn, opts.Workers, Version)
	summary, err := executor.Run(ctx, cli.NewSortReporter(os.Stdout))

	fmt.Printf("Journal: %s\n", jrn.Path())
	if opts.LogPath == "" {
		fmt.Printf("To undo: fiona undo --dest %s\n", opts.DestPath)
	} else {
		fmt.Printf("To undo: fiona undo --log %s\n", jrn.Path())
	}

	if err != nil {
		return fail(err)
	}
	return summary.ExitCode()
}

func runUndo(args []string) int {
	var opts cli.Opts
	if err := opts.ParseUndoFlags(args); err != nil {
		return fail(err)
	}

	var run *journal.Run
	var err error
	if opts.LogPath != "" {
		run, err = journal.Read(opts.LogPath)
	} else {
		run, err = journal.FindLatest(filepath.Join(opts.DestPath, journal.StateDir, journal.RunsDir))
		if err == nil && run.Header.Dest != opts.DestPath {
			err = fmt.Errorf("%w: %s belongs to dest %q, not to %q",
				journal.ErrRejected, filepath.Base(run.Path), run.Header.Dest, opts.DestPath)
		}
	}
	if err != nil {
		return fail(err)
	}

	plan, err := sorter.NewUndoPlan(run)
	if err != nil {
		return fail(err)
	}
	plan.Print(os.Stdout)

	if opts.DryRun {
		fmt.Println("Dry run: nothing is changed.")
		return ExitSuccess
	}
	if !opts.Yes && !cli.Confirm(os.Stdin, os.Stdout, "Would you like to continue?") {
		fmt.Println("Cancelled: nothing is changed.")
		return ExitSuccess
	}

	ctx, stop := notifyContext()
	defer stop()

	reverter := sorter.NewReverter(plan, opts.Workers)
	summary, err := reverter.Run(ctx, cli.NewUndoReporter(os.Stdout))
	if err != nil {
		return fail(err)
	}
	return summary.ExitCode()
}

// notifyContext cancels the context on SIGINT and SIGTERM. After the first
// signal the default handling is restored, so a second one terminates at once.
func notifyContext() (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()
	return ctx, stop
}
