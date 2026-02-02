package main

import (
	cli "FIONA/internal/cli"
	"FIONA/internal/scanner"
	"FIONA/internal/sorter"
	"fmt"
	"os"
)

var opts cli.Opts

func main() {
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
	files, err := sc.Scan(opts.Source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Something went wrong:\n%v\n", err)
	}

	pl := sorter.NewPlan(&opts)
	for _, f := range files {
		action := sorter.NewAction(f, rls, opts.Dest)
		pl.AddAction(action)
	}

	pl.Print(true)
}
