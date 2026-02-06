# FIONA

<p align="center">
  <img src="assets/FIONA_logo.png" alt="FIONA Logo" width="600"/>
</p>

<p align="center">
  <strong>F</strong>ile <strong>I</strong>ntelligent <strong>O</strong>rganization & <strong>N</strong>avigation <strong>A</strong>ssistant
</p>

<p align="center">
  A cross-platform file organizer written in Go that automatically sorts files based on configurable rules with future ML-powered image classification.
</p>

<p align="center">
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go" alt="Go Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
</p>

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

---

## 📋 Table of Contents

- [Features](#-features)
- [Installation](#-installation)
- [Quick Start](#-quick-start)
- [Usage](#-usage)
- [Sorting Rules](#-sorting-rules)
- [Configuration](#-configuration)
- [Examples](#-examples)
- [Development](#-development)
- [Roadmap](#-roadmap)
- [Contributing](#-contributing)
- [License](#-license)

---

## ✨ Features

### Current (Milestone 1 - Complete)

- **Smart File Sorting**: Organize files by type, extension, or date
- **Hierarchical Organization**: Primary + secondary sorting criteria (e.g., type/extension → `images/jpg/`)
- **Conflict Resolution**: Multiple strategies for handling existing files:
  - Replace: Overwrite existing files
  - Skip: Keep original files
  - Rename: Auto-increment filename (`file_1.jpg`, `file_2.jpg`, etc.)
- **Dry-Run Mode**: Preview operations before execution with interactive confirmation
- **Copy or Move**: Choose whether to copy or move files
- **Metadata Preservation**: Maintains file permissions and modification times
- **Cross-Platform**: Single binary for Windows, Linux, and macOS
- **Beautiful CLI Output**: Tree-view visualization of planned operations

### Planned Features

- **Parallel Processing** (M2): Multi-threaded file operations for speed
- **ML Image Classification** (M3): Automatic categorization using deep learning
- **Web Interface** (M4): Browser-based UI for non-CLI users
- **Watch Mode**: Continuously monitor directories and auto-sort new files
- **Undo/Rollback**: Reverse operations with transaction history
- **Custom Rules DSL**: Advanced rule configuration language

---

## 🚀 Installation

### Pre-built Binaries

Download the latest release for your platform:

```bash
# Linux
wget https://github.com/yourusername/fiona/releases/latest/download/fiona-linux-amd64
chmod +x fiona-linux-amd64
sudo mv fiona-linux-amd64 /usr/local/bin/fiona

# macOS
wget https://github.com/yourusername/fiona/releases/latest/download/fiona-darwin-amd64
chmod +x fiona-darwin-amd64
sudo mv fiona-darwin-amd64 /usr/local/bin/fiona

# Windows
# Download fiona-windows-amd64.exe and add to PATH
```

### Build from Source

**Requirements:**
- Go 1.21 or higher

```bash
git clone https://github.com/yourusername/fiona.git
cd fiona
go build -o fiona ./cmd/fiona
```

---

## 🎯 Quick Start

### Basic Usage

```bash
# Organize Downloads folder by file type
fiona -source ~/Downloads -dest ~/Downloads/Organized -primary type

# Sort photos by type then extension (images/jpg/, images/png/)
fiona -source ~/Pictures -dest ~/Pictures/Sorted -primary type -secondary extension

# Preview without executing (dry-run mode)
fiona -source ~/Documents -dest ~/Documents/Sorted -primary extension -dry-run
```

### Interactive Mode

By default, FIONA shows a preview and asks for confirmation:

```
================================================
              DRY-RUN PLAN
================================================

  Action:      copy
  Source:      /home/user/Downloads
  Destination: /home/user/Downloads/Organized

================================================

  Total: 42 files, 156.7 MB

================================================

  📁 images/  (15 files, 45.2 MB)
  ├── 📂 jpg/  (10 files, 32.1 MB)
  │   ├── photo1.jpg
  │   ├── photo2.jpg
  │   └── vacation.jpg
  └── 📂 png/  (5 files, 13.1 MB)
      ├── screenshot1.png
      └── logo.png

  📁 documents/  (8 files, 12.5 MB)
  └── 📂 pdf/  (8 files, 12.5 MB)
      ├── report.pdf
      └── invoice.pdf

================================================
Would you like to continue? (y/N)
```

---

## 📖 Usage

### Command-Line Flags

```
Usage: fiona [options]

Required:
  -source string
        Source directory to scan
  -dest string
        Destination directory for organized files

Sorting:
  -primary string
        Primary sorting criterion: type, extension, date (default "type")
  -secondary string
        Secondary sorting criterion: type, extension, date

Operation:
  -action string
        File operation: copy, move (default "copy")
  -conflict string
        Conflict resolution: replace, skip, rename (default "skip")
  -force string
        Skip confirmation prompt: yes, no (default "no")
  -dry-run
        Preview operations without executing (default true)

Scanning:
  -recursive
        Include subdirectories (default true)
  -hidden
        Include hidden files (default false)
```

---

## 🗂️ Sorting Rules

### Primary Criteria

#### By Type (`-primary type`)
Groups files by category based on extension:
- **images**: jpg, jpeg, png, gif, bmp, svg, webp
- **documents**: txt, pdf, doc, docx, xls, xlsx, ppt, pptx
- **videos**: mp4, avi, mkv, mov, flv, webm
- **audios**: mp3, wav, flac, aac, ogg
- **archives**: zip, rar, 7z, tar, gz
- **applications**: exe, msi, apk, deb, rpm
- **unknown**: everything else

**Result:** `images/`, `documents/`, `videos/`, etc.

#### By Extension (`-primary extension`)
Groups files by file extension:

**Result:** `jpg/`, `pdf/`, `mp4/`, `txt/`, etc.

#### By Date (`-primary date`)
Groups files by modification date:

**Result:** `2025/01/`, `2024/12/`, etc.

### Hierarchical Sorting

Combine primary and secondary criteria for deeper organization:

```bash
# Type → Extension
fiona -primary type -secondary extension
# Result: images/jpg/, images/png/, documents/pdf/

# Extension → Type
fiona -primary extension -secondary type
# Result: jpg/images/, png/images/, pdf/documents/

# Date → Type
fiona -primary date -secondary type
# Result: 2025/01/images/, 2025/01/documents/
```

---

## ⚙️ Configuration

### Conflict Strategies

**Replace** (`-conflict replace`)
```bash
# Overwrite existing files
fiona -source ~/Downloads -dest ~/Organized -conflict replace
```

**Skip** (`-conflict skip`)
```bash
# Keep existing files, don't copy/move duplicates
fiona -source ~/Downloads -dest ~/Organized -conflict skip
```

**Rename** (`-conflict rename`)
```bash
# Auto-increment filenames: photo.jpg → photo_1.jpg → photo_2.jpg
fiona -source ~/Downloads -dest ~/Organized -conflict rename
```

### Copy vs Move

**Copy** (default)
```bash
# Preserve original files
fiona -action copy -source ~/Downloads -dest ~/Backup
```

**Move**
```bash
# Delete originals after successful copy
fiona -action move -source ~/Downloads -dest ~/Organized
```

---

## 💡 Examples

### Organize Downloads by Type

```bash
fiona \
  -source ~/Downloads \
  -dest ~/Downloads/Organized \
  -primary type \
  -action move \
  -conflict rename \
  -force yes
```

**Before:**
```
Downloads/
├── photo.jpg
├── report.pdf
├── video.mp4
├── song.mp3
└── archive.zip
```

**After:**
```
Downloads/Organized/
├── images/
│   └── photo.jpg
├── documents/
│   └── report.pdf
├── videos/
│   └── video.mp4
├── audios/
│   └── song.mp3
└── archives/
    └── archive.zip
```

### Sort Photos Hierarchically

```bash
fiona \
  -source ~/Pictures \
  -dest ~/Pictures/Sorted \
  -primary type \
  -secondary extension \
  -action copy
```

**Result:**
```
Pictures/Sorted/
└── images/
    ├── jpg/
    │   ├── vacation1.jpg
    │   └── vacation2.jpg
    ├── png/
    │   └── screenshot.png
    └── gif/
        └── animation.gif
```

### Organize by Date

```bash
fiona \
  -source ~/Documents \
  -dest ~/Documents/Archive \
  -primary date \
  -secondary type \
  -action copy
```

**Result:**
```
Documents/Archive/
├── 2025/01/
│   ├── images/
│   └── documents/
└── 2024/12/
    ├── images/
    └── documents/
```

---

## 🛠️ Development

### Project Structure

```
fiona/
├── cmd/fiona/
│   └── main.go                 # Entry point
├── internal/
│   ├── cli/
│   │   ├── config.go           # CLI options and flags
│   │   └── flags.go            # Flag parsing
│   ├── scanner/
│   │   ├── scanner.go          # Directory traversal
│   │   └── fileinfo.go         # File metadata
│   ├── rules/
│   │   ├── rule.go             # Rule interface
│   │   ├── bytype.go           # Sort by file type
│   │   ├── byextension.go      # Sort by extension
│   │   └── bydate.go           # Sort by date
│   └── sorter/
│       ├── planner.go          # Action planning
│       ├── executor.go         # File operations
│       └── plan_test.go        # Tests
├── tests/
│   └── integration_test.go     # End-to-end tests
├── go.mod
├── Makefile
└── README.md
```

### Building

```bash
# Build for current platform
make build

# Cross-compile for all platforms
make build-all

# Run tests
make test

# Run with coverage
make coverage

# Format code
make fmt

# Lint
make lint
```

### Running Tests

```bash
# All tests
go test ./...

# With verbose output
go test -v ./...

# Specific package
go test ./internal/sorter -v

# Integration tests
go test ./tests -v

# With coverage
go test -cover ./...
```

### Adding New Rules

Implement the `Rule` interface:

```go
// internal/rules/rule.go
type Rule interface {
    GetDestination(fi *scanner.FileInfo) string
}
```

Example:

```go
// internal/rules/bysize.go
package rules

import "FIONA/internal/scanner"

type BySizeRule struct {
    IsPrimary bool
}

func (r *BySizeRule) GetDestination(fi *scanner.FileInfo) string {
    const mb = 1024 * 1024
    
    if fi.Size < mb {
        return "small"
    } else if fi.Size < 10*mb {
        return "medium"
    }
    return "large"
}
```

Register in factory:

```go
// internal/cli/flags.go
var ruleFactories = map[string]func(bool) rules.Rule{
    "type":      func(p bool) rules.Rule { return &rules.ByTypeRule{IsPrimary: p} },
    "extension": func(p bool) rules.Rule { return &rules.ByExtensionRule{IsPrimary: p} },
    "date":      func(p bool) rules.Rule { return &rules.ByDateRule{IsPrimary: p} },
    "size":      func(p bool) rules.Rule { return &rules.BySizeRule{IsPrimary: p} }, // NEW
}
```

---

## 🗺️ Roadmap

### ✅ Milestone 1: Core Functionality (Complete)
- File scanning with metadata collection
- Rule-based sorting (type, extension, date)
- Hierarchical organization (primary + secondary)
- Dry-run preview with tree visualization
- Copy/move operations with conflict resolution
- Metadata preservation

### 🚧 Milestone 2: Performance & Robustness (In Progress)
- [ ] Parallel file processing with worker pools
- [ ] Progress bar with real-time statistics
- [ ] Comprehensive error handling and recovery
- [ ] Transaction log for operations
- [ ] Undo/rollback functionality
- [ ] Watch mode for continuous monitoring

### 📅 Milestone 3: ML Integration (Planned)
- [ ] Python bridge via stdin/stdout IPC
- [ ] Pre-trained image classification model
- [ ] Custom ML categories configuration
- [ ] Confidence threshold settings
- [ ] Fallback to rule-based sorting

### 📅 Milestone 4: Web Interface (Planned)
- [ ] REST API with net/http
- [ ] Browser-based UI
- [ ] Real-time operation monitoring
- [ ] Configuration management
- [ ] Embedded static assets (single binary)

### 📅 Future Enhancements
- [ ] Plugin system for custom rules
- [ ] Cloud storage integration (S3, Drive, Dropbox)
- [ ] Duplicate file detection and deduplication
- [ ] Compression and archiving
- [ ] Content-based file organization
- [ ] GUI application (native or Electron)

---

## 🤝 Contributing

Contributions are welcome! Please follow these guidelines:

1. **Fork the repository**
2. **Create a feature branch**: `git checkout -b feature/amazing-feature`
3. **Write tests** for new functionality
4. **Format code**: `make fmt`
5. **Run tests**: `make test`
6. **Commit changes**: `git commit -m 'Add amazing feature'`
7. **Push to branch**: `git push origin feature/amazing-feature`
8. **Open a Pull Request**

### Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Write meaningful commit messages
- Add comments for complex logic
- Keep functions small and focused

### Testing

- Write table-driven tests where appropriate
- Maintain >80% code coverage
- Test edge cases and error conditions
- Use `t.TempDir()` for file system tests

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🙏 Acknowledgments

- Inspired by various file organization tools
- Built with [Go](https://go.dev/)
- Future ML powered by PyTorch

---

## 📞 Contact

- **Author**: Alexander
- **GitHub**: [@lookinALX](https://github.com/lookinALX)


---

## 🌟 Star History

If you find FIONA useful, please consider giving it a star ⭐

---

**Made with ❤️ and Go**