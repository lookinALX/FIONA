package main

import (
	"fmt"
	"os"

	"github.com/lookinalx/fiona/internal/cli"
	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/scanner"
	"github.com/lookinalx/fiona/internal/sorter"
	"github.com/lookinalx/fiona/internal/types"
	"github.com/lookinalx/fiona/ml"
)

const (
	ExitSuccess = 0
	ExitFailure = 1
)

var opts cli.Opts

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-version" || arg == "-v" {
			fmt.Printf("FIONA %s\n", Version)
			os.Exit(ExitSuccess)
		}
		if arg == "--help" || arg == "-h" {
			cli.PrintUsage()
			os.Exit(ExitSuccess)
		}
	}

	subcommand := os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	switch subcommand {
	case "undo":
		err := opts.ParseUndoFlags()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Something went wrong:\n%v\n", err)
			os.Exit(ExitFailure)
		}

		jrn := journal.NewJournal("", "")
		err = jrn.LoadFromJson(opts.LogPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Something went wrong:\n%v\n", err)
			os.Exit(ExitFailure)
		}

		rv := sorter.NewReverter(sorter.NewUndoPlan(&jrn), opts.Workers)

		rv.RunUndo()

		os.Exit(ExitSuccess)
	case "sort":
		err := opts.ParseSortFlags()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Something went wrong:\n%v\n", err)
			os.Exit(ExitFailure)
		}

		rls, err := opts.ParseSortFlagsToRules()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Something went wrong:\n%v\n", err)
			os.Exit(ExitFailure)
		}

		sc := scanner.NewScanner()
		files, err := sc.Scan(opts.SourcePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Something went wrong:\n%v\n", err)
			os.Exit(ExitFailure)
		}

		// TODO: ml set tags integration (image classification)
		if opts.Sort.Primary == "ml" || opts.Sort.Secondary == "ml" {
			for _, file := range files {
				file.Tag = ml.GetTag()
			}
		}

		pl := sorter.NewPlan(&opts)
		for _, f := range files {
			action := types.NewAction(f, rls, opts.DestPath)
			pl.AddAction(action)
		}

		executor := sorter.NewExecutor(&pl, &opts)
		executor.Execute()
		os.Exit(ExitSuccess)
	default:
		fmt.Println("!!! ⚠ Unknown command ⚠ !!!")
		cli.PrintShortUsage()
		os.Exit(ExitFailure)
	}
}
