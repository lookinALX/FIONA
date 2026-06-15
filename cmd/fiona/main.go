package main

import (
	"fmt"
	"os"

	"github.com/lookinalx/fiona/internal/cli"
	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/scanner"
	"github.com/lookinalx/fiona/internal/sorter"
	"github.com/lookinalx/fiona/internal/tui"
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

	if len(os.Args) < 2 {
		cli.PrintShortUsage()
		os.Exit(ExitFailure)
	}

	subcommand := os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	switch subcommand {
	case "tui":
		if err := tui.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Something went wrong:\n%v\n", err)
			os.Exit(ExitFailure)
		}
		os.Exit(ExitSuccess)
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

		if opts.Sort.Primary == "ml" || opts.Sort.Secondary == "ml" {
			server := ml.NewServer(opts.MlConfigPath)
			err = server.Start()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Something went wrong:\n%v\n", err)
				os.Exit(ExitFailure)
			}
			defer server.Stop()

			client := ml.NewClient(server.BaseURL())

			paths := make([]string, 0, len(files))
			for _, file := range files {
				paths = append(paths, file.Path)
			}

			tags, mlErr := client.Classify(paths)
			if mlErr != nil {
				fmt.Fprintf(os.Stderr, "Something went wrong:\n%v\n", mlErr)
				os.Exit(ExitFailure)
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
