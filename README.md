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
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Go Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/lookinalx/fiona"><img src="https://goreportcard.com/badge/github.com/lookinalx/fiona" alt="Go Report"></a>
</p>

---

## 📋 Table of Contents

- [Features](#-features)
- [Installation](#-installation)
- [Quick Start](#-quick-start)
- [Commands](#-commands)
- [Sorting Criteria](#-sorting-criteria)
- [ML Classification](#-ml-classification)
- [Examples](#-examples)
- [Development](#-development)
- [Roadmap](#-roadmap)
- [License](#-license)

---

## ✨ Features

- **Smart File Sorting** — organize files by MIME type, extension, date, size, or a combination
- **ML Image Classification** — sort photos into semantic categories using CLIP zero-shot classification
- **Hierarchical Organization** — primary + secondary criteria (e.g., `images/jpg/`, `2025/01/`)
- **Parallel Processing** — multi-worker execution for fast operation on large directories
- **Conflict Resolution** — replace, skip, or auto-rename on filename collision
- **Dry-Run Mode** — preview the full operation plan before executing
- **Copy or Move** — choose whether originals are preserved or relocated
- **Metadata Preservation** — maintains file permissions and modification times
- **Transaction Log** — every operation is logged to JSON for auditing and rollback
- **Undo/Rollback** — reverse any sort operation using the transaction log
- **Progress Bar** — real-time progress during execution
- **Cross-Platform** — single binary for Windows, Linux, and macOS

---

## 🚀 Installation

### Pre-built Binaries (recommended)

Download the latest release from [GitHub Releases](https://github.com/lookinALX/FIONA/releases/latest):
- `fiona` — main binary
- `fiona-classifier` — required for ML classification (`-c ml`)

Place both files in the same directory and run.

### Build from Source

**Requirements:** Go 1.21+

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

```bash
# Preview sorting Downloads by MIME type (dry-run is on by default)
fiona sort -s ~/Downloads -d ~/Organized -c mimetype

# Execute immediately
fiona sort -s ~/Downloads -d ~/Organized -c mimetype -n=false --force yes

# Sort photos using ML classification
fiona sort -s ~/Photos -d ~/Photos/Sorted -c ml

# Undo last operation
fiona undo --log path/to/fiona_logs.json
```

---

## 📖 Commands

FIONA uses subcommands:

```
fiona sort [OPTIONS]    — sort files
fiona undo [OPTIONS]    — reverse a previous sort using a log file
fiona --version         — print version
fiona --help            — print help
```

### sort flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--criteria` | `-c` | `mimetype` | Primary sorting criterion |
| `--then` | `-t` | — | Secondary sorting criterion |
| `--source` | `-s` | current dir | Source directory |
| `--dest` | `-d` | current dir | Destination directory |
| `--action` | `-a` | `copy` | `copy` or `move` |
| `--on-conflict` | — | `replace` | `replace`, `skip`, `rename` |
| `--dry-run` | `-n` | `true` | Preview without executing |
| `--force` | — | `N` | Skip confirmation (`yes`) |
| `--workers` | `-w` | CPU count × 2 | Number of parallel workers |
| `--log` | — | current dir | Directory to write `fiona_logs.json` |

### undo flags

| Flag | Default | Description |
|------|---------|-------------|
| `--log` | `./fiona_logs.json` | Path to the log file to read |
| `--workers` | `-w` | Number of parallel workers |

---

## 🗂️ Sorting Criteria

| Criterion | Flag value | Result structure |
|-----------|-----------|-----------------|
| MIME type | `mimetype` | `images/`, `documents/`, `videos/`, ... |
| Extension | `extension` | `jpg/`, `pdf/`, `mp4/`, ... |
| Full date | `date` | `2025-01-15/` |
| Year | `year` | `2025/` |
| Month | `month` | `01/` |
| Size | `size` | `small/`, `medium/`, `large/` |
| ML | `ml` | `people/`, `animals/`, `food/`, ... |

Combine primary and secondary criteria for nested structure:

```bash
# mimetype → extension: images/jpg/, documents/pdf/
fiona sort -c mimetype -t extension -s ~/Photos

# year → month: 2025/01/, 2025/02/
fiona sort -c year -t month -s ~/Documents
```

---

## 🤖 ML Classification

When using `-c ml`, FIONA spawns `fiona-classifier` — a standalone Python binary with a CLIP model — classifies all images in the source directory, and sorts them into semantic category folders.

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
fiona sort -s ~/Photos -d ~/Sorted -c ml --ml-config ~/my_categories.json
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

### Organize Downloads by type, move files, rename on conflict

```bash
fiona sort \
  -s ~/Downloads \
  -d ~/Downloads/Organized \
  -c mimetype \
  -a move \
  --on-conflict rename \
  --force yes
```

**Result:**
```
Organized/
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

### Sort photos using ML classification

```bash
fiona sort -s ~/Photos -d ~/Photos/Sorted -c ml --force yes
```

**Result:**
```
Sorted/
├── people/
│   └── portrait.jpg
├── pets/
│   ├── dog1.jpg
│   └── dog2.jpg
├── food/
│   └── pizza.jpg
├── nature/
│   └── sunset.png
└── documents/
    └── scan.pdf
```

### Sort photos by year then month

```bash
fiona sort -s ~/Pictures -d ~/Pictures/Archive -c year -t month -a copy
```

**Result:**
```
Archive/
├── 2025/
│   ├── 01/
│   └── 03/
└── 2024/
    └── 12/
```

### Preview before executing

```bash
fiona sort -s ~/Documents -d ~/Sorted -c mimetype
# Shows plan, asks for confirmation

Would you like to continue? (y/N)
```

### Undo a previous sort

```bash
fiona undo --log ~/Organized/fiona_logs.json
```

---

## 🛠️ Development

### Project Structure

```
FIONA/
├── cmd/fiona/
│   └── main.go
├── internal/
│   ├── cli/             # flags, validation, config
│   ├── scanner/         # directory traversal, file metadata
│   ├── rules/           # sorting rule implementations
│   ├── sorter/          # planner, executor, reverter
│   ├── journal/         # transaction logging
│   └── types/           # shared types (Action, LogEntry, etc.)
├── ml/
│   ├── classifier.py    # FastAPI server with CLIP model
│   ├── requirements.txt # Python dependencies
│   └── test_classifier.py
├── tests/
│   └── integration_test.go
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

### Adding a New Rule

Implement the `Rule` interface:

```go
type Rule interface {
    GetDestination(fi *types.FileInfo) string
}
```

Register in `internal/cli/flags.go`:

```go
var ruleFactories = map[string]ruleFactory{
    "myrule": func(isPrimary bool) rules.Rule {
        return &rules.MyRule{IsPrimary: isPrimary}
    },
}
```

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

### 🚧 Milestone 3 — ML Image Classification (In Progress)

**FIONA** (full version) — CLIP-based zero-shot classification:
- [x] FastAPI Python server with CLIP model (`fiona-classifier` binary)
- [x] Batch image classification via HTTP
- [x] Go subprocess management — start/stop `fiona-classifier` automatically
- [x] `ByMLRule` — sort files by ML-assigned tags
- [x] Fallback to MIME type for non-image files
- [x] Custom categories via JSON config file
- [ ] `--ml-config` flag wired into CLI
- [ ] `fiona-classifier` PyInstaller build optimisation (target < 500MB)

**FIONA Light** — MobileNetV2-based classification (Planned):
- Lightweight alternative (~170MB)
- 14 categories via ImageNet class mapping
- Transfer learning: fine-tune on your own dataset via `fiona train`
- Distributed as `fiona-classifier-light` binary

**Future:**
- Face recognition via InsightFace for personal people tags

### 📅 Milestone 4 — Web Interface
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