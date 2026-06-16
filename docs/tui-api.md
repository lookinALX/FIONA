# TUI API

Package `internal/tui` exposes a single entry point. The caller launches the TUI, waits for the user to choose an action, and receives a `Result`.

## Entry point

```go
func Run() (Result, error)
```

Blocks until the user confirms an action or cancels. Returns an error only if the TUI framework itself fails to start.

---

## Result

```go
type Result struct {
    Action     Action // what the user requested
    Primary    string // populated when Action == ActionSort
    Secondary  string // populated when Action == ActionSort
    SourcePath string // populated when Action == ActionSort
    DestPath   string // populated when Action == ActionSort
    FileAction string // "move" | "copy" — populated when Action == ActionSort
}
```

### Action values

| Constant       | Trigger                        | Fields populated                        |
|----------------|--------------------------------|-----------------------------------------|
| `ActionSort`   | User pressed **Sort**          | Primary, Secondary, SourcePath, DestPath, FileAction |
| `ActionUndo`   | User pressed **Undo**          | None                                    |
| `ActionCancel` | User pressed **Quit** / ctrl+c | None                                    |

### Helper method

```go
func (r Result) Cancelled() bool
```

Returns `true` when `Action == ActionCancel`. Shorthand for the common "did the user abort?" check.

---

## Field values

### Primary / Secondary

Possible strings for `Primary`:

```
"mimetype"  "extension"  "date"  "year"  "month"  "size"  "ml"
```

Possible strings for `Secondary` (same set plus):

```
"none"
```

### SourcePath / DestPath

Absolute filesystem paths as selected in the file tree. Always non-empty when `Action == ActionSort`.

### FileAction

`"move"` — files are moved to the destination (default).  
`"copy"` — files are copied, originals remain in place.

---

## Usage pattern in main.go

```go
result, err := tui.Run()
if err != nil {
    // TUI failed to start
}

switch result.Action {
case tui.ActionCancel:
    os.Exit(0)

case tui.ActionUndo:
    // run undo pipeline (no criteria needed)

case tui.ActionSort:
    opts.Sort.Primary   = result.Primary
    opts.Sort.Secondary = result.Secondary
    opts.SourcePath     = result.SourcePath
    opts.DestPath       = result.DestPath
    opts.FileAction     = result.FileAction // "move" | "copy"
    // run sort pipeline
}
```

---

## Bottom bar buttons

| Button    | Keyboard     | Mouse  | Effect                          |
|-----------|--------------|--------|---------------------------------|
| Sort      | enter        | click  | Confirms sort → `ActionSort`    |
| Undo      | enter        | click  | Confirms undo → `ActionUndo`    |
| Move/Copy | enter, space | click  | Toggles `FileAction` in-place   |
| Quit      | enter        | click  | Cancels → `ActionCancel`        |
| —         | ctrl+c       | —      | Cancels → `ActionCancel`        |

Tab / Shift+Tab cycle focus between all buttons and selectors.  
The Move/Copy button shows the **currently active** mode; pressing it switches to the other.
