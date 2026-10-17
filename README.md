# FIONA

<p align="center">
  <img src="assets/FIONA_logo.png" alt="FIONA Logo" width="600"/>
</p>

<p align="center">
  <strong>F</strong>ile <strong>I</strong>ntelligent <strong>O</strong>rganization & <strong>N</strong>avigation <strong>A</strong>ssistant
</p>

<p align="center">
  A cross-platform file organizer written in Go that automatically sorts files based on configurable rules with ML-powered image classification.
</p>

<p align="center">
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go" alt="Go Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/lookinalx/fiona"><img src="https://goreportcard.com/badge/github.com/lookinalx/fiona" alt="Go Report"></a>
</p>

---

## 📋 Table of Contents

- [Features](#-features)
- [Installation](#-installation)
- [Quick Start](#-quick-start)
- [Commands](#-commands)
- [Presets and Rules](#-presets-and-rules)
- [Categories and Dates](#-categories-and-dates)
- [Interactive Mode](#-interactive-mode)
- [History and Undo](#-history-and-undo)
- [Data Safety](#-data-safety)
- [ML Classification](#-ml-classification)
- [Examples](#-examples)
- [Development](#-development)
- [Roadmap](#-roadmap)
- [License](#-license)

![](assets/screenshot_tui.png)

---

## ✨ Features

- **Zero flags** — `fiona ~/Downloads` shows what will happen, asks once, sorts; `fiona undo` takes it back
- **Presets** — `downloads`, `photos`, `by-date`, `by-type` built in; your own in one JSON config
- **Rules per kind of file** — invoices by year, photos by month, the rest by type, in one run
- **Path templates** — `--to "{year}/{month}"`, `{category}/{ext}`, `photos/{year}`, …
- **Meaningful categories** — about 140 extensions in 8 categories; files without a known extension are recognised by content
- **Date taken** — photos go by the EXIF date (JPEG, HEIC, DNG and other RAW), not by when they were copied
- **ML Image Classification** — sort photos into semantic categories using CLIP zero-shot classification
- **Interactive mode** — pick a folder and a preset, watch the plan change, run, undo
- **History** — every run is remembered; undo the last one, or any one, from anywhere
- **Nothing is ever overwritten** — conflicts get a new name, skip, or the old file goes to trash
- **Write-Ahead Journal** — the whole plan is on disk before the first file is touched; even an interrupted run can be undone
- **Parallel Processing** — multi-worker execution for large folders
- **Cross-Platform** — single binary for Windows, Linux, and macOS

---

## 🚀 Installation

### Pre-built Binaries (recommended)

Download the latest release from [GitHub Releases](https://github.com/lookinALX/FIONA/releases/latest):
- `fiona` — main binary
- `fiona-classifier` — required for ML classification (`{ml}`)

Place both files in the same directory and run.

### Build from Source

**Requirements:** Go 1.24+

```bash
git clone https://github.com/lookinALX/FIONA.git
cd FIONA
go build -o fiona ./cmd/fiona
```

**For ML classification**, also build the Python classifier:

```bash
cd ml
pip install -r requirements.txt
pyinstaller \
  --onefile \
  --strip \
  --name fiona-classifier \
  --exclude-module matplotlib \
  --exclude-module scipy \
  --exclude-module notebook \
  --exclude-module jupyter \
  --exclude-module IPython \
  --exclude-module tkinter \
  --exclude-module PyQt5 \
  --exclude-module wx \
  --exclude-module torchaudio \
  --exclude-module cv2 \
  --exclude-module sklearn \
  --exclude-module pandas \
  --exclude-module numpy.testing \
  --exclude-module unittest \
  --exclude-module xmlrpc \
  --exclude-module email \
  --exclude-module html \
  --exclude-module http.server \
  classifier.py
# binary will be in ml/dist/fiona-classifier
```

Place `fiona-classifier` in the same directory as the `fiona` binary.

---

## 🎯 Quick Start

```
$ fiona ~/Downloads
Sorting ~/Downloads with preset "downloads"

  images/       86 files   412 MB
  documents/    61 files    95 MB  exists
  archives/     12 files   1.2 GB
  installers/    9 files   2.1 GB
  code/          8 files   140 KB
  other/        38 files    52 MB  .torrent 20, .bin 9, .dat 4, …

  Left as is: 3 folders

Move 214 files (1.8 GB) into 6 folders? [y/N/l] y
✓ Moved 214 files in 1.2s. Undo: fiona undo

$ fiona undo
Undo the run from today 15:28 (moved 214 files in ~/Downloads)? [y/N] y
✓ Restored 214 files, removed 6 empty folders.
```

Answer `l` to see where every file goes before deciding.

```bash
fiona                                  # interactive mode in the current folder
fiona -p photos ~/Pictures             # photos by year and month taken
fiona sort ~/Downloads --to "{year}/{ext}" --dry-run
fiona sort ~/Downloads ~/Sorted --copy -r   # copy into another folder, subfolders too
fiona history                          # what was sorted, what is undone
```

---

## 📖 Commands

```
fiona [FOLDER]                      sort FOLDER (default: the current one) with the default preset
fiona sort [FLAGS] [SOURCE] [DEST]  sort SOURCE into folders inside DEST (default: SOURCE)
fiona undo [RUN-ID] [FLAGS]         undo the last run, or the one given; again for the one before
fiona history [--all]               list runs and whether they are undone
fiona presets [show NAME]           list presets, or print one as JSON to copy into the config
fiona tui [FOLDER]                  interactive mode, also started by fiona without arguments
fiona --help | --version
```

Flags may go before or after the folders. Long flags take two dashes.

### sort flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--preset` | `-p` | `default_preset` of the config, else `downloads` | Preset of rules |
| `--to` | — | — | Path template for all files, like `"{year}/{month}"` |
| `--criteria` | `-c` | — | Group by: `category` (`mimetype`), `extension`, `year`, `month`, `day`, `date`, `size`, `ml` |
| `--then` | `-t` | — | Then group inside by another criterion |
| `--source` | `-s` | current folder | Same as `SOURCE` |
| `--dest` | `-d` | `SOURCE` | Same as `DEST`; created right before execution if missing |
| `--copy` | — | off | Copy instead of move |
| `--action` | `-a` | `move` | `move` or `copy` |
| `--on-conflict` | — | `rename` | `rename`, `skip`, `replace` (the old file goes to trash) |
| `--recursive` | `-r` | off | Sort files in subfolders too |
| `--hidden` | — | off | Include files and folders starting with a dot |
| `--list` | `-l` | off | List every file in the plan |
| `--dry-run` | `-n` | off | Show the plan and exit; stronger than `--yes` |
| `--yes` | `-y` | off | Do not ask for confirmation |
| `--workers` | `-w` | CPU count × 2 | Number of files processed at once |
| `--log` | — | `<dest>/.fiona/runs` | Folder for the run journal |
| `--ml-config` | — | — | Custom ML categories, for `{ml}` |

The rules come from the first of: `--to`, `-c`/`-t`, `--preset`, `default_preset` of the config, the built-in `downloads`. The first line of the plan says which one is used.

### undo flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `RUN-ID` | — | — | A run from `fiona history`; a prefix is enough |
| `--log` | — | — | Journal file of the run to undo (`<run-id>.jsonl`) |
| `--dest` | `-d` | — | Undo the last run that sorted into this folder |
| `--dry-run` | `-n` | off | Show what would be undone and exit |
| `--yes` | `-y` | off | Do not ask for confirmation |
| `--workers` | `-w` | CPU count × 2 | Number of files processed at once |

Without `RUN-ID`, `--log` and `--dest`, `fiona undo` takes the newest run that is not undone yet, wherever it was.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | done, cancelled, or nothing to do |
| 1 | something failed or the run was interrupted |
| 2 | a mistake in the command line or the config; nothing is touched |

Mistakes come with a suggestion: `fiona: unknown preset "photso". Did you mean "photos"?`

---

## 🧩 Presets and Rules

### Built-in presets

| Preset | Rules |
|--------|-------|
| `downloads` (default) | everything → `{category}` |
| `photos` | images and videos → `{year}/{month}`; everything else stays |
| `by-date` | everything → `{year}/{month}` |
| `by-type` | everything → `{category}/{ext}` |

### Your own presets

The config is `config.json` in the FIONA folder of your user configuration directory (`~/Library/Application Support/fiona` on macOS, `~/.config/fiona` on Linux, `%AppData%\fiona` on Windows; `fiona presets` prints the path). `FIONA_CONFIG_DIR` overrides it.

```json
{
  "default_preset": "work",
  "action": "move",
  "on_conflict": "rename",
  "presets": {
    "work": {
      "description": "Invoices by year, photos by month, the rest by type",
      "rules": [
        { "match": { "name": "invoice*", "ext": ["pdf"] }, "to": "invoices/{year}" },
        { "match": { "category": ["images"] }, "to": "photos/{year}/{month}" },
        { "to": "{category}" }
      ]
    }
  }
}
```

- The first rule that matches a file wins. A file no rule matches stays where it is; the plan counts it under `Left as is`.
- `match` may test `category` (list), `ext` (list, without the dot) and `name` (glob on the file name). Case does not matter. Filled fields must all match; an empty `match` matches everything.
- A preset with the name of a built-in one replaces it.
- `fiona presets show downloads` prints a preset as a piece of config to start from.
- A mistake is reported with the place: `config.json: preset "work", rule 2: unknown placeholder {yaer}. Did you mean {year}?`

### Templates

`/` separates folders; text and placeholders can be mixed: `photos/{year}`, `{year}-{month} trip`.

| Placeholder | Value |
|-------------|-------|
| `{category}` | `images`, `documents`, … |
| `{ext}` | extension in lower case, `_no_ext` if there is none |
| `{year}` | `2025` |
| `{month}` | `01` |
| `{day}` | `15` |
| `{date}` | `2025-01-15` |
| `{size}` | `tiny` (< 10 KB), `small` (< 100 KB), `medium` (< 1 MB), `large` (< 10 MB), `huge` (< 100 MB), `massive` |
| `{ml}` | tag of the ML classifier; non-images get their category |

`-c X -t Y` is the same as `--to "{X}/{Y}"`.

A folder that already exists in another case is reused: files for `images/` go into an existing `Images/`.

---

## 🗂️ Categories and Dates

| Category | Extensions |
|----------|------------|
| `images` | jpg jpeg png gif bmp webp svg heic heif avif tif tiff ico psd cr2 cr3 nef arw dng raf orf rw2 |
| `videos` | mp4 mov avi mkv webm flv wmv m4v mpg mpeg 3gp mts |
| `audio` | mp3 wav flac aac ogg m4a opus wma aiff |
| `documents` | pdf doc docx odt rtf txt md pages xls xlsx ods csv numbers ppt pptx odp key epub fb2 mobi djvu |
| `archives` | zip rar 7z tar gz tgz bz2 xz zst |
| `installers` | dmg pkg exe msi deb rpm apk appimage iso |
| `code` | go py js ts jsx tsx java c h cpp hpp rs rb php sh html css json xml yaml yml toml sql ipynb |
| `other` | everything else |

A file without an extension, or with an unknown one, is recognised by its first bytes: a PNG called `picture` goes to `images`, a text file `README` to `documents`.

Date placeholders use the date a photo was taken, read from EXIF (`DateTimeOriginal`) of JPEG, HEIC/HEIF/AVIF, TIFF and RAW files built on it (DNG, CR2, NEF, ARW, ORF, RW2) and Fujifilm RAF. Other files, and photos without EXIF, use the modification time. Dates are in local time.

---

## 🖥️ Interactive Mode

`fiona` without arguments in a terminal, or `fiona tui [FOLDER]`.

The left panel is the folder tree, opened at the current folder. The right panel is the plan for the folder under the cursor and the chosen preset — the same overview the CLI prints. It changes as you move through the tree or switch presets; nothing on disk changes until you answer `y`.

| Key | Action |
|-----|--------|
| `↑ ↓ → ←`, `e` | move through the folders; type a path |
| `p`, `shift+p` | next or previous preset; `custom` takes a template (`t` to edit) |
| `c`, `d`, `r`, `.` | copy instead of move; another destination; subfolders; hidden files |
| `tab`, `l` | switch to the plan; list every file |
| `m` | run the ML classifier when the preset uses `{ml}` |
| `enter`, then `y` | sort |
| `u` (after a run) | undo it |
| `h` | history: undo any run |
| `?`, `q` | help, quit |

---

## ↩️ History and Undo

Every executed run is added to `history.jsonl` next to the config (readable by the owner only). The state of each run is read from its journal.

```
$ fiona history
  RUN                   WHEN             ACTION  FOLDER                 FILES  STATE
  20261017-132835-1d51  today 15:28      move    ~/Downloads            214    done
  20261016-090112-a3f0  yesterday 09:01  copy    ~/Camera → ~/Pictures  58     undone
```

`fiona undo` undoes the newest run that is not undone yet; run it again for the one before, like Ctrl+Z. `fiona undo 20261016` picks a run by the start of its id. When everything is undone already, it says so and exits with 0.

---

## 🛡️ Data Safety

**What is scanned.** By default only regular files at the top level of the source. With `-r` FIONA goes into subdirectories, but never into bundles (`.app`, `.bundle`, `.framework`, `.photoslibrary`, `.xcodeproj`) or projects (directories containing `.git`, `.hg`, `.svn`, `node_modules`, `go.mod` or `package.json`). Hidden entries are skipped unless `--hidden` is given. Symlinks and special files are never followed or sorted; they are reported in the plan as `Skipped by scanner`. If the destination lies inside the source, it is not scanned.

**Nothing is overwritten.** Every conflict is resolved in the plan, before execution:

| `--on-conflict` | Behaviour |
|-----------------|-----------|
| `rename` (default) | the new file gets the first free name: `photo_1.jpg`, `photo_2.jpg`, … |
| `skip` | the source file stays where it is |
| `replace` | the existing file is moved to `<dest>/.fiona/trash/<run-id>/`, then the new one takes its place |

Trash of runs older than 5 days is removed automatically the next time a sort is executed in the same destination.

**Journal.** Each executed run writes `<dest>/.fiona/runs/<run-id>.jsonl` (JSON Lines): the whole plan first, then the result of every operation. If the journal cannot be created, no file is touched.

**Undo.** `fiona undo` names the run and its date and asks for confirmation. It moves files back (or removes copies), returns replaced files from trash and removes the directories the run created if they are empty. A file that was changed after the run (different size or modification time) or whose original location is occupied is skipped, never overwritten. A run interrupted by Ctrl+C or a crash can be undone too. A journal that points outside of its own source and destination is rejected as a whole. Each run can be undone once.

### Upgrading from 0.4.x

- The default action is `move`, it was `copy`; `--copy` brings the old behaviour back.
- The destination defaults to the source; before, both defaulted to the current folder.
- Folder names: `audios` → `audio`, `applications` → `installers`, `unknown` → `other`; extensions in folder names are lower case.
- `date` gives `2025-01-15` and `month` gives `01`, instead of `January 2025` and `January`.
- Long flags with one dash (`-criteria`) no longer work.
- The plan lists folders, not files; use `--list` or answer `l`. The journal path is shown by `fiona history`.
- Mistakes in the command line or the config exit with 2 instead of 1.

### Upgrading from 0.3.x

- Scanning is not recursive and skips hidden files; the old behaviour is `-r --hidden`.
- `--on-conflict` defaults to `rename` instead of `replace`.
- `--dry-run` is off by default and never executes anything; `--force yes` is removed, use `--yes`.
- The journal is `<dest>/.fiona/runs/<run-id>.jsonl` instead of `fiona_logs.json` in the current directory. Old journals cannot be used for undo.
- `--workers 0` is an error.

---

## 🤖 ML Classification

When a rule uses `{ml}` (`--to "{ml}"`, `-c ml`, or a preset), FIONA spawns `fiona-classifier` — a standalone Python binary with a CLIP model — and sorts images into semantic category folders. Only the files whose matching rule uses `{ml}` are sent to the classifier: with `{ "match": { "category": ["images"] }, "to": "photos/{ml}" }` documents are never classified. In interactive mode the classifier starts only when you press `m`.

### Default Categories

| Category | Description |
|----------|-------------|
| `people` | Portraits, selfies, group photos |
| `family` | Family gatherings, relatives |
| `pets` | Dogs, cats, domestic animals |
| `nature` | Landscapes, forests, mountains |
| `food` | Meals, restaurants, cooking |
| `travel` | Trips, landmarks, sightseeing |
| `events` | Parties, weddings, celebrations |
| `sports` | Athletic activities, fitness |
| `home` | Rooms, furniture, interiors |
| `vehicles` | Cars, motorcycles, aircraft |
| `work` | Office, desks, meetings |
| `documents` | Scans, receipts, printed text |
| `screenshots` | Screen captures, app UIs |
| `other` | Everything else |

### Custom Categories

Pass a JSON config file with custom category names and CLIP prompts:

```json
{
  "dad": "a photo of my father, an older man with grey hair",
  "vacation_2024": "photos from our summer vacation in Italy",
  "cats": "a photo of a cat"
}
```

```bash
fiona sort ~/Photos ~/Sorted --to "{ml}" --ml-config ~/my_categories.json
```

### Non-Image Files

Files that cannot be opened as images (PDFs, videos, text files) are automatically classified by MIME type and sorted alongside images without interrupting the operation.

### ML Log

After each run, `fiona-classifier` writes a detailed log to `ml_log_results.json` in the working directory containing confidence scores for every classified image.

### Requirements

- `fiona-classifier` binary must be in the same directory as `fiona`
- GPU is used automatically if available (CUDA), otherwise CPU

---

## 💡 Examples

### Organize Downloads by type, in place

```bash
fiona ~/Downloads
```

**Result:**
```
Downloads/
├── images/
│   ├── photo.jpg
│   └── screenshot.png
├── documents/
│   ├── report.pdf
│   └── notes.txt
├── videos/
│   └── clip.mp4
└── archives/
    └── backup.zip
```

### Photos by the month they were taken, leaving everything else

```bash
fiona -p photos ~/Pictures
```

**Result:**
```
Pictures/
├── 2024/
│   └── 12/
├── 2025/
│   ├── 01/
│   └── 03/
└── notes.pdf
```

### Copy into another folder by type and extension

```bash
fiona sort ~/Documents ~/Sorted -p by-type --copy -r
```

### Sort photos using ML classification

```bash
fiona sort ~/Photos ~/Photos/Sorted --to "{ml}" --yes
```

### See every file before deciding

```bash
fiona ~/Downloads --list --dry-run
```

### Undo

```bash
fiona undo                        # the last run, wherever it was
fiona undo                        # and the one before it
fiona undo 20261010-1530          # a run from fiona history
fiona undo --dest ~/Organized     # the last run into this folder
```

---

## 🛠️ Development

### Project Structure

```
FIONA/
├── cmd/fiona/           # subcommands: sort, undo, history, presets, tui
├── internal/
│   ├── app/             # the pipeline shared by the CLI and the TUI
│   ├── cli/             # flags, validation, prompt, progress, help
│   ├── config/          # config.json and the built-in presets
│   ├── history/         # global index of runs
│   ├── report/          # plan overview and results as text
│   ├── rules/           # templates, placeholders, rules and matching
│   ├── scanner/         # traversal, categories, EXIF
│   ├── sorter/          # planner, executor, reverter
│   ├── journal/         # write-ahead run journal (JSON Lines)
│   ├── suggest/         # "did you mean" for typos
│   ├── tui/             # interactive mode
│   └── types/           # shared types (Action, LogEntry, etc.)
├── ml/
│   ├── classifier.py    # FastAPI server with CLIP model
│   ├── requirements.txt # Python dependencies
│   └── test_classifier.py
├── tests/               # end-to-end tests of the binary
├── go.mod
└── README.md
```

### Running Tests

```bash
# All tests
go test ./...

# With race detector
go test -race ./...

# Integration tests only
go test ./tests -v

# With coverage
go test -cover ./...

# Python classifier tests
cd ml && pytest test_classifier.py -v
```

### Adding a Placeholder

Add an entry to `rules.Placeholders` in `internal/rules/placeholder.go`:

```go
{"weekday", "Monday, Tuesday, …", func(fi *scanner.FileInfo) string { return fi.Date().Weekday().String() }},
```

It becomes available in `--to`, in presets and in the help at once.

---

## 🗺️ Roadmap

### ✅ Milestone 1 — Core (Complete)
- File scanning with full metadata
- Rule-based sorting: mimetype, extension, date, year, month, size
- Primary + secondary criteria with nested directory structure
- Dry-run preview with tree visualization
- Copy / move with conflict resolution (replace, skip, rename)
- Metadata preservation (permissions, modification time)
- Cross-filesystem move support

### ✅ Milestone 2 — Performance & Robustness (Complete)
- Parallel processing with configurable worker pool
- Real-time progress bar
- Transaction logging to JSON
- Undo / rollback via `fiona undo`
- Subcommand CLI (`sort`, `undo`)

### ✅ Milestone 3 — ML Image Classification (Complete)
- FastAPI Python server with CLIP model (`fiona-classifier` binary)
- Batch image classification via HTTP
- Go subprocess management — start/stop `fiona-classifier` automatically
- `ByMLRule` — sort files by ML-assigned tags
- Fallback to MIME type for non-image files
- Custom categories via JSON config file
- `--ml-config` flag wired into CLI
- `fiona-classifier` PyInstaller build optimisation (271 MB, CPU-only)

### ✅ Milestone 4 — Simplicity and TUI (Complete)
- `fiona <folder>` without flags, `move` into the same folder by default
- Presets and a user config; rules per kind of file with path templates
- Categories for ~140 extensions and detection by content; EXIF date taken
- Global history: `fiona undo` like Ctrl+Z, `fiona history`
- Interactive mode: live plan preview, run with progress, undo, history browser
- Compact output, "did you mean" for typos, exit code 2 for mistakes

### 📅 Milestone 5 — Web Interface
- REST API with `net/http`
- Browser-based UI with embedded assets
- Real-time operation monitoring

### 📅 Future
- `fiona analyze` — unsupervised clustering + LLM to suggest folder structure automatically
- Watch mode for continuous directory monitoring
- Duplicate detection and deduplication
- Cloud storage integration (S3, Google Drive, Dropbox)
- Plugin system for custom rules

---

## 📄 License

MIT — see [LICENSE](LICENSE)

---

**Made with ❤️ and Go — [@lookinALX](https://github.com/lookinALX)**