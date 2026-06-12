## Milestone 4 — TUI Interface [Unreleased]

### 12.06.2026

#### Added
- `fiona tui` subcommand launching a full-screen terminal UI (charmbracelet/bubbletea)
- ASCII art logo embedded from `assets/ascii-art.txt` via `//go:embed`
- Primary/Secondary sort criterion selectors with keyboard (◀/▶) and mouse support
- Custom file tree component with expand/collapse, scrolling, and mouse click navigation
- Source and Destination directory pickers displayed side-by-side when terminal height is limited
- Adaptive horizontal layout: trees switch to side-by-side mode when they don't fit stacked
- Bottom menu bar with Sort/Quit buttons and navigation hints (cream background)
- Logo fallback to text "FIONA" when terminal height is too small; logo is prioritised on resize
- Too-narrow guard: shows error screen when terminal width is less than logo width
- Mouse support via `tea.WithMouseCellMotion()`; hidden text cursor via `tea.HideCursor`
- FIONA colour palette (`#2E4A22`, `#5C7A3A`, `#8A9E6A`, `#D8DDB5`) applied throughout UI

---

## Milestone 3 — ML Image Classification [Unreleased]

### 20.03.2026

#### Added
- `--ml-config` flag for custom ML category configuration via JSON file
- Custom category support: pass a JSON file with category names and CLIP prompts
- Dynamic port allocation for `fiona-classifier` subprocess (no more port conflicts)
- Config validation on both Go side (file exists, .json extension) and Python side (non-empty keys/values)

#### Changed
- `ml.NewServer()` now accepts `configPath` parameter, passed as `--config` argument to `fiona-classifier`
- `fiona-classifier` accepts `--config` and `--port` CLI arguments (replaces positional args)

#### Optimized
- PyInstaller build reduced from 6.8 GB to 271 MB via CPU-only PyTorch build
- Removed unnecessary `torchvision` dependency
- Added `--strip` flag to PyInstaller build
- `requirements.txt` now pins CPU-only PyTorch via `--index-url`