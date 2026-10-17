package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/lookinalx/fiona/internal/cli"
	"github.com/lookinalx/fiona/internal/config"
	"github.com/lookinalx/fiona/internal/report"
	"github.com/lookinalx/fiona/internal/suggest"
	"github.com/lookinalx/fiona/internal/tui"
)

const (
	ExitSuccess = 0
	ExitFailure = 1
	ExitUsage   = 2 // a mistake in the command line or the config, nothing is touched
)

var subcommands = []string{"sort", "undo", "history", "presets", "tui", "help"}

// env is what the commands write to and read from.
type env struct {
	stdin         io.Reader
	stdout        io.Writer
	stderr        io.Writer
	stdinTerminal bool
	terminal      bool // stdout is a terminal
	styles        report.Styles
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	e := env{
		stdin:         os.Stdin,
		stdout:        os.Stdout,
		stderr:        os.Stderr,
		stdinTerminal: term.IsTerminal(int(os.Stdin.Fd())),
		terminal:      term.IsTerminal(int(os.Stdout.Fd())),
		styles:        report.NewStyles(lipgloss.NewRenderer(os.Stdout)),
	}

	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--version" || arg == "-v" {
			fmt.Fprintf(e.stdout, "FIONA %s\n", Version)
			return ExitSuccess
		}
		if arg == "--help" || arg == "-h" {
			cli.PrintUsage(e.stdout, Version)
			return ExitSuccess
		}
	}

	if len(args) == 0 {
		if e.stdinTerminal && e.terminal {
			return e.runTUI(nil)
		}
		cli.PrintShortUsage(e.stderr)
		return ExitUsage
	}

	switch args[0] {
	case "sort":
		return e.runSort(args[1:])
	case "undo":
		return e.runUndo(args[1:])
	case "history":
		return e.runHistory(args[1:])
	case "presets":
		return e.runPresets(args[1:])
	case "tui":
		return e.runTUI(args[1:])
	case "help":
		cli.PrintUsage(e.stdout, Version)
		return ExitSuccess
	}

	// a bare folder or flags first mean sort
	if strings.HasPrefix(args[0], "-") || looksLikePath(args[0]) {
		return e.runSort(args)
	}

	msg := fmt.Sprintf("unknown command or folder %q.", args[0])
	if guess, ok := suggest.Closest(args[0], subcommands); ok {
		msg += fmt.Sprintf(" Did you mean %q?", guess)
	}
	fmt.Fprintln(e.stderr, "fiona: "+msg)
	cli.PrintShortUsage(e.stderr)
	return ExitUsage
}

func looksLikePath(arg string) bool {
	if info, err := os.Stat(arg); err == nil && info.IsDir() {
		return true
	}
	return strings.ContainsAny(arg, `/\`) || strings.HasPrefix(arg, ".") || strings.HasPrefix(arg, "~")
}

// fail prints the error and returns the exit code that fits it.
func (e env) fail(err error) int {
	code := ExitFailure
	var usageErr *cli.UsageError
	var cfgErr *config.Error
	if errors.As(err, &usageErr) || errors.As(err, &cfgErr) {
		code = ExitUsage
	}

	for _, line := range strings.Split(err.Error(), "\n") {
		if strings.HasPrefix(line, "Available: ") {
			fmt.Fprintln(e.stderr, line)
			continue
		}
		fmt.Fprintln(e.stderr, "fiona: "+line)
	}
	return code
}

func (e env) warn(format string, args ...any) {
	fmt.Fprintf(e.stderr, "fiona: "+format+"\n", args...)
}

func (e env) println(lines ...string) {
	for _, line := range lines {
		fmt.Fprintln(e.stdout, line)
	}
}

func (e env) prompt() *cli.Prompt {
	return cli.NewPrompt(e.stdin, e.stdout, !e.stdinTerminal)
}

func (e env) runTUI(args []string) int {
	source := "."
	switch len(args) {
	case 0:
	case 1:
		source = args[0]
	default:
		return e.fail(&cli.UsageError{Err: fmt.Errorf("unexpected arguments: %s", strings.Join(args[1:], " "))})
	}

	resolved, err := cli.ResolvePath(source)
	if err != nil {
		return e.fail(err)
	}
	if info, err := os.Stat(resolved); err != nil || !info.IsDir() {
		return e.fail(&cli.UsageError{Err: fmt.Errorf("not a folder: %s", source)})
	}

	cfg, err := config.Load()
	if err != nil {
		return e.fail(err)
	}
	historyDir, err := config.Dir()
	if err != nil {
		return e.fail(err)
	}

	if err := tui.Run(tui.Options{Source: resolved, Config: cfg, HistoryDir: historyDir, Version: Version}); err != nil {
		return e.fail(err)
	}
	return ExitSuccess
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

func resolvedCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if resolved, err := cli.ResolvePath(cwd); err == nil {
		return resolved
	}
	return filepath.Clean(cwd)
}
