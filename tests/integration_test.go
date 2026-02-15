package tests

import (
	"FIONA/internal/cli"
	"FIONA/internal/scanner"
	"FIONA/internal/sorter"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func createTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("file doesn't exists: %s", path)
	}
}

func TestEndToEnd_CopyByTypeThenExtension(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	extensions := []string{".jpg", ".pdf", ".png", ".jpg", ".txt"}

	for i := range 5 {
		filename := "file" + strconv.Itoa(i) + extensions[i]
		createTestFile(t, filepath.Join(srcDir, filename), []byte("fake content"))
	}

	opts := cli.Opts{
		Source:           srcDir,
		Dest:             destDir,
		Sort:             cli.SortOption{Primary: cli.CritMIMEType, Secondary: cli.CritExtension},
		Action:           "copy",
		Force:            "yes",
		DryRun:           false,
		ConflictStrategy: cli.ConflictSkip,
		Workers:          runtime.NumCPU(),
	}

	rls, err := opts.ParseToRules()
	if err != nil {
		t.Fatal(err)
	}

	sc := scanner.NewScanner()
	files, err := sc.Scan(opts.Source)
	if err != nil {
		t.Fatal(err)
	}

	plan := sorter.NewPlan(&opts)
	for _, f := range files {
		action := sorter.NewAction(f, rls, opts.Dest)
		plan.AddAction(action)
	}

	executor := sorter.NewExecutor(&plan, &opts)
	executor.Execute()

	assertFileExists(t, filepath.Join(destDir, "images", "jpg", "file0.jpg"))
	assertFileExists(t, filepath.Join(destDir, "documents", "pdf", "file1.pdf"))
	assertFileExists(t, filepath.Join(destDir, "images", "png", "file2.png"))
	assertFileExists(t, filepath.Join(destDir, "images", "jpg", "file3.jpg"))
	assertFileExists(t, filepath.Join(destDir, "documents", "txt", "file4.txt"))

	assertFileExists(t, filepath.Join(srcDir, "file0.jpg"))
}
