//go:build unix

package tests

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lookinalx/fiona/internal/journal"
)

// S14: SIGINT in the middle of a run gives a non-zero exit code and a run that can be undone.
func TestS14_InterruptedRunCanBeUndone(t *testing.T) {
	t.Parallel()

	// if the machine is fast enough to finish before the signal arrives, try again with more files
	for numFiles := 3000; numFiles <= 24000; numFiles *= 2 {
		if interruptAndUndo(t, numFiles) {
			return
		}
	}
	t.Fatal("the run finished before it could be interrupted")
}

func interruptAndUndo(t *testing.T, numFiles int) bool {
	t.Helper()

	src, dest := t.TempDir(), t.TempDir()
	for i := 0; i < numFiles; i++ {
		createTestFile(t, filepath.Join(src, fmt.Sprintf("file%05d.txt", i)), []byte(fmt.Sprintf("content %d", i)))
	}
	before := snapshot(t, src)

	cmd := exec.Command(fionaBin, "sort", "-s", src, "-d", dest, "-a", "move", "-w", "1", "-y")
	cmd.Dir = t.TempDir()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("cannot start fiona: %v", err)
	}

	// wait until the journal shows the first finished operation
	runsDir := filepath.Join(dest, journal.StateDir, journal.RunsDir)
	deadline := time.Now().Add(time.Minute)
	for started := false; !started; {
		if time.Now().After(deadline) {
			cmd.Process.Kill()
			t.Fatalf("fiona did not start executing:\n%s\n%s", stdout.String(), stderr.String())
		}
		for _, path := range journals(t, runsDir) {
			if data, err := os.ReadFile(path); err == nil && bytes.Contains(data, []byte(`"type":"done"`)) {
				started = true
			}
		}
		time.Sleep(200 * time.Microsecond)
	}

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("cannot send SIGINT: %v", err)
	}
	waitErr := cmd.Wait()

	if strings.Contains(stderr.String(), "DATA RACE") {
		t.Errorf("data race in fiona:\n%s", stderr.String())
	}

	run := lastRun(t, dest)
	if run.Footer != nil {
		return false
	}

	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) || exitErr.ExitCode() != 1 {
		t.Errorf("exit = %v, want exit code 1\nstdout:\n%s", waitErr, stdout.String())
	}
	assertOutput(t, stdout.String(), "Execution is interrupted.")

	left, moved := snapshot(t, src), snapshot(t, dest)
	if len(left) == 0 || len(moved) == 0 || len(left)+len(moved) != numFiles {
		t.Errorf("%d files in source and %d in dest after the interrupt, want both and %d in total", len(left), len(moved), numFiles)
	}

	// the journal stays consistent: a file is in dest exactly when it has a result
	done := 0
	for _, op := range run.Ops {
		if op.Result != nil {
			done++
		}
	}
	if done != len(moved) {
		t.Errorf("journal has %d results, but %d files are in dest", done, len(moved))
	}

	res := mustFiona(t, 0, "", "undo", "--dest", dest, "-y")

	assertOutput(t, res.stdout, "The run was interrupted", fmt.Sprintf("moved back: %d", len(moved)), "errors: 0")
	assertSnapshot(t, src, before)
	assertSnapshot(t, dest, map[string]string{})

	return true
}
