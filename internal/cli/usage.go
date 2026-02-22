package cli

import (
	"flag"
	"fmt"
	"os"
)

func init() {
	flag.Usage = PrintUsage
}

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

  -a, --action <string>
        How to handle files: copy or move (default: copy)

  --on-conflict <string>
        How to handle file conflicts: replace, skip, or rename (default: replace)

  -n, --dry-run
        Preview changes without executing (default: true)

  --force <yes|N>
        Force execution without confirmation (default: N)

  -w, --workers <int>
        Number of workers for the current run (default: max availible)

  --log <string>
		Log directory to write log files to (default: current directory)

OPTIONS UNDO:
	--log <string>
		Log file to read logs from, must be json (default: current directory, file name fiona_logs.json)
	  -w, --workers <int>
        Number of workers for the current run (default: max availible)

EXAMPLES:
  # Preview sorting by file extension
  fiona sort -c extension

  # Sort photos by year, then month, and move them
  fiona sort -c year -t month -a move -s ~/Photos

  # Copy documents by type with rename on conflict
  fiona sort -c mimetype --on-conflict rename -s ~/Documents -d ~/Organized

  # Execute immediately without dry-run
  fiona sort -c extension -n=false --force yes

  # Undo last fiona run
  fiona undo TBD

AVAILABLE CRITERIA:
  mimetype   - Group by MIME type (images, videos, documents, etc.)
  extension  - Group by file extension (.jpg, .pdf, .txt, etc.)
  year       - Group by file year
  month      - Group by file month
  date       - Group by full date
  size       - Group by file size ranges

For more information: https://github.com/lookinALX/FIONA
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
