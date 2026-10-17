package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lookinalx/fiona/internal/app"
	"github.com/lookinalx/fiona/internal/history"
	"github.com/lookinalx/fiona/internal/report"
	"github.com/lookinalx/fiona/internal/rules"
	"github.com/lookinalx/fiona/internal/scanner"
	"github.com/lookinalx/fiona/internal/sorter"
	"github.com/lookinalx/fiona/internal/types"
)

type screen int

const (
	screenMain screen = iota
	screenConfirm
	screenRunning
	screenDone
	screenHistory
	screenUndoConfirm
)

type focus int

const (
	focusTree focus = iota
	focusPlan
	focusTemplate // typing the template of the custom preset
	focusDest     // choosing another destination
)

const (
	customPreset    = "custom"
	defaultTemplate = "{category}"
	scanDebounce    = 200 * time.Millisecond
)

type model struct {
	opts          Options
	width, height int
	screen        screen
	focus         focus
	help          bool
	debounce      time.Duration

	tree     FileTree
	destTree FileTree
	dest     string // empty: sort into the selected folder itself

	presets  []string // the names of the config, then "custom"
	preset   int
	template lineInput
	copy     bool
	recurse  bool
	hidden   bool

	rules      *rules.Set
	ruleSource string
	rulesErr   error

	scanSeq    int
	scanCancel context.CancelFunc
	scanning   bool
	scanned    *scanner.Result
	scanErr    error

	planning       bool
	replan         bool
	plan           *sorter.Plan
	mlPending      int
	classifying    bool
	classifyQueued bool
	classifyErr    error
	classifier     app.Classifier

	showList   bool
	planOffset int
	notice     string // a short message for the hint bar

	progress  *progress
	runCancel context.CancelFunc
	runStart  time.Time
	outcome   []string
	runDest   string // destination of the last run, for "open folder"
	journal   string // of the last sort, for undo from the done screen

	records      []history.Record
	histCursor   int
	histErr      error
	undoPlan     *sorter.UndoPlan
	undoLines    []string
	undoQuestion string
}

func newModel(opts Options) model {
	opts = opts.withDefaults()

	m := model{
		opts:     opts,
		debounce: scanDebounce,
		tree:     NewFileTree(opts.Source),
		presets:  append(opts.Config.Names(), customPreset),
		template: lineInput{value: defaultTemplate},
		copy:     opts.Config.Action == string(types.OpCopy),
	}
	for i, name := range m.presets {
		if name == opts.Config.DefaultPresetName() {
			m.preset = i
		}
	}
	m.applyPreset()
	m.scanSeq++
	m.scanning = true
	return m
}

func (m model) Init() tea.Cmd {
	return m.startScan(m.scanSeq)
}

// close stops what runs in the background.
func (m model) close() {
	if m.scanCancel != nil {
		m.scanCancel()
	}
	if m.runCancel != nil {
		m.runCancel()
	}
	if m.classifier != nil {
		m.classifier.Close()
	}
}

// ── requests ──────────────────────────────────────────────────────────────────

func (m model) source() string {
	return m.tree.Selected()
}

func (m model) destination() string {
	if m.dest != "" {
		return m.dest
	}
	return m.source()
}

func (m model) request() app.Request {
	action := string(types.OpMove)
	if m.copy {
		action = string(types.OpCopy)
	}
	onConflict := m.opts.Config.OnConflict
	if onConflict == "" {
		onConflict = types.OnConflictRename
	}
	return app.Request{
		Source:     m.source(),
		Dest:       m.destination(),
		Rules:      m.rules,
		Action:     action,
		OnConflict: onConflict,
		Recursive:  m.recurse,
		Hidden:     m.hidden,
	}
}

func (m *model) applyPreset() {
	name := m.presets[m.preset]
	if name == customPreset {
		m.rules, m.rulesErr = compileTemplate(m.template.value)
		m.ruleSource = fmt.Sprintf("template %q", m.template.value)
		return
	}
	m.rules, m.rulesErr = m.opts.Config.Rules(name)
	m.ruleSource = "preset " + name
}

// requestScan forgets the current plan and scans the selected folder after a
// short pause, so that moving through the tree does not start a scan per row.
func (m *model) requestScan() tea.Cmd {
	m.scanSeq++
	if m.scanCancel != nil {
		m.scanCancel()
		m.scanCancel = nil
	}
	m.scanning = true
	m.scanned, m.scanErr, m.plan = nil, nil, nil
	m.mlPending, m.classifyErr, m.planOffset = 0, nil, 0

	seq := m.scanSeq
	if m.debounce == 0 {
		return m.startScan(seq)
	}
	return tea.Tick(m.debounce, func(time.Time) tea.Msg { return debounceMsg{seq: seq} })
}

func (m *model) startScan(seq int) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.scanCancel = cancel
	return scanCmd(ctx, seq, m.request())
}

// requestPlan rebuilds the plan from the scan. Planning and classification
// never run at the same time: both use the tags of the files.
func (m *model) requestPlan() tea.Cmd {
	if m.scanned == nil || m.rules == nil {
		m.plan = nil
		return nil
	}
	if m.planning || m.classifying {
		m.replan = true
		return nil
	}
	m.planning, m.replan = true, false
	return planCmd(m.scanSeq, m.scanned, m.request())
}

func (m *model) startClassify() tea.Cmd {
	if m.mlPending == 0 || m.classifying || m.scanned == nil {
		return nil
	}
	if m.planning {
		m.classifyQueued = true
		return nil
	}
	if m.classifier == nil {
		m.classifier = m.opts.NewClassifier()
	}
	m.classifying, m.classifyQueued, m.classifyErr = true, false, nil
	return classifyCmd(m.scanSeq, app.NeedsML(m.scanned, m.rules), m.classifier)
}

func (m *model) startRun() tea.Cmd {
	plan := m.plan
	m.screen = screenRunning
	m.progress = &progress{root: plan.BaseDirSource}
	m.runStart = time.Now()
	m.runDest = plan.BaseDirDest

	ctx, cancel := context.WithCancel(context.Background())
	m.runCancel = cancel
	opts := app.ExecOptions{Workers: m.opts.Workers, Version: m.opts.Version, HistoryDir: m.opts.HistoryDir}
	return tea.Batch(tick(), runCmd(ctx, plan, opts, m.progress))
}

func (m *model) startUndo() tea.Cmd {
	plan := m.undoPlan
	m.screen = screenRunning
	m.progress = &progress{undo: true, root: plan.Run.Header.Dest}
	m.runStart = time.Now()
	m.runDest = plan.Run.Header.Source

	ctx, cancel := context.WithCancel(context.Background())
	m.runCancel = cancel
	return tea.Batch(tick(), undoCmd(ctx, plan, m.opts.Workers, m.progress))
}

// ── update ────────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case debounceMsg:
		if msg.seq != m.scanSeq {
			return m, nil
		}
		return m, m.startScan(msg.seq)

	case scanDoneMsg:
		if msg.seq != m.scanSeq {
			return m, nil
		}
		m.scanning, m.scanCancel = false, nil
		m.scanned, m.scanErr = msg.result, msg.err
		return m, m.requestPlan()

	case planDoneMsg:
		m.planning = false
		if msg.scanSeq == m.scanSeq {
			m.plan, m.mlPending = msg.plan, msg.ml
		}
		switch {
		case m.classifyQueued:
			return m, m.startClassify()
		case m.replan:
			return m, m.requestPlan()
		}
		return m, nil

	case classifyDoneMsg:
		m.classifying = false
		if msg.scanSeq == m.scanSeq {
			m.classifyErr = msg.err
		}
		m.replan = false
		return m, m.requestPlan()

	case tickMsg:
		if m.screen == screenRunning {
			return m, tick()
		}
		return m, nil

	case runDoneMsg:
		return m.finishRun(msg), nil

	case historyMsg:
		m.records, m.histErr, m.histCursor = msg.records, msg.err, 0
		return m, nil

	case undoReadyMsg:
		if msg.err != nil {
			m.notice = msg.err.Error()
			return m, nil
		}
		m.undoPlan = msg.plan
		m.undoLines, m.undoQuestion = report.UndoPreview(msg.plan, time.Now(), report.Styles{})
		m.screen = screenUndoConfirm
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)
	}

	return m, nil
}

func (m model) finishRun(msg runDoneMsg) model {
	m.screen = screenDone
	m.runCancel = nil
	m.focus = focusTree

	if msg.undo {
		m.journal = ""
		m.outcome = report.UndoOutcome(report.UndoResult{Plan: msg.undoPlan, Summary: msg.summary, Problems: msg.problems}, report.Styles{})
	} else if msg.outcome.JournalPath == "" {
		m.journal = ""
		m.outcome = []string{"✗ Nothing was changed: " + errText(msg.err)}
		return m
	} else {
		m.journal = msg.outcome.JournalPath
		m.outcome = report.SortOutcome(report.SortResult{
			Plan: msg.plan, Summary: msg.summary, Elapsed: msg.outcome.Elapsed, Problems: msg.problems,
		}, report.Styles{})
		if msg.outcome.HistoryErr != nil {
			m.outcome = append(m.outcome, "", "The run is not added to the history: "+msg.outcome.HistoryErr.Error())
		}
	}
	if msg.err != nil {
		m.outcome = append(m.outcome, "", "✗ "+msg.err.Error())
	}
	return m
}

func errText(err error) string {
	if err == nil {
		return "unknown error"
	}
	return err.Error()
}

func (m *model) resize() {
	l := m.layout()
	m.tree = m.tree.SetSize(l.innerW(l.treeW), l.innerH()-1)
	m.destTree = m.destTree.SetSize(l.innerW(l.planW), l.innerH()-1)
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// keys pressed quickly may arrive together; outside text fields each counts
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 1 && !m.typing() {
		var cmds []tea.Cmd
		var next tea.Model = m
		for _, r := range msg.Runes {
			var cmd tea.Cmd
			next, cmd = next.(model).handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: msg.Alt})
			cmds = append(cmds, cmd)
		}
		return next, tea.Batch(cmds...)
	}

	key := msg.String()
	m.notice = ""

	if key == "ctrl+c" {
		if m.screen == screenRunning {
			if m.runCancel != nil {
				m.runCancel()
			}
			m.notice = "stopping after the current files…"
			return m, nil
		}
		return m, tea.Quit
	}

	if m.help {
		m.help = false
		return m, nil
	}

	switch m.screen {
	case screenConfirm:
		switch key {
		case "y", "enter":
			return m, m.startRun()
		case "n", "esc", "q":
			m.screen = screenMain
		}
		return m, nil

	case screenUndoConfirm:
		switch key {
		case "y", "enter":
			return m, m.startUndo()
		case "n", "esc", "q":
			m.screen = screenMain
		}
		return m, nil

	case screenRunning:
		return m, nil

	case screenDone:
		return m.handleDoneKey(key)

	case screenHistory:
		return m.handleHistoryKey(key)
	}

	return m.handleMainKey(msg)
}

func (m model) handleDoneKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "u":
		if m.journal == "" {
			m.notice = "nothing to undo here"
			return m, nil
		}
		return m, undoPlanCmd(nil, m.journal)
	case "o":
		openFolder(m.runDest)
	case "n", "enter", "esc":
		m.screen = screenMain
		return m, m.requestScan()
	case "h":
		m.screen = screenHistory
		return m, historyCmd(m.opts.HistoryDir)
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) handleHistoryKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		m.histCursor = max(0, m.histCursor-1)
	case "down", "j":
		m.histCursor = min(len(m.records)-1, m.histCursor+1)
	case "enter", "u":
		if m.histCursor >= len(m.records) {
			return m, nil
		}
		r := m.records[m.histCursor]
		if !r.Undoable() {
			m.notice = fmt.Sprintf("this run cannot be undone: %s", r.State)
			return m, nil
		}
		return m, undoPlanCmd(r.Run, "")
	case "esc", "h", "q":
		m.screen = screenMain
		return m, m.requestScan()
	}
	return m, nil
}

func (m model) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch m.focus {
	case focusTemplate:
		switch key {
		case "enter", "tab", "esc":
			m.focus, m.template.focused = focusTree, false
			return m, nil
		}
		var changed bool
		if m.template, changed = m.template.Update(msg); changed {
			m.applyPreset()
			return m, m.requestPlan()
		}
		return m, nil

	case focusDest:
		switch key {
		case "esc":
			m.focus, m.dest = focusTree, ""
			return m, m.requestScan()
		case "enter":
			m.focus, m.dest = focusTree, m.destTree.Selected()
			if m.dest == m.source() {
				m.dest = ""
			}
			return m, m.requestScan()
		}
		m.destTree, _ = m.destTree.Update(msg)
		return m, nil
	}

	if m.tree.editing {
		if key == "tab" {
			m.tree = m.tree.StopEditing()
			return m, nil
		}
		var changed bool
		m.tree, changed = m.tree.Update(msg)
		if changed {
			return m, m.requestScan()
		}
		return m, nil
	}

	switch key {
	case "q", "esc":
		return m, tea.Quit
	case "?":
		m.help = true
		return m, nil
	case "tab":
		if m.focus == focusTree {
			m.focus = focusPlan
		} else {
			m.focus = focusTree
		}
		return m, nil
	case "enter":
		return m.confirm()
	case "p", "P", "shift+tab":
		if key == "p" {
			m.preset = (m.preset + 1) % len(m.presets)
		} else {
			m.preset = (m.preset - 1 + len(m.presets)) % len(m.presets)
		}
		m.applyPreset()
		m.planOffset = 0
		return m, m.requestPlan()
	case "t":
		if m.presets[m.preset] == customPreset {
			m.focus, m.template.focused = focusTemplate, true
			return m, nil
		}
	case "c":
		m.copy = !m.copy
		return m, m.requestPlan()
	case "r":
		m.recurse = !m.recurse
		return m, m.requestScan()
	case ".":
		m.hidden = !m.hidden
		return m, m.requestScan()
	case "d":
		m.focus = focusDest
		start := m.dest
		if start == "" {
			start = m.source()
		}
		l := m.layout()
		m.destTree = NewFileTree(start).SetSize(l.innerW(l.planW), l.innerH()-1)
		return m, nil
	case "m":
		return m, m.startClassify()
	case "h":
		m.screen = screenHistory
		m.records = nil
		return m, historyCmd(m.opts.HistoryDir)
	case "l":
		if m.focus == focusPlan {
			m.showList = !m.showList
			m.planOffset = 0
			return m, nil
		}
	}

	if m.focus == focusPlan {
		switch key {
		case "up", "k":
			m.planOffset = max(0, m.planOffset-1)
		case "down", "j":
			m.planOffset++
		case "pgup":
			m.planOffset = max(0, m.planOffset-m.layout().innerH())
		case "pgdown":
			m.planOffset += m.layout().innerH()
		}
		return m, nil
	}

	var changed bool
	m.tree, changed = m.tree.Update(msg)
	if changed {
		return m, m.requestScan()
	}
	return m, nil
}

// confirm opens the question when the plan is ready and has something to do.
func (m model) confirm() (tea.Model, tea.Cmd) {
	switch {
	case m.rulesErr != nil:
		m.notice = "fix the template first"
	case m.scanning || m.planning || m.plan == nil:
		m.notice = "the plan is not ready yet"
	case m.classifying:
		m.notice = "wait for the classifier"
	case m.mlPending > 0:
		m.notice = fmt.Sprintf("press m to classify %s first", report.Count(m.mlPending, "file"))
	case m.plan.Stats().Files == 0:
		m.notice = "nothing to do"
	default:
		m.screen = screenConfirm
	}
	return m, nil
}

// ── mouse ─────────────────────────────────────────────────────────────────────

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	l := m.layout()

	if msg.Action != tea.MouseActionPress {
		return m, nil
	}

	inTree := msg.X < l.treeW && msg.Y >= l.boxTop && msg.Y < l.boxTop+l.boxH
	inPlan := msg.X >= l.treeW && msg.Y >= l.boxTop && msg.Y < l.boxTop+l.boxH

	switch msg.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		delta := 1
		if msg.Button == tea.MouseButtonWheelUp {
			delta = -1
		}
		if inPlan && m.screen == screenMain {
			m.planOffset = max(0, m.planOffset+delta)
			return m, nil
		}
		if inTree && m.screen == screenMain && m.focus != focusDest {
			before := m.tree.cursor
			m.tree.move(delta)
			if m.tree.cursor != before {
				return m, m.requestScan()
			}
		}
		return m, nil

	case tea.MouseButtonLeft:
	default:
		return m, nil
	}

	if msg.Y == l.barY {
		// only a click right on a hint does something
		if key, ok := hintAt(m.hints(), msg.X); ok {
			return m.handleKey(keyMsg(key))
		}
		return m, nil
	}

	if m.screen != screenMain || m.help {
		return m, nil
	}

	if msg.Y == l.presetsY {
		if i, ok := presetAt(m.presets, m.preset, msg.X); ok && i != m.preset {
			m.preset = i
			m.applyPreset()
			return m, m.requestPlan()
		}
		return m, nil
	}

	if inTree && m.focus != focusDest {
		m.focus = focusTree
		row := msg.Y - l.boxTop - 2 // border and title
		if m.tree.editing {
			row--
		}
		var changed bool
		m.tree, changed = m.tree.Click(row)
		if changed {
			return m, m.requestScan()
		}
		return m, nil
	}
	if inPlan {
		if m.focus == focusDest {
			var changed bool
			m.destTree, changed = m.destTree.Click(msg.Y - l.boxTop - 2)
			_ = changed
			return m, nil
		}
		m.focus = focusPlan
	}
	return m, nil
}

// keyMsg makes the key message a hint stands for.
func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

// openFolder shows a folder in the file manager of the system.
func openFolder(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if cmd.Start() == nil {
		go cmd.Wait()
	}
}

// typing reports whether keys go into a text field.
func (m model) typing() bool {
	return m.screen == screenMain && !m.help && (m.focus == focusTemplate || m.tree.editing)
}
