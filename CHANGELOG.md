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