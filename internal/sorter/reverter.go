package sorter

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/lookinalx/fiona/internal/types"

	"github.com/schollz/progressbar/v3"
)

type Reverter struct {
	mu      sync.Mutex
	plan    *UndoPlan
	workers int
}

func NewReverter(plan *UndoPlan, workers int) Reverter {
	return Reverter{
		plan:    plan,
		workers: workers,
	}
}

func (r *Reverter) RunUndo() {
	fmt.Println(separator)
	fmt.Println("Starting undo process...")

	bar := progressbar.Default(int64(len(r.plan.UndoActions)))

	jobs := make(chan types.UndoAction, len(r.plan.UndoActions))
	var wg sync.WaitGroup

	dirsToClean := make(map[string]struct{})
	var dirsMu sync.Mutex

	for i := 0; i < r.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for undo := range jobs {
				if r.plan.FileAction == "copy" {
					err := os.Remove(undo.SourcePath)
					if err != nil && !os.IsNotExist(err) {
						r.mu.Lock()
						fmt.Fprintf(os.Stderr, "failed by undoing the file %s --> %v\n", undo.SourcePath, err)
						r.mu.Unlock()
					} else {
						dir := filepath.Dir(undo.SourcePath)
						dirsMu.Lock()
						dirsToClean[dir] = struct{}{}
						dirsMu.Unlock()
					}
				} else {
					err := os.MkdirAll(filepath.Dir(undo.SourcePath), 0755)
					if err != nil {
						r.mu.Lock()
						fmt.Fprintf(os.Stderr, "failed by creating the directory %s --> %v\n", undo.SourcePath, err)
						r.mu.Unlock()
						continue
					}
					err = smartMoveFile(undo.SourcePath, undo.DestPath)
					if err != nil {
						r.mu.Lock()
						fmt.Fprintf(os.Stderr, "failed by moving %s --> %v\n", undo.SourcePath, err)
						r.mu.Unlock()
					} else {
						dir := filepath.Dir(undo.SourcePath)
						dirsMu.Lock()
						dirsToClean[dir] = struct{}{}
						dirsMu.Unlock()
					}
				}
				bar.Add(1)
			}
		}(i)
	}

	for _, undo := range r.plan.UndoActions {
		jobs <- undo
	}
	close(jobs)

	wg.Wait()

	fmt.Println("\nCleaning up empty directories...")
	for dir := range dirsToClean {
		err := removeEmptyDirs(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed by removing empty directory %s --> %v\n", dir, err)
		}
	}

	fmt.Println("Undo process is ended.")
}

func removeEmptyDirs(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		err := os.Remove(path)
		if err != nil {
			return err
		}

		parent := filepath.Dir(path)
		if parent != "." && parent != "/" {
			removeEmptyDirs(parent)
		}
	}

	return nil
}
