package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/rules"
	"github.com/lookinalx/fiona/internal/sorter"
	"github.com/lookinalx/fiona/internal/types"
)

type fakeClassifier struct {
	tags  map[string]string // by file name
	asked []string
}

func (f *fakeClassifier) Classify(_ context.Context, paths []string) (map[string]string, error) {
	out := make(map[string]string)
	for _, p := range paths {
		f.asked = append(f.asked, filepath.Base(p))
		if tag, ok := f.tags[filepath.Base(p)]; ok {
			out[p] = tag
		} else {
			out[p] = notImage
		}
	}
	return out, nil
}

func (f *fakeClassifier) Close() error { return nil }

type silent struct{}

func (silent) Start(int)                {}
func (silent) Done(types.Action, error) {}
func (silent) Finish(sorter.Summary)    {}

func write(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func request(t *testing.T, dir string, rs ...rules.Rule) Request {
	t.Helper()
	set, err := rules.Compile(rs)
	if err != nil {
		t.Fatal(err)
	}
	return Request{Source: dir, Dest: dir, Rules: set, Action: "move", OnConflict: types.OnConflictRename}
}

// A16: only the files whose rule uses {ml} go to the classifier.
func TestNeedsML_OnlyFilesOfRulesWithML(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "cat.jpg", "dog.png", "report.pdf", "song.mp3")
	req := request(t, dir,
		rules.Rule{Match: rules.Match{Category: []string{"images"}}, To: "{ml}"},
		rules.Rule{To: "{category}"},
	)

	scanned, err := Scan(req)
	if err != nil {
		t.Fatal(err)
	}
	classifier := &fakeClassifier{tags: map[string]string{"cat.jpg": "cats"}}
	if err := Classify(context.Background(), NeedsML(scanned, req.Rules), classifier); err != nil {
		t.Fatal(err)
	}

	sort.Strings(classifier.asked)
	if len(classifier.asked) != 2 || classifier.asked[0] != "cat.jpg" || classifier.asked[1] != "dog.png" {
		t.Errorf("classifier got %v, want only the images", classifier.asked)
	}

	plan := Plan(scanned, req, "20261010-153012-a1b2")
	dirs := map[string]string{}
	for _, a := range plan.Actions {
		dirs[filepath.Base(a.SourcePath)] = a.DestDir
	}
	want := map[string]string{"cat.jpg": "cats", "dog.png": "images", "report.pdf": "documents", "song.mp3": "audio"}
	for name, dir := range want {
		if dirs[name] != dir {
			t.Errorf("%s goes to %q, want %q", name, dirs[name], dir)
		}
	}

	// tagged files are not classified again when the plan is rebuilt
	if again := NeedsML(scanned, req.Rules); len(again) != 0 {
		t.Errorf("NeedsML() after classification = %d files", len(again))
	}
}

func TestNeedsML_NoMLInRules(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "cat.jpg")
	req := request(t, dir, rules.Rule{To: "{category}"})
	scanned, err := Scan(req)
	if err != nil {
		t.Fatal(err)
	}
	if files := NeedsML(scanned, req.Rules); files != nil {
		t.Errorf("NeedsML() = %v", files)
	}
}

func TestExecuteAndUndoThroughHistory(t *testing.T) {
	dir, historyDir := t.TempDir(), t.TempDir()
	write(t, dir, "a.jpg", "b.pdf")
	req := request(t, dir, rules.Rule{To: "{category}"})

	scanned, err := Scan(req)
	if err != nil {
		t.Fatal(err)
	}
	plan := Plan(scanned, req, journal.NewRunID(time.Now()))
	out, err := Execute(context.Background(), plan, ExecOptions{Workers: 2, Version: "test", HistoryDir: historyDir}, silent{})
	if err != nil || !out.InHistory || out.Summary.Moved != 2 {
		t.Fatalf("Execute() = %+v, %v", out, err)
	}

	run, err := FindRun(UndoTarget{HistoryDir: historyDir})
	if err != nil || run.Header.RunID != plan.RunID {
		t.Fatalf("FindRun() = %v, %v", run, err)
	}
	undo, err := sorter.NewUndoPlan(run)
	if err != nil {
		t.Fatal(err)
	}
	if summary, err := ExecuteUndo(context.Background(), undo, 2, silent{}); err != nil || summary.Restored != 2 {
		t.Fatalf("ExecuteUndo() = %+v, %v", summary, err)
	}

	_, err = FindRun(UndoTarget{HistoryDir: historyDir})
	var nothing *NothingToUndoError
	if !errors.As(err, &nothing) || nothing.Last == nil || nothing.Last.Header.RunID != plan.RunID {
		t.Errorf("FindRun() after undo error = %v", err)
	}
}

func TestFindRun_FallsBackToCurrentFolder(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.jpg")
	req := request(t, dir, rules.Rule{To: "{category}"})
	scanned, err := Scan(req)
	if err != nil {
		t.Fatal(err)
	}
	plan := Plan(scanned, req, journal.NewRunID(time.Now()))
	// a run that did not reach the history
	if _, err := Execute(context.Background(), plan, ExecOptions{Workers: 1}, silent{}); err != nil {
		t.Fatal(err)
	}

	run, err := FindRun(UndoTarget{HistoryDir: t.TempDir(), Cwd: dir})
	if err != nil || run.Header.RunID != plan.RunID {
		t.Errorf("FindRun() = %v, %v", run, err)
	}

	_, err = FindRun(UndoTarget{HistoryDir: t.TempDir(), Cwd: t.TempDir()})
	var nothing *NothingToUndoError
	if !errors.As(err, &nothing) || nothing.Error() != "Nothing to undo: no runs yet." {
		t.Errorf("FindRun() in an empty folder error = %v", err)
	}
}
