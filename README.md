![alt text](FIONA_logo.png)
# File Intelligence Organization and Navigation Assistant

You can group your files by different options, such as category or extension.

## Installation

1. Download the appropriate file for your system:
   - Windows 64-bit: `fiona-win-x64.exe`
   - Windows 32-bit: `fiona-win-x86.exe`
   - Linux 64-bit: `fiona-linux-x64`
   - macOS 64-bit: `fiona-osx-x64`

2. Rename to `fiona.exe` (Windows) or `fiona` (Linux/Mac)

3. Add to PATH:
   - **Windows**: Copy to `C:\tools\` and add to PATH
   - **Linux/Mac**: Copy to `/usr/local/bin/` or `~/bin/`

## Usage
For now, only the CLI application is available.

```bash
fiona              # Interactive mode
fiona group --by extension
fiona rollback