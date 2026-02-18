package main

import (
	"FIONA/internal/cli"
	"FIONA/internal/scanner"
	"FIONA/internal/sorter"
	"fmt"
	"os"
)

var opts cli.Opts

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-version" || arg == "-v" {
			fmt.Printf("FIONA %s\n", Version)
			os.Exit(0)
		}
	}

	err := opts.ParseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Something went wrong:\n%v\n", err)
		return
	}
	rls, err := opts.ParseToRules()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Something went wrong:\n%v\n", err)
	}

	sc := scanner.NewScanner()
	files, err := sc.Scan(opts.SourcePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Something went wrong:\n%v\n", err)
	}

	pl := sorter.NewPlan(&opts)
	for _, f := range files {
		action := sorter.NewAction(f, rls, opts.DestPath)
		pl.AddAction(action)
	}

	executor := sorter.NewExecutor(&pl, &opts)
	executor.Execute()
}
