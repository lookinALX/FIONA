package sorter

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/lookinalx/fiona/internal/cli"
	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/types"

	"github.com/schollz/progressbar/v3"
)

const separator = "================================================"

type Executor struct {
	plan       *Plan
	fileAction string
	onConflict string
	dryRun     bool
	force      string
	workers    int
	jrn        journal.Journal
}

type ExecutionResult struct {
	mu           sync.Mutex
	successCount int
	errorCount   int
}

func NewExecutor(plan *Plan, opt *cli.Opts) Executor {
	return Executor{
		plan:       plan,
		fileAction: opt.FileAction,
		onConflict: opt.ConflictStrategy,
		dryRun:     opt.DryRun,
		force:      opt.Force,
		workers:    opt.Workers,
		jrn:        journal.NewJournal(filepath.Join(opt.LogPath, "fiona_logs.json"), opt.FileAction),
	}
}

func (ex *Executor) Execute() {
	if ex.dryRun {
		ex.plan.Print(true)
	}
	switch ex.force {
	case "yes":
		ex.start()
	default:
		if isContinue() {
			ex.start()
		}
	}
}

func (ex *Executor) start() {
	fmt.Println(separator)
	fmt.Println("Starting execution...")

	bar := progressbar.Default(int64(len(ex.plan.Actions)))

	jobs := make(chan types.Action, len(ex.plan.Actions))
	var wg sync.WaitGroup

	exres := ExecutionResult{sync.Mutex{}, 0, 0}

	for i := 0; i < ex.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for action := range jobs {
				msg, err := ex.executeAction(action)

				status := ""
				errMsg := ""

				if err != nil {
					exres.IncrementFail()
					status = "failed"
					errMsg = err.Error()
				} else {
					exres.IncrementSuccess()
					status = "succeeded"
				}
				ex.jrn.AppendLogEntryFromAction(action, status, msg, errMsg)
				bar.Add(1)
			}
		}(i)
	}

	for _, action := range ex.plan.Actions {
		jobs <- action
	}
	close(jobs)

	wg.Wait()

	bar.Finish()

	if err := ex.jrn.SaveAsJson(); err != nil {
		fmt.Println("!!! ⚠ !!! Failed to save as json:", err)
	}

	fmt.Println("Execution is ended.")
	fmt.Printf("✓ Succeeded operations: %d\n", exres.successCount)
	fmt.Printf("⊘ Failed operations: %d\n", exres.errorCount)
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

func (ex *Executor) executeAction(action types.Action) (string, error) {
	err := os.MkdirAll(action.DestPath, 0755)
	msg := ""
	if err != nil {
		return msg, err
	}
	msg, err = ex.ProcessFile(action.SourcePath, filepath.Join(action.DestPath, filepath.Base(action.SourcePath)))
	return msg, err
}

func (ex *Executor) ProcessFile(source, destination string) (string, error) {
	_, err := os.Stat(destination)
	fileExists := (err == nil)
	msg := ""

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return msg, fmt.Errorf("cannot stat destination: %w", err)
	}

	finalDest := destination

	if fileExists {
		switch ex.onConflict {
		case cli.ConflictReplace:
			msg = fmt.Sprintf("⟲ Replaced: %s\n", filepath.Base(finalDest))
			err := os.Remove(destination)
			if err != nil && errors.Is(err, os.ErrNotExist) {
				return msg, fmt.Errorf("cannot remove existing file: %w", err)
			}

		case cli.ConflictSkip:
			msg = fmt.Sprintf("⊘ Skipping (already exists): %s\n", filepath.Base(destination))
			return msg, nil

		case cli.ConflictRename:
			finalDest = generateNewName(destination)
			msg = fmt.Sprintf("⚠ Conflict resolved: saving as %s\n", filepath.Base(finalDest))

		default:
			return msg, fmt.Errorf("invalid onConflict strategy: %s", ex.onConflict)
		}
	}

	if ex.fileAction == "copy" {
		return msg, copyWithMetadata(source, finalDest)
	}
	return msg, smartMoveFile(source, finalDest)
}

func generateNewName(path string) string {
	dir := filepath.Dir(path)
	ext := filepath.Ext(path)
	nameWithoutExt := strings.TrimSuffix(filepath.Base(path), ext)

	// photo.jpg → photo_1.jpg, photo_2.jpg, ...
	for i := 1; ; i++ {
		newName := fmt.Sprintf("%s_%d%s", nameWithoutExt, i, ext)
		newPath := filepath.Join(dir, newName)

		f, err := os.OpenFile(newPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			f.Close()
			os.Remove(newPath)
			return newPath
		}
	}
}

func smartMoveFile(source, destination string) error {
	err := os.Rename(source, destination)
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.EXDEV) {
		if err = copyWithMetadata(source, destination); err != nil {
			return err
		}
		return os.Remove(source)
	}
	return err
}

func copyWithMetadata(source, destination string) error {
	srcFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		os.Remove(destination)
		return err
	}

	err = dstFile.Sync()
	if err != nil {
		os.Remove(destination)
		return err
	}

	err = os.Chmod(destination, srcInfo.Mode())
	if err != nil {
		return err
	}

	err = os.Chtimes(destination, srcInfo.ModTime(), srcInfo.ModTime())
	if err != nil {
		return err
	}

	return nil
}

func (exres *ExecutionResult) IncrementFail() {
	exres.mu.Lock()
	defer exres.mu.Unlock()

	exres.errorCount++
}

func (exres *ExecutionResult) IncrementSuccess() {
	exres.mu.Lock()
	defer exres.mu.Unlock()

	exres.successCount++
}
