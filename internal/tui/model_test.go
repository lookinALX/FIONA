package tui

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lookinalx/fiona/internal/app"
	"github.com/lookinalx/fiona/internal/config"
)

const (
	testWidth  = 110
	testHeight = 32
)

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// listing maps the files under dir, FIONA's state left out, to their content.
func listing(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".fiona" {
			return filepath.SkipDir
		}
		if d.Type().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(dir, path)
			files[filepath.ToSlash(rel)] = string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func loadConfig(t *testing.T, content string) *config.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), config.FileName)
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// start makes a model for dir, sized and with the first plan ready.
func start(t *testing.T, dir string, cfg *config.Config, classifier app.Classifier) model {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{Source: resolved, Config: cfg, HistoryDir: t.TempDir(), Version: "test", Workers: 2}
	if classifier != nil {
		opts.NewClassifier = func() app.Classifier { return classifier }
	}

	m := newModel(opts)
	m.debounce = 0
	m = drain(t, m, m.Init())
	m = send(t, m, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	t.Cleanup(m.close)
	return m
}

// send delivers a message and runs every command it starts, and the commands
// those start, until there is nothing left: background work finishes in the test.
func send(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	next, cmd := m.Update(msg)
	return drain(t, next.(model), cmd)
}

func drain(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 10000 {
			t.Fatal("commands do not settle")
		}
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case nil:
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case tea.QuitMsg:
			return m
		default:
			next, more := m.Update(msg)
			m = next.(model)
			queue = append(queue, more)
		}
	}
	return m
}

func key(k string) tea.KeyMsg {
	return keyMsg(k)
}

func folderNames(m model) []string {
	var names []string
	for _, f := range m.plan.Folders() {
		names = append(names, f.Name)
	}
	return names
}

func TestStart_PlanIsShownAtOnce(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"photo.jpg": "p", "report.pdf": "r"})

	m := start(t, dir, loadConfig(t, ""), nil)

	if m.plan == nil {
		t.Fatal("no plan after start")
	}
	if got := folderNames(m); !reflect.DeepEqual(got, []string{"images", "documents"}) {
		t.Errorf("folders = %v", got)
	}
	view := m.View()
	for _, want := range []string{"Result · preset downloads", "images/", "documents/", "enter move 2 files"} {
		if !strings.Contains(view, want) {
			t.Errorf("view does not contain %q:\n%s", want, view)
		}
	}
}

// A11: enter asks first; the disk changes only after y.
func TestA11_EnterAsksBeforeTouchingDisk(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"photo.jpg": "p", "report.pdf": "r"})
	before := listing(t, dir)

	m := start(t, dir, loadConfig(t, ""), nil)
	m = send(t, m, key("enter"))

	if m.screen != screenConfirm {
		t.Fatalf("screen = %v, want the question", m.screen)
	}
	if !strings.Contains(m.View(), "Move 2 files (2 B) into 2 folders?") {
		t.Errorf("the question is not shown:\n%s", m.View())
	}
	if got := listing(t, dir); !reflect.DeepEqual(got, before) {
		t.Fatalf("disk changed before the answer: %v", got)
	}

	m = send(t, m, key("n"))
	if m.screen != screenMain || !reflect.DeepEqual(listing(t, dir), before) {
		t.Fatalf("n did not cancel: screen %v, files %v", m.screen, listing(t, dir))
	}

	m = send(t, send(t, m, key("enter")), key("y"))

	if m.screen != screenDone {
		t.Fatalf("screen = %v, want done", m.screen)
	}
	if want := map[string]string{"images/photo.jpg": "p", "documents/report.pdf": "r"}; !reflect.DeepEqual(listing(t, dir), want) {
		t.Errorf("files = %v, want %v", listing(t, dir), want)
	}
	if !strings.Contains(strings.Join(m.outcome, "\n"), "✓ Moved 2 files") {
		t.Errorf("outcome = %q", m.outcome)
	}
}

// A12: a click beside the hints does nothing; a click on a hint is its key.
func TestA12_ClickOnlyOnHints(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"photo.jpg": "p"})
	m := start(t, dir, loadConfig(t, ""), nil)
	bar := m.layout().barY

	_, from, to := hintSegments(m.hints())
	for _, x := range []int{0, to[0] + 1, testWidth - 1} {
		next, cmd := m.Update(tea.MouseMsg{X: x, Y: bar, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
		if next.(model).screen != screenMain || cmd != nil {
			t.Errorf("click at column %d did something", x)
		}
	}
	// a click in an empty part of the plan does not run anything either
	next, cmd := m.Update(tea.MouseMsg{X: testWidth - 5, Y: m.layout().boxTop + 10, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if next.(model).screen != screenMain || cmd != nil {
		t.Error("click in the plan did something")
	}

	m = send(t, m, tea.MouseMsg{X: from[0], Y: bar, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if m.screen != screenConfirm {
		t.Errorf("click on the enter hint: screen = %v, want the question", m.screen)
	}
}

// A13: u on the done screen undoes the run.
func TestA13_UndoFromDoneScreen(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"photo.jpg": "p", "report.pdf": "r", "inbox/keep.txt": "k"})
	before := listing(t, dir)

	m := start(t, dir, loadConfig(t, ""), nil)
	m = send(t, send(t, m, key("enter")), key("y"))
	if m.screen != screenDone {
		t.Fatalf("screen = %v", m.screen)
	}

	m = send(t, m, key("u"))
	if m.screen != screenUndoConfirm || !strings.Contains(m.undoQuestion, "Undo the run from today") {
		t.Fatalf("screen = %v, question = %q", m.screen, m.undoQuestion)
	}
	m = send(t, m, key("y"))

	if got := listing(t, dir); !reflect.DeepEqual(got, before) {
		t.Errorf("after undo: %v, want %v", got, before)
	}
	if !strings.Contains(strings.Join(m.outcome, "\n"), "✓ Restored 2 files") {
		t.Errorf("outcome = %q", m.outcome)
	}
	if m.journal != "" {
		t.Error("an undo can be undone again from the done screen")
	}
}

// A24: the plan follows the preset and the folder without enter.
func TestA24_PlanFollowsPresetAndFolder(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"photo.jpg": "p", "report.pdf": "r", "sub/clip.mp4": "c"})
	before := listing(t, dir)

	m := start(t, dir, loadConfig(t, ""), nil)
	if got := folderNames(m); !reflect.DeepEqual(got, []string{"images", "documents"}) {
		t.Fatalf("folders = %v", got)
	}

	m = send(t, m, key("p")) // photos: only images and videos, by year and month
	if m.presets[m.preset] != "photos" || m.plan.Stats().Files != 1 || m.plan.Stats().Unmatched != 1 {
		t.Errorf("preset %s: stats = %+v", m.presets[m.preset], m.plan.Stats())
	}
	m = send(t, m, key("p"))
	m = send(t, m, key("P"))
	if m.presets[m.preset] != "photos" {
		t.Errorf("preset = %s after p, P", m.presets[m.preset])
	}

	// keys that arrive together count one by one
	m = send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ppp")})
	if m.presets[m.preset] != "custom" {
		t.Errorf("preset = %s after ppp", m.presets[m.preset])
	}

	// the custom template is typed in place
	m = send(t, m, key("t"))
	for range len(defaultTemplate) {
		m = send(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	m = send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("{ext}")})
	m = send(t, m, key("enter"))
	if got := folderNames(m); !reflect.DeepEqual(got, []string{"jpg", "pdf"}) {
		t.Errorf("template {ext}: folders = %v", got)
	}

	// moving into the subfolder plans it instead
	m = send(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.source() != filepath.Join(m.opts.Source, "sub") {
		t.Fatalf("selected = %s", m.source())
	}
	if got := folderNames(m); !reflect.DeepEqual(got, []string{"mp4"}) {
		t.Errorf("subfolder: folders = %v", got)
	}

	if m.screen != screenMain {
		t.Errorf("screen = %v", m.screen)
	}
	if got := listing(t, dir); !reflect.DeepEqual(got, before) {
		t.Errorf("disk changed: %v", got)
	}
}

type countingClassifier struct {
	mu    sync.Mutex
	calls int
	paths []string
}

func (c *countingClassifier) Classify(_ context.Context, paths []string) (map[string]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	out := map[string]string{}
	for _, p := range paths {
		c.paths = append(c.paths, filepath.Base(p))
		out[p] = "cats"
	}
	return out, nil
}

func (c *countingClassifier) Close() error { return nil }

// A25: the classifier runs only after m.
func TestA25_ClassifierWaitsForM(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"cat.jpg": "c", "report.pdf": "r"})
	cfg := loadConfig(t, `{"default_preset": "ai", "presets": {"ai": {"rules": [
		{"match": {"category": ["images"]}, "to": "{ml}"},
		{"to": "{category}"}
	]}}}`)
	classifier := &countingClassifier{}

	m := start(t, dir, cfg, classifier)

	if classifier.calls != 0 {
		t.Fatalf("the classifier ran %d times before m", classifier.calls)
	}
	if m.mlPending != 1 || !strings.Contains(m.View(), "press m") {
		t.Errorf("mlPending = %d, view:\n%s", m.mlPending, m.View())
	}

	m = send(t, m, key("enter"))
	if m.screen != screenMain || !strings.Contains(m.notice, "press m") {
		t.Errorf("enter before classification: screen %v, notice %q", m.screen, m.notice)
	}

	m = send(t, m, key("m"))
	if classifier.calls != 1 || !reflect.DeepEqual(classifier.paths, []string{"cat.jpg"}) {
		t.Fatalf("classifier: %d calls with %v", classifier.calls, classifier.paths)
	}
	names := folderNames(m)
	sort.Strings(names)
	if m.mlPending != 0 || !reflect.DeepEqual(names, []string{"cats", "documents"}) {
		t.Errorf("after m: pending %d, folders %v", m.mlPending, names)
	}
}

func TestDestination(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"photo.jpg": "p", "sorted/.keep": ""})
	m := start(t, dir, loadConfig(t, ""), nil)

	m = send(t, m, key("d"))
	if m.focus != focusDest {
		t.Fatalf("focus = %v", m.focus)
	}
	m = send(t, m, tea.KeyMsg{Type: tea.KeyDown}) // into sorted/
	m = send(t, m, key("enter"))

	if m.dest != filepath.Join(m.opts.Source, "sorted") || m.plan.BaseDirDest != m.dest {
		t.Errorf("dest = %q, plan dest = %q", m.dest, m.plan.BaseDirDest)
	}
	if !strings.Contains(m.View(), "into ") {
		t.Error("the header does not show the destination")
	}

	m = send(t, m, key("d"))
	m = send(t, m, key("esc"))
	if m.dest != "" {
		t.Errorf("esc did not return to the same folder: %q", m.dest)
	}
}

func TestHistoryScreen(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"photo.jpg": "p"})
	m := start(t, dir, loadConfig(t, ""), nil)
	m = send(t, send(t, m, key("enter")), key("y"))
	m = send(t, m, key("h"))

	if m.screen != screenHistory || len(m.records) != 1 {
		t.Fatalf("screen = %v, records = %d", m.screen, len(m.records))
	}
	m = send(t, m, key("enter"))
	if m.screen != screenUndoConfirm {
		t.Fatalf("screen = %v", m.screen)
	}
	m = send(t, m, key("y"))
	if got := listing(t, dir); !reflect.DeepEqual(got, map[string]string{"photo.jpg": "p"}) {
		t.Errorf("after undo from history: %v", got)
	}
}

func TestView_SmallTerminals(t *testing.T) {
	dir := t.TempDir()
	m := start(t, dir, loadConfig(t, ""), nil)

	for _, size := range [][2]int{{1, 1}, {0, 0}, {40, 10}, {59, 30}, {60, 14}, {200, 60}} {
		m = send(t, m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		_ = m.View() // must not panic
	}
	if !strings.Contains(m.View(), "▀") && !strings.Contains(m.View(), "FIONA") {
		t.Error("no title on a big terminal")
	}
}

func TestTree_OpensAtStart(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"a/b/c/file": "", "a/x/file": ""})
	start, err := filepath.EvalSymlinks(filepath.Join(dir, "a", "b"))
	if err != nil {
		t.Fatal(err)
	}

	ft := NewFileTree(start)
	if ft.Selected() != start {
		t.Fatalf("selected = %s, want %s", ft.Selected(), start)
	}

	ft, changed := ft.Update(tea.KeyMsg{Type: tea.KeyDown})
	if !changed || ft.Selected() != filepath.Join(start, "c") {
		t.Errorf("down: selected = %s, changed = %v", ft.Selected(), changed)
	}
	ft, changed = ft.Update(tea.KeyMsg{Type: tea.KeyRight})
	if changed {
		t.Error("expanding changed the selection")
	}
}
