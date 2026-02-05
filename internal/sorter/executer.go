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
	"syscall"
)

const separator = "================================================"

type Executor struct {
	plan       *Plan
	fileAction string
	onConflict string
	dryRun     bool
	force      string
}

func NewExecutor(plan *Plan, opt *cli.Opts) Executor {
	return Executor{
		plan:       plan,
		fileAction: opt.Action,
		onConflict: opt.ConflictStrategy,
		dryRun:     opt.DryRun,
		force:      opt.Force,
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
	for _, action := range ex.plan.Actions {
		err := ex.executeAction(action)
		if err != nil {
			fmt.Println("Error executing action:", err)
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
	fmt.Printf("Processing file --> %s\n", action.SourcePath)
	err = ProcessFile(action.SourcePath, filepath.Join(action.DestPath, filepath.Base(action.SourcePath)), ex.onConflict, ex.fileAction)
	return err
}

func ProcessFile(source, destination, onConflict, fileAction string) error {
	_, err := os.Stat(destination)
	fileExists := (err == nil)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot stat destination: %w", err)
	}

	finalDest := destination

	if fileExists {
		switch onConflict {
		case cli.ConflictReplace:
			if err := os.Remove(destination); err != nil {
				return fmt.Errorf("cannot remove existing file: %w", err)
			}

		case cli.ConflictSkip:
			fmt.Printf("⊘ Skipping (already exists): %s\n", filepath.Base(destination))
			return nil

		case cli.ConflictRename:
			finalDest = generateNewName(destination)
			fmt.Printf("⚠ Conflict resolved: saving as %s\n", filepath.Base(finalDest))

		default:
			return fmt.Errorf("invalid onConflict strategy: %s", onConflict)
		}
	}

	if fileAction == "copy" {
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

		if _, err := os.Stat(newPath); errors.Is(err, os.ErrNotExist) {
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
