//go:build unix

package scanner

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestScan_SpecialFilesAreSkipped(t *testing.T) {
	tfs := NewTestFileSystem(t)
	tfs.CreateFile("file.txt", "content")
	fifo := filepath.Join(tfs.Root, "pipe")
	if err := syscall.Mkfifo(fifo, 0644); err != nil {
		t.Skipf("cannot create fifo: %v", err)
	}

	names, res := scanNames(t, tfs.Root, Options{})

	assertNames(t, names, "file.txt")
	if len(res.Skipped) != 1 || res.Skipped[0].Path != fifo || res.Skipped[0].Reason != ReasonNotRegular {
		t.Errorf("skipped = %v, want %s with reason %q", res.Skipped, fifo, ReasonNotRegular)
	}
}
