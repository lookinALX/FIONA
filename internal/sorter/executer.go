package sorter

import (
	"FIONA/internal/cli"
	"bufio"
	"fmt"
	"os"
	"strings"
)

const separator = "================================================"

type Executor struct {
	plan       *Plan
	fileAction string
	dryRun     bool
}

func NewExecutor(plan *Plan, opt *cli.Opts) Executor {
	return Executor{
		plan:       plan,
		fileAction: opt.Action,
		dryRun:     opt.DryRun,
	}
}

func (ex *Executor) Execute() {
	if ex.dryRun {
		ex.plan.Print(true)
	}
	if isContinue() {
		fmt.Println(separator)
		fmt.Println("Starting execution...")
	}
	return
}

func isContinue() bool {
	fmt.Println("Would you like to continue? (y/N)")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSuffix(input, "\n")
	if input == "y" {
		return true
	}
	return false
}
