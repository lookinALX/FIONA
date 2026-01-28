package main

import (
	cli "FIONA/internal/cli"
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
	opts.PrintOptions()
}
