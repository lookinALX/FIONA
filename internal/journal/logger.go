package journal

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lookinalx/fiona/internal/types"
)

// Writer is a write-ahead journal of one run: the whole plan first, results as they come.
type Writer struct {
	mu   sync.Mutex
	f    *os.File
	path string
}

// Create opens a new journal file <dir>/<run-id>.jsonl. The file must not exist yet.
func Create(dir, runID string) (*Writer, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create journal directory: %w", err)
	}

	path := filepath.Join(dir, runID+fileExt)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create journal file: %w", err)
	}

	return &Writer{f: f, path: path}, nil
}

func (w *Writer) Path() string {
	return w.path
}

// Begin writes the header and the whole plan and flushes them to disk.
func (w *Writer) Begin(header Header, mkdirs []string, actions []types.Action) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	buf := bufio.NewWriter(w.f)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)

	err := enc.Encode(headerRecord{
		Type:         typeHeader,
		Version:      Version,
		RunID:        header.RunID,
		StartedAt:    header.StartedAt.UTC().Truncate(time.Second),
		Source:       header.Source,
		Dest:         header.Dest,
		FileAction:   header.FileAction,
		OnConflict:   header.OnConflict,
		FionaVersion: header.FionaVersion,
	})
	if err != nil {
		return fmt.Errorf("failed to write journal: %w", err)
	}

	for _, dir := range mkdirs {
		if err := enc.Encode(mkdirRecord{Type: typeMkdir, Path: dir}); err != nil {
			return fmt.Errorf("failed to write journal: %w", err)
		}
	}

	for _, action := range actions {
		rec := opRecord{
			Type:     typeOp,
			Seq:      action.Seq,
			Kind:     string(action.Kind),
			Src:      action.SourcePath,
			Dst:      action.TargetPath,
			Conflict: action.Conflict,
			Trash:    action.TrashPath,
			Note:     action.Note,
		}
		if action.Executable() {
			size, mtime := action.Size, action.ModTime.UTC()
			rec.Size, rec.Mtime = &size, &mtime
		}
		if err := enc.Encode(rec); err != nil {
			return fmt.Errorf("failed to write journal: %w", err)
		}
	}

	if err := buf.Flush(); err != nil {
		return fmt.Errorf("failed to write journal: %w", err)
	}
	if err := w.f.Sync(); err != nil {
		return fmt.Errorf("failed to sync journal: %w", err)
	}

	return nil
}

// Done records the result of one operation. size and mtime are the actual
// values of the target file after the operation and are ignored on failure.
func (w *Writer) Done(seq int, status string, opErr error, size int64, mtime time.Time) error {
	rec := doneRecord{Type: typeDone, Seq: seq, Status: status}
	if status == types.StatusOK {
		mtime = mtime.UTC()
		rec.Size, rec.Mtime = &size, &mtime
	}
	if opErr != nil {
		rec.Error = opErr.Error()
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	return w.writeLine(rec)
}

// Close writes the footer, unless the run was interrupted, and flushes the journal to disk.
func (w *Writer) Close(summary Summary) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var errs []error
	if !summary.Interrupted {
		errs = append(errs, w.writeLine(footerRecord{
			Type:       typeFooter,
			FinishedAt: time.Now().UTC().Truncate(time.Second),
			OK:         summary.OK,
			Failed:     summary.Failed,
			Skipped:    summary.Skipped,
		}))
	}
	errs = append(errs, w.f.Sync(), w.f.Close())

	return errors.Join(errs...)
}

func (w *Writer) writeLine(rec any) error {
	line, err := marshalLine(rec)
	if err != nil {
		return err
	}
	if _, err := w.f.Write(line); err != nil {
		return fmt.Errorf("failed to write journal: %w", err)
	}
	return nil
}

func marshalLine(rec any) ([]byte, error) {
	line, err := json.Marshal(rec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal journal record: %w", err)
	}
	return append(line, '\n'), nil
}

// MarkUndone appends the record that makes a repeated undo of this journal impossible.
func MarkUndone(path string, at time.Time) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("failed to open journal file: %w", err)
	}
	defer f.Close()

	line, err := marshalLine(undoneRecord{Type: typeUndone, At: at.UTC().Truncate(time.Second)})
	if err != nil {
		return err
	}

	// a run killed in the middle of a write leaves a torn last line without a newline
	if size, err := f.Seek(0, io.SeekEnd); err == nil && size > 0 {
		last := make([]byte, 1)
		if _, err := f.ReadAt(last, size-1); err == nil && last[0] != '\n' {
			line = append([]byte{'\n'}, line...)
		}
	}

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("failed to write journal: %w", err)
	}

	return f.Sync()
}
