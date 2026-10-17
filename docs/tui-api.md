# TUI API

Package `internal/tui` is the interactive mode. It runs the whole pipeline itself through `internal/app`: the caller only starts it and waits until the user quits.

## Entry point

```go
func Run(opts Options) error
```

Blocks until the user quits. Returns an error only if the terminal UI cannot run. Background work (scans, the classifier, a running sort) is stopped before `Run` returns.

```go
type Options struct {
    Source     string         // resolved folder the tree opens at
    Config     *config.Config // presets, default preset, action and on-conflict
    HistoryDir string         // where history.jsonl lives
    Version    string         // written to run journals
    Workers    int            // default: twice the number of CPUs

    // NewClassifier makes the ML classifier on first use; default fiona-classifier.
    NewClassifier func() app.Classifier
}
```

`cmd/fiona` starts it for `fiona tui [FOLDER]` and for `fiona` without arguments when stdin and stdout are terminals.

## Screens

```
main ─enter→ confirm ─y→ running → done ─n→ main
 │ h            └─esc→ main          │ u
 ↓                                   ↓
history ─enter→ undoConfirm ─y→ running → done
```

| Screen | Shows | Keys |
|--------|-------|------|
| `main` | folder tree; plan of the selected folder with the chosen preset; preset line; hints | `↑↓→←` `e` tree, `tab` tree ⇄ plan, `l` files in the plan, `p`/`P` preset, `t` template of `custom`, `c` copy, `d` destination, `r` subfolders, `.` hidden, `m` classify, `enter` sort, `h` history, `?` help, `q` quit |
| `confirm` | the question of the CLI in the hint bar | `y`/`enter`, `n`/`esc` |
| `running` | progress bar, current file, time | `ctrl+c` stops after the current files |
| `done` | the result line of the CLI and the problems | `u` undo, `o` open folder, `n` new sort, `h` history, `q` quit |
| `history` | runs with their state | `enter` undo the selected run, `esc` back |
| `undoConfirm` | what will be returned, the question in the hint bar | `y`, `n` |

## How the plan follows the user

- Moving the cursor in the tree schedules a scan 200 ms after the last key; a newer scan cancels the older one through its context. Every result carries the scan sequence number; a result for a folder that is no longer selected is dropped.
- The scan also decides the category of every file, so that reading file contents does not happen while planning.
- Changing the preset, the template, copy/move re-plans from the same scan; the destination, subfolders and hidden files scan again.
- Planning and ML classification never run at the same time: both use the tags of the files. A request that comes while one is running is queued.
- The classifier is never started by itself. While files wait for it, the plan says so and `enter` asks to press `m` first.
- The disk is not touched before `y` on the `confirm` screen.

## Progress

A running sort or undo is a command that calls `app.Execute` or `app.ExecuteUndo` with a `sorter.Reporter` that keeps counters under a mutex. The screen reads them on a 100 ms tick; the command returns a message with the result when the run ends.

## Mouse

A click selects a folder in the tree (a second click opens it), a preset in the preset line, or a hint in the hint bar. Only a click right on a hint acts; the gaps between hints and the empty space after them do nothing. The wheel moves through the tree or scrolls the plan.

## Tests

`internal/tui/model_test.go` drives the model with messages and runs every command it returns until nothing is left, so background work finishes inside the test. The scan delay is set to zero there.
