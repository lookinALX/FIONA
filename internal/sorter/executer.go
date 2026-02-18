package sorter

import (
	"FIONA/internal/cli"
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

const separator = "================================================"

type Executor struct {
	plan       *Plan
	fileAction string
	onConflict string
	dryRun     bool
	force      string
	workers    int
	logMutex   sync.Mutex
}

type ExecutionResult struct {
	mu           sync.Mutex
	successCount int
	errorCount   int
	errors       []error
}

func NewExecutor(plan *Plan, opt *cli.Opts) Executor {
	return Executor{
		plan:       plan,
		fileAction: opt.Action,
		onConflict: opt.ConflictStrategy,
		dryRun:     opt.DryRun,
		force:      opt.Force,
		workers:    opt.Workers,
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

	jobs := make(chan Action, len(ex.plan.Actions))
	var wg sync.WaitGroup

	exres := ExecutionResult{sync.Mutex{}, 0, 0, []error{}}

	for i := 0; i < ex.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for action := range jobs {
				err := ex.executeAction(action)
				if err != nil {
					exres.AddError(fmt.Errorf("cannot process %s: %w", action.SourcePath, err))
				} else {
					exres.IncrementSuccess()
				}
			}
		}(i)
	}

	for _, action := range ex.plan.Actions {
		jobs <- action
	}
	close(jobs)

	wg.Wait()

	fmt.Println("Execution is ended.")
	fmt.Printf("✓ Succeeded operations: %d\n", exres.successCount)
	fmt.Printf("⊘ Failed operations: %d\n", exres.errorCount)

	if len(exres.errors) > 0 {
		fmt.Println("⚠ Errors encountered: ")
		for _, err := range exres.errors {
			fmt.Println("   --> ", err)
		}
	}
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

func (ex *Executor) executeAction(action Action) error {
	err := os.MkdirAll(action.DestPath, 0755)
	if err != nil {
		return err
	}
	ex.log("Processing file --> %s\n", action.SourcePath)
	err = ex.ProcessFile(action.SourcePath, filepath.Join(action.DestPath, filepath.Base(action.SourcePath)))
	return err
}

func (ex *Executor) ProcessFile(source, destination string) error {
	_, err := os.Stat(destination)
	fileExists := (err == nil)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot stat destination: %w", err)
	}

	finalDest := destination

	if fileExists {
		switch ex.onConflict {
		case cli.ConflictReplace:
			err := os.Remove(destination)
			if err != nil && errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("cannot remove existing file: %w", err)
			}

		case cli.ConflictSkip:
			ex.log("⊘ Skipping (already exists): %s\n", filepath.Base(destination))
			return nil

		case cli.ConflictRename:
			finalDest = generateNewName(destination)
			ex.log("⚠ Conflict resolved: saving as %s\n", filepath.Base(finalDest))

		default:
			return fmt.Errorf("invalid onConflict strategy: %s", ex.onConflict)
		}
	}

	if ex.fileAction == "copy" {
		return copyWithMetadata(source, finalDest)
	}
	return smartMoveFile(source, finalDest)
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

func (ex *Executor) log(format string, args ...interface{}) {
	ex.logMutex.Lock()
	defer ex.logMutex.Unlock()
	fmt.Printf(format, args...)
}

func (exres *ExecutionResult) AddError(err error) {
	exres.mu.Lock()
	defer exres.mu.Unlock()

	exres.errorCount++
	exres.errors = append(exres.errors, err)
}

func (exres *ExecutionResult) IncrementSuccess() {
	exres.mu.Lock()
	defer exres.mu.Unlock()

	exres.successCount++
}
