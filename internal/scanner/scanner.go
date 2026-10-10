package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	ReasonSymlink    = "symlink"
	ReasonNotRegular = "special file"
	ReasonNoAccess   = "no access"
)

// stateDir holds FIONA journals and trash and is never scanned.
const stateDir = ".fiona"

var opaqueSuffixes = []string{".app", ".bundle", ".framework", ".photoslibrary", ".xcodeproj"}

var projectMarkers = map[string]struct{}{
	".git": {}, ".hg": {}, ".svn": {},
	"node_modules": {}, "go.mod": {}, "package.json": {},
}

type Options struct {
	Recursive     bool     // false by default
	IncludeHidden bool     // false by default
	Exclude       []string // absolute paths: dest, if it is inside source
}

type SkippedEntry struct {
	Path   string
	Reason string
}

type Result struct {
	Files   []*FileInfo
	Skipped []SkippedEntry
}

type walker struct {
	opt          Options
	excludeInfos []os.FileInfo
	res          *Result
}

// Scan collects regular files under root. Only an error reading root itself is fatal.
func Scan(root string, opt Options) (*Result, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("scanner: can not scan directory: %w", err)
	}

	w := walker{opt: opt, res: &Result{}}
	for _, path := range opt.Exclude {
		if info, err := os.Stat(path); err == nil {
			w.excludeInfos = append(w.excludeInfos, info)
		}
	}

	w.walk(root, entries)

	return w.res, nil
}

func (w *walker) walk(dir string, entries []fs.DirEntry) {
	for _, d := range entries {
		name := d.Name()
		path := filepath.Join(dir, name)

		if !w.opt.IncludeHidden && strings.HasPrefix(name, ".") {
			continue
		}

		typ := d.Type()
		switch {
		case typ.IsDir():
			w.walkDir(path, d)
		case typ&fs.ModeSymlink != 0:
			w.skip(path, ReasonSymlink)
		case !typ.IsRegular():
			w.skip(path, ReasonNotRegular)
		default:
			info, err := d.Info()
			if err != nil {
				w.skip(path, ReasonNoAccess)
				continue
			}
			if !info.Mode().IsRegular() {
				w.skip(path, ReasonNotRegular)
				continue
			}
			w.res.Files = append(w.res.Files, newFileInfo(path, info))
		}
	}
}

func (w *walker) walkDir(path string, d fs.DirEntry) {
	name := d.Name()
	if !w.opt.Recursive || name == stateDir || hasOpaqueSuffix(name) || w.excluded(path, d) {
		return
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		w.skip(path, ReasonNoAccess)
		return
	}

	for _, e := range entries {
		if _, isMarker := projectMarkers[e.Name()]; isMarker {
			return
		}
	}

	w.walk(path, entries)
}

func (w *walker) excluded(path string, d fs.DirEntry) bool {
	if len(w.opt.Exclude) == 0 {
		return false
	}
	for _, ex := range w.opt.Exclude {
		if path == ex {
			return true
		}
	}
	// the same directory may be spelled differently, e.g. on a case-insensitive file system
	info, err := d.Info()
	if err != nil {
		return false
	}
	for _, exInfo := range w.excludeInfos {
		if os.SameFile(info, exInfo) {
			return true
		}
	}
	return false
}

func (w *walker) skip(path, reason string) {
	w.res.Skipped = append(w.res.Skipped, SkippedEntry{Path: path, Reason: reason})
}

func hasOpaqueSuffix(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range opaqueSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}
