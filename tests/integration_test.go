package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lookinalx/fiona/internal/app"
	"github.com/lookinalx/fiona/internal/cli"
	"github.com/lookinalx/fiona/internal/config"
	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/rules"
	"github.com/lookinalx/fiona/internal/sorter"
	"github.com/lookinalx/fiona/internal/types"
	"github.com/lookinalx/fiona/ml"
)

// fionaBin is the real binary: scenarios check exit codes, stdin and signals.
var fionaBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "fiona-test-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fionaBin = filepath.Join(dir, "fiona")
	if runtime.GOOS == "windows" {
		fionaBin += ".exe"
	}

	args := []string{"build"}
	if raceEnabled {
		args = append(args, "-race")
	}
	args = append(args, "-o", fionaBin, "github.com/lookinalx/fiona/cmd/fiona")
	if out, err := exec.Command("go", args...).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "cannot build fiona: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// ── helpers ────────────────────────────────────────────────────────────

type result struct {
	stdout string
	stderr string
	code   int
}

// configDirs keeps one configuration and history folder per test, so that
// parallel tests do not see each other's runs and the user's own are never touched.
var configDirs sync.Map

func configDir(t *testing.T) string {
	t.Helper()
	if dir, ok := configDirs.Load(t); ok {
		return dir.(string)
	}
	dir := t.TempDir()
	configDirs.Store(t, dir)
	return dir
}

// fionaEnv is the environment of the binary under test.
func fionaEnv(t *testing.T) []string {
	return append(os.Environ(), config.EnvDir+"="+configDir(t))
}

// fiona runs the binary in an empty working directory and feeds it stdin.
func fiona(t *testing.T, stdin string, args ...string) result {
	t.Helper()
	return fionaIn(t, t.TempDir(), stdin, args...)
}

// fionaIn runs the binary in dir.
func fionaIn(t *testing.T, dir, stdin string, args ...string) result {
	t.Helper()

	cmd := exec.Command(fionaBin, args...)
	cmd.Dir = dir
	cmd.Env = fionaEnv(t)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	res := result{}
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("cannot run fiona: %v", err)
		}
		res.code = exitErr.ExitCode()
	}
	res.stdout, res.stderr = stdout.String(), stderr.String()

	if strings.Contains(res.stderr, "DATA RACE") {
		t.Errorf("data race in fiona %v:\n%s", args, res.stderr)
	}
	return res
}

// mustFiona fails the test if the command does not exit with the wanted code.
func mustFiona(t *testing.T, wantCode int, stdin string, args ...string) result {
	t.Helper()

	res := fiona(t, stdin, args...)
	if res.code != wantCode {
		t.Fatalf("fiona %v: exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", args, res.code, wantCode, res.stdout, res.stderr)
	}
	return res
}

func assertOutput(t *testing.T, output string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(output, w) {
			t.Errorf("output does not contain %q:\n%s", w, output)
		}
	}
}

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
	if _, err := os.Lstat(path); err != nil {
		t.Errorf("file doesn't exists: %s", path)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("path should not exist: %s", path)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("cannot read file: %v", err)
		return
	}
	if string(got) != want {
		t.Errorf("%s: content = %q, want %q", path, got, want)
	}
}

// snapshot maps every regular file under root to its content. FIONA's own
// state directory is left out, so that sorted trees can be compared.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()

	files := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == journal.StateDir {
			return filepath.SkipDir
		}
		if d.Type().IsRegular() {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			files[filepath.ToSlash(rel)] = string(content)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func assertSnapshot(t *testing.T, root string, want map[string]string) {
	t.Helper()

	got := snapshot(t, root)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s:\n got %v\nwant %v", root, got, want)
	}
}

// contents returns the set of file contents under root, whatever the names are.
func contents(t *testing.T, root string) map[string]int {
	t.Helper()

	set := make(map[string]int)
	for _, content := range snapshot(t, root) {
		set[content]++
	}
	return set
}

func journals(t *testing.T, dir string) []string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

// lastRun reads the only journal of dest.
func lastRun(t *testing.T, dest string) *journal.Run {
	t.Helper()

	paths := journals(t, filepath.Join(dest, journal.StateDir, journal.RunsDir))
	if len(paths) != 1 {
		t.Fatalf("expected 1 journal in %s, got %d", dest, len(paths))
	}
	run, err := journal.Read(paths[0])
	if err != nil {
		t.Fatalf("cannot read journal: %v", err)
	}
	return run
}

// sameNamed creates count files with the same name in different subdirectories.
func sameNamed(t *testing.T, root string, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		createTestFile(t, filepath.Join(root, fmt.Sprintf("dir%03d", i), "same.txt"), []byte(fmt.Sprintf("content %d", i)))
	}
}

// ── SORT ───────────────────────────────────────────────────────────────

// S1: repeated move with dest inside source plans nothing and loses nothing.
func TestS1_MoveTwice_DestInsideSource(t *testing.T) {
	t.Parallel()

	for _, flags := range [][]string{{"-r"}, {}} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			src := t.TempDir()
			dest := filepath.Join(src, "sorted")
			createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))
			createTestFile(t, filepath.Join(src, "report.pdf"), []byte("report"))

			args := append([]string{"sort", "-s", src, "-d", dest, "-a", "move", "-y"}, flags...)
			mustFiona(t, 0, "", args...)

			want := map[string]string{
				"sorted/images/photo.jpg":     "photo",
				"sorted/documents/report.pdf": "report",
			}
			assertSnapshot(t, src, want)

			second := mustFiona(t, 0, "", args...)

			assertOutput(t, second.stdout, "Nothing to sort in ", ": no files.")
			assertSnapshot(t, src, want)
			if n := len(journals(t, filepath.Join(dest, journal.StateDir, journal.RunsDir))); n != 1 {
				t.Errorf("a run that did nothing left a journal: %d journals", n)
			}
		})
	}
}

// S2: repeated recursive move with source == dest finds every file in place.
func TestS2_MoveTwice_SourceEqualsDest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "photo.jpg"), []byte("photo"))
	createTestFile(t, filepath.Join(dir, "inbox", "photo.jpg"), []byte("another photo"))
	createTestFile(t, filepath.Join(dir, "inbox", "report.pdf"), []byte("report"))

	args := []string{"sort", "-s", dir, "-d", dir, "-a", "move", "-r", "-y"}
	mustFiona(t, 0, "", args...)

	want := map[string]string{
		"images/photo.jpg":     "another photo",
		"images/photo_1.jpg":   "photo",
		"documents/report.pdf": "report",
	}
	assertSnapshot(t, dir, want)

	second := mustFiona(t, 0, "", args...)

	assertOutput(t, second.stdout, "Already in place: 3", "Nothing to do.")
	assertSnapshot(t, dir, want)
}

// S3: files with the same name from different subdirectories do not overwrite each other.
func TestS3_SameNameFromDifferentSubdirectories(t *testing.T) {
	t.Parallel()

	src, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(src, "trip", "IMG_0001.jpg"), []byte("trip"))
	createTestFile(t, filepath.Join(src, "wedding", "IMG_0001.jpg"), []byte("wedding"))

	res := mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-r", "-y")

	assertSnapshot(t, dest, map[string]string{
		"images/IMG_0001.jpg":   "trip",
		"images/IMG_0001_1.jpg": "wedding",
	})
	assertOutput(t, res.stdout, "Renamed on conflict: 1", "✓ Moved 2 files")
}

// S4: 200 files with the same name survive a parallel move with rename.
func TestS4_ManySameNamedFiles_Rename(t *testing.T) {
	t.Parallel()

	const numFiles, repeats = 200, 20

	for i := 0; i < repeats; i++ {
		src, dest := t.TempDir(), t.TempDir()
		sameNamed(t, src, numFiles)
		want := contents(t, src)

		res := mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-a", "move", "-r", "--on-conflict", "rename", "-w", "16", "-y")

		if left := snapshot(t, src); len(left) != 0 {
			t.Fatalf("repeat %d: %d files are left in source", i, len(left))
		}
		if got := snapshot(t, dest); len(got) != numFiles {
			t.Fatalf("repeat %d: %d files in dest, want %d", i, len(got), numFiles)
		}
		if got := contents(t, dest); !reflect.DeepEqual(got, want) {
			t.Fatalf("repeat %d: contents of the files are not preserved", i)
		}
		assertOutput(t, res.stdout, fmt.Sprintf("✓ Moved %d files", numFiles))
	}
}

// S5: with skip exactly one of 200 same-named files is moved, the rest stay.
func TestS5_ManySameNamedFiles_Skip(t *testing.T) {
	t.Parallel()

	const numFiles, repeats = 200, 20

	for i := 0; i < repeats; i++ {
		src, dest := t.TempDir(), t.TempDir()
		sameNamed(t, src, numFiles)
		before := snapshot(t, src)

		res := mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-a", "move", "-r", "--on-conflict", "skip", "-w", "16", "-y")

		assertOutput(t, res.stdout, fmt.Sprintf("Skipped, already exist: %d", numFiles-1), "✓ Moved 1 file ")

		left, moved := snapshot(t, src), snapshot(t, dest)
		if len(left) != numFiles-1 || len(moved) != 1 {
			t.Fatalf("repeat %d: %d files in source and %d in dest, want %d and 1", i, len(left), len(moved), numFiles-1)
		}
		for name, content := range left {
			if before[name] != content {
				t.Fatalf("repeat %d: %s changed", i, name)
			}
		}
		if moved["documents/same.txt"] != before["dir000/same.txt"] {
			t.Fatalf("repeat %d: moved file has content %q", i, moved["documents/same.txt"])
		}
	}
}

// S6: replace sends the old file to trash and undo brings it back.
func TestS6_ReplaceUsesTrashAndUndoRestores(t *testing.T) {
	t.Parallel()

	for _, action := range []string{"move", "copy"} {
		t.Run(action, func(t *testing.T) {
			src, dest := t.TempDir(), t.TempDir()
			createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("new photo"))
			createTestFile(t, filepath.Join(dest, "images", "photo.jpg"), []byte("old photo"))
			srcBefore, destBefore := snapshot(t, src), snapshot(t, dest)

			res := mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-a", action, "--on-conflict", "replace", "-y")

			assertOutput(t, res.stdout, "Replaced, old files go to trash: 1", "✓ ")
			run := lastRun(t, dest)
			assertFileContent(t, filepath.Join(dest, "images", "photo.jpg"), "new photo")
			assertFileContent(t, filepath.Join(dest, ".fiona", "trash", run.Header.RunID, "images", "photo.jpg"), "old photo")

			undo := mustFiona(t, 0, "", "undo", "--dest", dest, "-y")

			assertOutput(t, undo.stdout, "Restore from trash: 1", "restored 1 from trash")
			assertSnapshot(t, src, srcBefore)
			assertSnapshot(t, dest, destBefore)
			assertNotExists(t, filepath.Join(dest, ".fiona", "trash"))
		})
	}
}

// S7: undo after skip and rename never touches files that existed before the run.
func TestS7_UndoKeepsPreexistingFiles(t *testing.T) {
	t.Parallel()

	for _, action := range []string{"copy", "move"} {
		for _, strategy := range []string{"skip", "rename"} {
			t.Run(action+"/"+strategy, func(t *testing.T) {
				src, dest := t.TempDir(), t.TempDir()
				createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("new photo"))
				createTestFile(t, filepath.Join(src, "report.pdf"), []byte("new report"))
				createTestFile(t, filepath.Join(dest, "images", "photo.jpg"), []byte("old photo"))
				createTestFile(t, filepath.Join(dest, "images", "photo_1.jpg"), []byte("old photo 1"))
				srcBefore, destBefore := snapshot(t, src), snapshot(t, dest)

				mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-a", action, "--on-conflict", strategy, "-y")
				assertFileContent(t, filepath.Join(dest, "documents", "report.pdf"), "new report")

				undo := mustFiona(t, 0, "", "undo", "--dest", dest, "-y")

				assertOutput(t, undo.stdout, "✓ ")
				assertSnapshot(t, src, srcBefore)
				assertSnapshot(t, dest, destBefore)
			})
		}
	}
}

// S8: dry-run changes nothing, whatever other flags say.
func TestS8_DryRunChangesNothing(t *testing.T) {
	t.Parallel()

	for _, flags := range [][]string{{"--dry-run", "--yes"}, {"-n", "-y"}, {"--dry-run=true", "--yes"}} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			src := t.TempDir()
			dest := filepath.Join(t.TempDir(), "not", "created")
			createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))
			before := snapshot(t, src)

			args := append([]string{"sort", "-s", src, "-d", dest, "-a", "move"}, flags...)
			res := mustFiona(t, 0, "", args...)

			assertOutput(t, res.stdout, "images/", "1 file", "Dry run: nothing is changed.")
			assertSnapshot(t, src, before)
			assertNotExists(t, filepath.Dir(dest))
			assertNotExists(t, filepath.Join(src, ".fiona"))
		})
	}

	t.Run("existing dest", func(t *testing.T) {
		dir := t.TempDir()
		createTestFile(t, filepath.Join(dir, "photo.jpg"), []byte("photo"))

		mustFiona(t, 0, "", "sort", "-s", dir, "-d", dir, "-a", "move", "--dry-run", "--yes")

		assertSnapshot(t, dir, map[string]string{"photo.jpg": "photo"})
		assertNotExists(t, filepath.Join(dir, ".fiona"))
		assertNotExists(t, filepath.Join(dir, "images"))
	})
}

// S9: all invalid flags are reported together and nothing is created.
func TestS9_InvalidFlags(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))
	missingLog := filepath.Join(t.TempDir(), "no", "such", "log")

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "everything at once",
			args: []string{"-w", "0", "--on-conflict", "bogus", "--log", missingLog},
			want: []string{"invalid number of workers", "invalid on-conflict", "log path doesn't exist"},
		},
		{
			name: "negative workers",
			args: []string{"-w", "-3", "--on-conflict", "bogus", "--log", missingLog},
			want: []string{"invalid number of workers", "invalid on-conflict", "log path doesn't exist"},
		},
		{
			name: "zero workers only",
			args: []string{"-w", "0"},
			want: []string{"invalid number of workers"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "not", "created")

			args := append([]string{"sort", "-s", src, "-d", dest, "-y"}, tt.args...)
			res := mustFiona(t, 2, "", args...)

			assertOutput(t, res.stderr, tt.want...)
			assertNotExists(t, filepath.Dir(dest))
			assertSnapshot(t, src, map[string]string{"photo.jpg": "photo"})
		})
	}
}

// S10: symlinks are skipped, not followed, and do not break the run.
func TestS10_Symlinks(t *testing.T) {
	t.Parallel()

	outside := t.TempDir()
	createTestFile(t, filepath.Join(outside, "secret.txt"), []byte("secret"))
	createTestFile(t, filepath.Join(outside, "dir", "inside.txt"), []byte("inside"))

	src, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))
	links := map[string]string{
		"broken.txt":  filepath.Join(outside, "missing.txt"),
		"to_dir":      filepath.Join(outside, "dir"),
		"to_file.txt": filepath.Join(outside, "secret.txt"),
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(src, name)); err != nil {
			t.Skipf("cannot create symlink: %v", err)
		}
	}

	res := mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-a", "move", "-r", "-y")

	assertOutput(t, res.stdout, "Skipped by scanner: 3 (3 symlinks)", "✓ Moved 1 file ")
	assertSnapshot(t, dest, map[string]string{"images/photo.jpg": "photo"})
	for name := range links {
		if info, err := os.Lstat(filepath.Join(src, name)); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("symlink %s is not left in place", name)
		}
	}
	assertSnapshot(t, outside, map[string]string{"secret.txt": "secret", "dir/inside.txt": "inside"})
}

// S11: bundles and projects are never taken apart.
func TestS11_OpaqueDirectories(t *testing.T) {
	t.Parallel()

	for _, flags := range [][]string{{}, {"-r"}, {"-r", "--hidden"}} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			src, dest := t.TempDir(), t.TempDir()
			createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))
			createTestFile(t, filepath.Join(src, "myproject", ".git", "HEAD"), []byte("ref: refs/heads/main"))
			createTestFile(t, filepath.Join(src, "myproject", "main.c"), []byte("int main() {}"))
			createTestFile(t, filepath.Join(src, "myproject", "docs", "readme.txt"), []byte("readme"))
			createTestFile(t, filepath.Join(src, "Foo.app", "Contents", "Info.plist"), []byte("plist"))
			createTestFile(t, filepath.Join(src, "Foo.app", "Contents", "icon.png"), []byte("icon"))
			createTestFile(t, filepath.Join(src, "unzipped", "go.mod"), []byte("module unzipped"))
			createTestFile(t, filepath.Join(src, "unzipped", "logo.png"), []byte("logo"))

			args := append([]string{"sort", "-s", src, "-d", dest, "-a", "move", "-y"}, flags...)
			mustFiona(t, 0, "", args...)

			assertSnapshot(t, dest, map[string]string{"images/photo.jpg": "photo"})
			assertSnapshot(t, src, map[string]string{
				"myproject/.git/HEAD":         "ref: refs/heads/main",
				"myproject/main.c":            "int main() {}",
				"myproject/docs/readme.txt":   "readme",
				"Foo.app/Contents/Info.plist": "plist",
				"Foo.app/Contents/icon.png":   "icon",
				"unzipped/go.mod":             "module unzipped",
				"unzipped/logo.png":           "logo",
			})
		})
	}
}

// S16: a failed operation is reported and makes the exit code 1.
func TestS16_FailedOperation(t *testing.T) {
	t.Parallel()

	if os.Getuid() == 0 {
		t.Skip("root can write to any directory")
	}

	src, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))
	createTestFile(t, filepath.Join(src, "report.pdf"), []byte("report"))

	locked := filepath.Join(dest, "images")
	if err := os.MkdirAll(locked, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0755)

	res := mustFiona(t, 1, "", "sort", "-s", src, "-d", dest, "-a", "move", "-y")

	assertOutput(t, res.stdout, "! Moved 1 of 2 files, 1 failed:", "✗ photo.jpg: ", "Undo: fiona undo")
	assertSnapshot(t, src, map[string]string{"photo.jpg": "photo"})
	assertSnapshot(t, dest, map[string]string{"documents/report.pdf": "report"})

	run := lastRun(t, dest)
	if run.Footer == nil || run.Footer.OK != 1 || run.Footer.Failed != 1 {
		t.Errorf("footer = %+v", run.Footer)
	}
}

func TestSort_Defaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "photo.jpg"), []byte("photo"))
	createTestFile(t, filepath.Join(dir, ".hidden.jpg"), []byte("hidden"))
	createTestFile(t, filepath.Join(dir, "sub", "nested.jpg"), []byte("nested"))
	createTestFile(t, filepath.Join(dir, "images", "photo.jpg"), []byte("old photo"))

	// no flags: move into the source itself, top level only, no hidden files, rename on conflict
	mustFiona(t, 0, "", "sort", dir, "-y")

	assertSnapshot(t, dir, map[string]string{
		"images/photo.jpg":   "old photo",
		"images/photo_1.jpg": "photo",
		".hidden.jpg":        "hidden",
		"sub/nested.jpg":     "nested",
	})
}

func TestSort_RecursiveAndHiddenRestoreOldBehaviour(t *testing.T) {
	t.Parallel()

	src, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))
	createTestFile(t, filepath.Join(src, ".hidden.png"), []byte("hidden"))
	createTestFile(t, filepath.Join(src, ".config", "nested.gif"), []byte("nested"))

	mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-a", "move", "-r", "--hidden", "-y")

	assertSnapshot(t, dest, map[string]string{
		"images/photo.jpg":   "photo",
		"images/.hidden.png": "hidden",
		"images/nested.gif":  "nested",
	})
}

func TestSort_Confirmation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		stdin    string
		executed bool
		message  string
	}{
		{"y", "y\n", true, ""},
		{"yes with windows line ending", "YES\r\n", true, ""},
		{"n", "n\n", false, "Cancelled: nothing is changed."},
		{"empty line", "\n", false, "Cancelled: nothing is changed."},
		{"end of input", "", false, "No answer: nothing is changed. Use --yes to run without confirmation."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := t.TempDir()
			dest := filepath.Join(t.TempDir(), "sorted")
			createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))

			res := mustFiona(t, 0, tt.stdin, "sort", "-s", src, "-d", dest, "-a", "move")

			assertOutput(t, res.stdout, "Move 1 file (5 B) into 1 folder in ", "? [y/N/l]")
			if tt.executed {
				assertSnapshot(t, dest, map[string]string{"images/photo.jpg": "photo"})
				assertSnapshot(t, src, map[string]string{})
			} else {
				assertOutput(t, res.stdout, tt.message)
				assertSnapshot(t, src, map[string]string{"photo.jpg": "photo"})
				assertNotExists(t, dest)
			}
		})
	}
}

func TestSort_RemovedForceFlag(t *testing.T) {
	t.Parallel()

	src, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))

	res := mustFiona(t, 2, "", "sort", "-s", src, "-d", dest, "-a", "move", "--force", "yes")

	assertOutput(t, res.stderr, "unknown flag: --force")
	assertSnapshot(t, src, map[string]string{"photo.jpg": "photo"})
	assertSnapshot(t, dest, map[string]string{})
}

func TestSort_JournalLocation(t *testing.T) {
	t.Parallel()

	t.Run("default", func(t *testing.T) {
		src, dest := t.TempDir(), t.TempDir()
		createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))

		res := mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-y")

		run := lastRun(t, dest)
		resolvedDest, err := cli.ResolvePath(dest)
		if err != nil {
			t.Fatal(err)
		}
		if run.Header.Dest != resolvedDest || run.Header.FileAction != "move" || run.Header.OnConflict != "rename" || run.Header.FionaVersion != "0.5.0" {
			t.Errorf("header = %+v", run.Header)
		}
		if _, ok := journal.RunTime(run.Header.RunID); !ok || filepath.Base(run.Path) != run.Header.RunID+".jsonl" {
			t.Errorf("run id = %q, journal = %q", run.Header.RunID, run.Path)
		}
		if run.Footer == nil || run.Footer.OK != 1 {
			t.Errorf("footer = %+v", run.Footer)
		}
		assertOutput(t, res.stdout, "Undo: fiona undo\n")
	})

	t.Run("custom directory", func(t *testing.T) {
		src, dest, logDir := t.TempDir(), t.TempDir(), t.TempDir()
		createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))

		mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-a", "move", "--log", logDir, "-y")

		paths := journals(t, logDir)
		if len(paths) != 1 {
			t.Fatalf("expected 1 journal in the custom directory, got %d", len(paths))
		}
		assertNotExists(t, filepath.Join(dest, ".fiona"))

		mustFiona(t, 0, "", "undo", "--log", paths[0], "-y")
		assertSnapshot(t, src, map[string]string{"photo.jpg": "photo"})
	})

	t.Run("every run has its own journal", func(t *testing.T) {
		src, dest := t.TempDir(), t.TempDir()
		createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))

		mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "--copy", "-y")
		mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "--copy", "-y")

		if n := len(journals(t, filepath.Join(dest, journal.StateDir, journal.RunsDir))); n != 2 {
			t.Errorf("expected 2 journals, got %d", n)
		}
	})
}

func TestSort_CleansTrashOlderThanFiveDays(t *testing.T) {
	t.Parallel()

	src, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))

	trash := filepath.Join(dest, ".fiona", "trash")
	expired := journal.NewRunID(time.Now().Add(-6 * 24 * time.Hour))
	recent := journal.NewRunID(time.Now().Add(-4 * 24 * time.Hour))
	createTestFile(t, filepath.Join(trash, expired, "images", "old.jpg"), []byte("expired"))
	createTestFile(t, filepath.Join(trash, recent, "images", "old.jpg"), []byte("recent"))

	// a dry run cleans nothing
	mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "--dry-run")
	assertFileExists(t, filepath.Join(trash, expired, "images", "old.jpg"))

	res := mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-y")

	// cleaning is silent unless it fails
	if strings.Contains(res.stdout+res.stderr, "trash") {
		t.Errorf("unexpected output about the trash:\n%s%s", res.stdout, res.stderr)
	}
	assertNotExists(t, filepath.Join(trash, expired))
	assertFileContent(t, filepath.Join(trash, recent, "images", "old.jpg"), "recent")
}

func TestVersion(t *testing.T) {
	t.Parallel()

	res := mustFiona(t, 0, "", "--version")
	assertOutput(t, res.stdout, "FIONA 0.5.0")
}

// ── UNDO ───────────────────────────────────────────────────────────────

// forgeJournal writes a journal by hand, as an attacker or a bug would.
func forgeJournal(t *testing.T, source, dest, fileAction string, records ...string) string {
	t.Helper()

	header := fmt.Sprintf(`{"type":"header","version":2,"run_id":"20261010-153012-a1b2","started_at":"2026-10-10T15:30:12Z","source":%q,"dest":%q,"file_action":%q,"on_conflict":"rename","fiona_version":"0.4.0"}`,
		source, dest, fileAction)

	path := filepath.Join(t.TempDir(), "20261010-153012-a1b2.jsonl")
	lines := append([]string{header}, records...)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// S12: a forged journal cannot make undo touch foreign files.
func TestS12_ForgedJournal(t *testing.T) {
	t.Parallel()

	t.Run("dst outside dest rejects the whole journal", func(t *testing.T) {
		source, dest, elsewhere := t.TempDir(), t.TempDir(), t.TempDir()
		victim := filepath.Join(elsewhere, "victim.txt")
		createTestFile(t, victim, []byte("victim"))
		honest := filepath.Join(dest, "documents", "honest.txt")
		createTestFile(t, honest, []byte("honest"))

		path := forgeJournal(t, source, dest, "copy",
			fmt.Sprintf(`{"type":"op","seq":0,"kind":"copy","src":%q,"dst":%q,"size":6}`, filepath.Join(source, "honest.txt"), honest),
			fmt.Sprintf(`{"type":"op","seq":1,"kind":"copy","src":%q,"dst":%q,"size":6}`, filepath.Join(source, "victim.txt"), victim),
			`{"type":"done","seq":0,"status":"ok","size":6}`,
			`{"type":"done","seq":1,"status":"ok","size":6}`,
		)

		res := mustFiona(t, 1, "", "undo", "--log", path, "-y")

		assertOutput(t, res.stderr, "journal rejected", "outside of dest")
		assertFileContent(t, victim, "victim")
		assertFileContent(t, honest, "honest")
	})

	t.Run("size mismatch skips the file", func(t *testing.T) {
		source, dest := t.TempDir(), t.TempDir()
		mine := filepath.Join(dest, "documents", "mine.txt")
		createTestFile(t, mine, []byte("my own file, not a copy made by fiona"))

		path := forgeJournal(t, source, dest, "copy",
			fmt.Sprintf(`{"type":"op","seq":0,"kind":"copy","src":%q,"dst":%q,"size":6}`, filepath.Join(source, "mine.txt"), mine),
			`{"type":"done","seq":0,"status":"ok","size":6,"mtime":"2026-10-10T15:30:12Z"}`,
		)

		res := mustFiona(t, 0, "", "undo", "--log", path, "-y")

		assertOutput(t, res.stdout, "Skip, changed after the run: 1", "! Removed 0 of 1 copy, 1 skipped:", "⊘ documents/mine.txt: skipped")
		assertFileContent(t, mine, "my own file, not a copy made by fiona")
	})

	t.Run("the question is always asked", func(t *testing.T) {
		src, dest := t.TempDir(), t.TempDir()
		createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))
		mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-a", "move", "-y")
		sorted := snapshot(t, dest)

		for stdin, message := range map[string]string{"": "No answer: nothing is changed.", "n\n": "Cancelled: nothing is changed."} {
			res := mustFiona(t, 0, stdin, "undo", "--dest", dest)

			assertOutput(t, res.stdout, "Undo the run from today ", "(moved 1 file from ", "? [y/N]", message)
			assertSnapshot(t, dest, sorted)
		}

		res := mustFiona(t, 0, "y\n", "undo", "--dest", dest)

		assertOutput(t, res.stdout, "? [y/N]", "✓ Restored 1 file")
		assertSnapshot(t, src, map[string]string{"photo.jpg": "photo"})
	})

	t.Run("journal of another dest is rejected", func(t *testing.T) {
		src, dest, other := t.TempDir(), t.TempDir(), t.TempDir()
		createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))
		mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-a", "move", "-y")

		run := lastRun(t, dest)
		data, err := os.ReadFile(run.Path)
		if err != nil {
			t.Fatal(err)
		}
		createTestFile(t, filepath.Join(other, ".fiona", "runs", filepath.Base(run.Path)), data)

		res := mustFiona(t, 1, "", "undo", "--dest", other, "-y")

		assertOutput(t, res.stderr, "journal rejected")
		assertSnapshot(t, dest, map[string]string{"images/photo.jpg": "photo"})
	})

	t.Run("old journal format is rejected", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "fiona_logs.json")
		createTestFile(t, path, []byte(`{"file_action":"move","logs":{"0":{"SourcePath":"/a/b.txt","DestPath":"/c","Status":"succeeded"}}}`))

		res := mustFiona(t, 1, "", "undo", "--log", path, "-y")

		assertOutput(t, res.stderr, "old format", "safe undo is impossible")
	})
}

// S13: after kill -9 undo returns every file that was really moved.
func TestS13_UndoOfKilledRun(t *testing.T) {
	t.Parallel()

	src, dest := t.TempDir(), t.TempDir()
	for i := 0; i < 30; i++ {
		createTestFile(t, filepath.Join(src, fmt.Sprintf("file%02d.txt", i)), []byte(fmt.Sprintf("content %d", i)))
	}
	before := snapshot(t, src)

	mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-a", "move", "-y")

	// as after kill -9: half of the results and the footer never reached the disk
	run := lastRun(t, dest)
	data, err := os.ReadFile(run.Path)
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	done := 0
	for _, line := range strings.SplitAfter(string(data), "\n") {
		switch {
		case strings.Contains(line, `"type":"footer"`):
			continue
		case strings.Contains(line, `"type":"done"`):
			done++
			if done%2 == 0 {
				continue
			}
		}
		kept = append(kept, line)
	}
	if err := os.WriteFile(run.Path, []byte(strings.Join(kept, "")+`{"type":"done","seq":1,"stat`), 0644); err != nil {
		t.Fatal(err)
	}

	res := mustFiona(t, 0, "", "undo", "--dest", dest, "-y")

	assertOutput(t, res.stdout, "The run was interrupted:", "✓ Restored 30 files")
	assertSnapshot(t, src, before)
	assertSnapshot(t, dest, map[string]string{})
}

// S17: a run is undone only once.
func TestS17_UndoTwice(t *testing.T) {
	t.Parallel()

	src, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))

	mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-a", "move", "-y")
	run := lastRun(t, dest)

	mustFiona(t, 0, "", "undo", "--log", run.Path, "-y")
	assertSnapshot(t, src, map[string]string{"photo.jpg": "photo"})

	// someone sorts the same file by hand in between
	createTestFile(t, filepath.Join(dest, "images", "photo.jpg"), []byte("photo"))

	byLog := mustFiona(t, 1, "", "undo", "--log", run.Path, "-y")
	assertOutput(t, byLog.stderr, "already undone")

	byDest := mustFiona(t, 0, "", "undo", "--dest", dest, "-y")
	assertOutput(t, byDest.stdout, "Nothing to undo: the last run in ", "is already undone")

	assertFileContent(t, filepath.Join(dest, "images", "photo.jpg"), "photo")
	assertFileContent(t, filepath.Join(src, "photo.jpg"), "photo")
}

func TestUndo_DryRun(t *testing.T) {
	t.Parallel()

	src, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(src, "photo.jpg"), []byte("photo"))
	mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-a", "move", "-y")

	res := mustFiona(t, 0, "", "undo", "--dest", dest, "-n", "-y")

	assertOutput(t, res.stdout, "Would undo the run from today ", "(moved 1 file from ", "Dry run: nothing is changed.")
	assertSnapshot(t, dest, map[string]string{"images/photo.jpg": "photo"})

	// the run is still there to be undone
	mustFiona(t, 0, "", "undo", "--dest", dest, "-y")
	assertSnapshot(t, src, map[string]string{"photo.jpg": "photo"})
}

func TestUndo_LatestRunFirst(t *testing.T) {
	t.Parallel()

	src, dest := t.TempDir(), t.TempDir()

	createTestFile(t, filepath.Join(src, "first.jpg"), []byte("first"))
	mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-a", "move", "-y")

	// run identifiers have a resolution of one second
	time.Sleep(1100 * time.Millisecond)

	createTestFile(t, filepath.Join(src, "second.jpg"), []byte("second"))
	mustFiona(t, 0, "", "sort", "-s", src, "-d", dest, "-a", "move", "-y")

	mustFiona(t, 0, "", "undo", "--dest", dest, "-y")
	assertSnapshot(t, src, map[string]string{"second.jpg": "second"})
	assertSnapshot(t, dest, map[string]string{"images/first.jpg": "first"})

	mustFiona(t, 0, "", "undo", "--dest", dest, "-y")
	assertSnapshot(t, src, map[string]string{"first.jpg": "first", "second.jpg": "second"})
	assertSnapshot(t, dest, map[string]string{})
}

func TestUndo_NothingToUndo(t *testing.T) {
	t.Parallel()

	res := mustFiona(t, 0, "", "undo", "--dest", t.TempDir(), "-y")
	assertOutput(t, res.stdout, "Nothing to undo: no runs in ")

	// without flags the history is used, then the current folder
	res = mustFiona(t, 0, "", "undo", "-y")
	assertOutput(t, res.stdout, "Nothing to undo: no runs yet.")
}

// ── ML ─────────────────────────────────────────────────────────────────

// mockMLServer starts a test HTTP server that mimics fiona-classifier.
// tagMap maps filename → ML category returned by /classify.
func mockMLServer(t *testing.T, tagMap map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		case "/classify":
			var body struct {
				Paths []string `json:"paths"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}

			result := make(map[string]string, len(body.Paths))
			for _, path := range body.Paths {
				name := filepath.Base(path)
				if tag, ok := tagMap[name]; ok {
					result[path] = tag
				} else {
					result[path] = "__not_image__"
				}
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(result)

		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

// runMLSort runs the pipeline of fiona sort --to "{ml}" with a mock classifier.
func runMLSort(t *testing.T, srcDir, destDir, serverURL string) sorter.Summary {
	t.Helper()

	set, err := rules.Compile([]rules.Rule{{To: "{ml}"}})
	if err != nil {
		t.Fatal(err)
	}
	req := app.Request{
		Source:     srcDir,
		Dest:       destDir,
		Rules:      set,
		Action:     "copy",
		OnConflict: types.OnConflictSkip,
	}

	scanned, err := app.Scan(req)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if err := app.Classify(context.Background(), app.NeedsML(scanned, set), app.Client(serverURL)); err != nil {
		t.Fatalf("Classify: %v", err)
	}

	plan := app.Plan(scanned, req, journal.NewRunID(time.Now()))
	out, err := app.Execute(context.Background(), plan, app.ExecOptions{Workers: runtime.NumCPU(), LogDir: t.TempDir(), Version: "test"}, silentReporter{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return out.Summary
}

type silentReporter struct{}

func (silentReporter) Start(int)                {}
func (silentReporter) Done(types.Action, error) {}
func (silentReporter) Finish(sorter.Summary)    {}

// S15: a category name from the ML config cannot lead the write out of dest.
func TestS15_MLCategoryCannotEscapeDest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	srcDir := filepath.Join(root, "a", "b", "src")
	destDir := filepath.Join(root, "a", "b", "dest")
	createTestFile(t, filepath.Join(srcDir, "beach.jpg"), []byte("fake jpg"))
	createTestFile(t, filepath.Join(srcDir, "cat.jpg"), []byte("fake jpg"))

	srv := mockMLServer(t, map[string]string{
		"beach.jpg": "../../x",
		"cat.jpg":   "..",
	})
	defer srv.Close()

	summary := runMLSort(t, srcDir, destDir, srv.URL)

	if summary.Copied != 2 || summary.Failed != 0 || summary.Rejected != 0 {
		t.Errorf("summary = %+v", summary)
	}
	assertSnapshot(t, destDir, map[string]string{
		".._.._x/beach.jpg": "fake jpg",
		"_other/cat.jpg":    "fake jpg",
	})
	assertNotExists(t, filepath.Join(root, "a", "x"))
	assertNotExists(t, filepath.Join(root, "x"))
	assertNotExists(t, filepath.Join(root, "a", "b", "cat.jpg"))
}

// TestEndToEnd_MLSort_ImagesCategorisedByMLTag verifies that images are
// placed in folders matching the ML tag returned by the classifier.
func TestEndToEnd_MLSort_ImagesCategorisedByMLTag(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	createTestFile(t, filepath.Join(srcDir, "dog.jpg"), []byte("fake jpg"))
	createTestFile(t, filepath.Join(srcDir, "cat.jpg"), []byte("fake jpg"))
	createTestFile(t, filepath.Join(srcDir, "pizza.jpg"), []byte("fake jpg"))

	tagMap := map[string]string{
		"dog.jpg":   "animals",
		"cat.jpg":   "pets",
		"pizza.jpg": "food",
	}

	srv := mockMLServer(t, tagMap)
	defer srv.Close()

	runMLSort(t, srcDir, destDir, srv.URL)

	assertFileExists(t, filepath.Join(destDir, "animals", "dog.jpg"))
	assertFileExists(t, filepath.Join(destDir, "pets", "cat.jpg"))
	assertFileExists(t, filepath.Join(destDir, "food", "pizza.jpg"))
}

// TestEndToEnd_MLSort_NonImageFallsBackToMimeType verifies that files
// tagged __not_image__ are sorted by their MIME type instead.
func TestEndToEnd_MLSort_NonImageFallsBackToMimeType(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	createTestFile(t, filepath.Join(srcDir, "report.pdf"), []byte("fake pdf"))
	createTestFile(t, filepath.Join(srcDir, "notes.txt"), []byte("fake txt"))

	// mock returns __not_image__ for non-images
	tagMap := map[string]string{}

	srv := mockMLServer(t, tagMap)
	defer srv.Close()

	runMLSort(t, srcDir, destDir, srv.URL)

	// non-images fall back to MIME type — both are documents
	assertFileExists(t, filepath.Join(destDir, "documents", "report.pdf"))
	assertFileExists(t, filepath.Join(destDir, "documents", "notes.txt"))
}

// TestEndToEnd_MLSort_MixedFiles verifies correct handling of a batch
// containing both images (ML-tagged) and non-images (fallback).
func TestEndToEnd_MLSort_MixedFiles(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	createTestFile(t, filepath.Join(srcDir, "vacation.jpg"), []byte("fake jpg"))
	createTestFile(t, filepath.Join(srcDir, "contract.pdf"), []byte("fake pdf"))
	createTestFile(t, filepath.Join(srcDir, "sunset.png"), []byte("fake png"))

	tagMap := map[string]string{
		"vacation.jpg": "travel",
		"sunset.png":   "nature",
		// contract.pdf → __not_image__ (not in map)
	}

	srv := mockMLServer(t, tagMap)
	defer srv.Close()

	runMLSort(t, srcDir, destDir, srv.URL)

	assertFileExists(t, filepath.Join(destDir, "travel", "vacation.jpg"))
	assertFileExists(t, filepath.Join(destDir, "nature", "sunset.png"))
	assertFileExists(t, filepath.Join(destDir, "documents", "contract.pdf"))
}

// TestEndToEnd_MLSort_EmptyDirectory verifies that an empty source dir
// completes without errors.
func TestEndToEnd_MLSort_EmptyDirectory(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	srv := mockMLServer(t, map[string]string{})
	defer srv.Close()

	// should not panic or error
	runMLSort(t, srcDir, destDir, srv.URL)

	entries, err := os.ReadDir(destDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty destDir, got %d entries", len(entries))
	}
}

// TestEndToEnd_MLSort_ServerReturnsError verifies that a classifier error
// is propagated and execution does not proceed silently.
func TestEndToEnd_MLSort_ServerReturnsError(t *testing.T) {
	srcDir := t.TempDir()

	createTestFile(t, filepath.Join(srcDir, "photo.jpg"), []byte("fake jpg"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/classify":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"error": "model not loaded"})
		}
	}))
	defer srv.Close()

	client := ml.NewClient(srv.URL)
	_, err := client.Classify([]string{filepath.Join(srcDir, "photo.jpg")})
	if err == nil {
		t.Error("expected error when classifier returns error key")
	}
}

func TestEndToEnd_MLSort_CustomConfig(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	configDir := t.TempDir()

	createTestFile(t, filepath.Join(srcDir, "beach.jpg"), []byte("fake jpg"))
	createTestFile(t, filepath.Join(srcDir, "cat.jpg"), []byte("fake jpg"))
	createTestFile(t, filepath.Join(srcDir, "report.pdf"), []byte("fake pdf"))

	configPath := filepath.Join(configDir, "custom_categories.json")
	configData := []byte(`{
		"vacation_2024": "photos from summer vacation at the beach in 2024",
		"my_pets": "photos of my cat and other pet animals at home"
	}`)
	createTestFile(t, configPath, configData)

	tagMap := map[string]string{
		"beach.jpg": "vacation_2024",
		"cat.jpg":   "my_pets",
	}

	srv := mockMLServer(t, tagMap)
	defer srv.Close()

	runMLSort(t, srcDir, destDir, srv.URL)

	assertFileExists(t, filepath.Join(destDir, "vacation_2024", "beach.jpg"))
	assertFileExists(t, filepath.Join(destDir, "my_pets", "cat.jpg"))
	assertFileExists(t, filepath.Join(destDir, "documents", "report.pdf"))
}
