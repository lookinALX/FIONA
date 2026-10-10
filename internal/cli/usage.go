package cli

import (
	"fmt"
	"os"
)

func PrintUsage() {
	fmt.Fprintf(os.Stderr, `
╔════════════════════════════════════════════════════╗
║            FIONA - File Organizer                  ║
╚════════════════════════════════════════════════════╝

Usage:
	fiona sort [OPTIONS]
	fiona undo [OPTIONS]

Organize and sort files into directories based on various criteria.

OPTIONS SORT:
  -c, --criteria <string>
        Primary sorting criteria (default: mimetype)
        Available: mimetype, extension, year, month, date, size

  -t, --then <string>
        Secondary sorting criteria (optional)
        Available: mimetype, extension, year, month, date, size

  -s, --source <path>
        Source directory to scan for files (default: current directory)

  -d, --dest <path>
        Destination directory for organized files (default: current directory)
        Created right before execution if it does not exist

  -a, --action <string>
        How to handle files: copy or move (default: copy)

  --on-conflict <string>
        How to handle file conflicts: rename, skip, or replace (default: rename)
        replace moves the existing file to <dest>/.fiona/trash/<run-id>/,
        trash older than 5 days is removed on the next executed sort

  -r, --recursive
        Scan subdirectories too (default: top-level files only)
        Never enters bundles (.app, .bundle, .framework, .photoslibrary, .xcodeproj)
        and projects (folders with .git, .hg, .svn, node_modules, go.mod, package.json)

  --hidden
        Include files and directories starting with a dot (default: skipped)

  -n, --dry-run
        Print the plan and exit, nothing on disk is changed; stronger than --yes

  -y, --yes
        Execute without confirmation

  -w, --workers <int>
        Number of workers for the current run, 1 or more (default: max availible)

  --log <path>
        Directory to write the run journal to (default: <dest>/.fiona/runs)

OPTIONS UNDO:
  --log <path>
        Journal file of the run to undo (<run-id>.jsonl)

  -d, --dest <path>
        Undo the last run of this destination directory that is not undone yet
        (default: current directory); cannot be combined with --log

  -n, --dry-run
        Print the undo preview and exit, nothing on disk is changed

  -y, --yes
        Execute without confirmation

  -w, --workers <int>
        Number of workers for the current run, 1 or more (default: max availible)

EXAMPLES:
  # Sort the top-level files of the current directory by extension (asks before executing)
  fiona sort -c extension

  # Sort photos by year, then month, and move them
  fiona sort -c year -t month -a move -s ~/Photos

  # Copy documents by type, including subdirectories, skipping files that already exist
  fiona sort -c mimetype -r --on-conflict skip -s ~/Documents -d ~/Organized

  # Preview only
  fiona sort -c extension --dry-run

  # Execute immediately without confirmation
  fiona sort -c extension --yes

  # Undo the last run of a destination directory
  fiona undo --dest ~/Organized

AVAILABLE CRITERIA:
  mimetype   - Group by MIME type (images, videos, documents, etc.)
  extension  - Group by file extension (.jpg, .pdf, .txt, etc.)
  year       - Group by file year
  month      - Group by file month
  date       - Group by full date
  size       - Group by file size ranges

For more information: https://github.com/lookinalx/fiona
`)
}

func PrintShortUsage() {
	fmt.Fprintf(os.Stderr, `
Usage:
	fiona sort [OPTIONS]
	fiona undo [OPTIONS])

Use fiona -h or --help for more information.
`)
}
