package sorter

import "FIONA/internal/cli"

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

}
