package sorter

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lookinalx/fiona/internal/journal"
	"github.com/lookinalx/fiona/internal/rules"
	"github.com/lookinalx/fiona/internal/scanner"
	"github.com/lookinalx/fiona/internal/types"
)

const (
	noteAlreadyExists = "already exists"
	noteUnsafePath    = "unsafe destination path"
	noteNotADirectory = "destination path is not a directory"
)

type PlanParams struct {
	Source     string // absolute
	Dest       string // absolute
	FileAction string // "move" or "copy"
	OnConflict string
	RunID      string
}

// Router decides the destination folders of a file; false leaves it in place.
type Router interface {
	Route(fi *scanner.FileInfo) ([]string, bool)
}

type PlanStats struct {
	Files     int // operations that change the disk
	Size      int64
	Renamed   int
	Replaced  int
	Skipped   int
	Noop      int
	Rejected  int
	Unmatched int // files no rule matched
}

type Plan struct {
	Actions       []types.Action
	Mkdirs        []string // directories that do not exist yet, parents first
	Skipped       []scanner.SkippedEntry
	Kept          []string // folders the scanner left as is
	Unmatched     []string // files no rule matched, they stay in place
	BaseDirSource string
	BaseDirDest   string
	FileAction    string
	OnConflict    string
	RunID         string
	DirCounts     map[string]int
	DirSizes      map[string]int64
}

type planBuilder struct {
	plan     *Plan
	router   Router
	reserved map[string]struct{}      // lower-cased targets taken by earlier operations
	sources  map[string]struct{}      // lower-cased paths of all files of this plan
	dirs     map[string]dirState      // lower-cased directory path -> how it is spelled and whether it exists
	listings map[string][]os.DirEntry // directory -> its entries on disk
	nextIdx  map[string]int           // lower-cased target -> next suffix to try on rename
}

type dirState struct {
	path   string
	exists bool
}

// NewPlan resolves every conflict in a single thread, so that each operation
// gets a unique target path and workers have nothing to race for.
func NewPlan(scanned *scanner.Result, router Router, params PlanParams) *Plan {
	b := planBuilder{
		plan: &Plan{
			Actions:       []types.Action{},
			Skipped:       scanned.Skipped,
			Kept:          scanned.Kept,
			BaseDirSource: params.Source,
			BaseDirDest:   params.Dest,
			FileAction:    params.FileAction,
			OnConflict:    params.OnConflict,
			RunID:         params.RunID,
			DirCounts:     map[string]int{},
			DirSizes:      map[string]int64{},
		},
		router:   router,
		reserved: map[string]struct{}{},
		sources:  map[string]struct{}{},
		dirs:     map[string]dirState{},
		listings: map[string][]os.DirEntry{},
		nextIdx:  map[string]int{},
	}

	sorted := make([]*scanner.FileInfo, len(scanned.Files))
	copy(sorted, scanned.Files)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	for _, fi := range sorted {
		b.sources[pathKey(fi.Path)] = struct{}{}
	}

	seq := 0
	for _, fi := range sorted {
		components, ok := router.Route(fi)
		if !ok {
			b.plan.Unmatched = append(b.plan.Unmatched, fi.Path)
			continue
		}

		action := b.planFile(fi, components)
		action.Seq = seq
		seq++
		b.plan.Actions = append(b.plan.Actions, action)

		if action.Executable() {
			b.plan.DirCounts[action.DestDir]++
			b.plan.DirSizes[action.DestDir] += action.Size
		}
	}

	return b.plan
}

func (b *planBuilder) planFile(fi *scanner.FileInfo, components []string) types.Action {
	dest := b.plan.BaseDirDest
	action := types.Action{
		SourcePath: fi.Path,
		Size:       fi.Size,
		ModTime:    fi.ModTime,
	}

	for i, component := range components {
		components[i] = rules.SanitizeComponent(component)
	}
	action.DestDir = filepath.Join(components...)

	if !filepath.IsLocal(action.DestDir) || !isWithin(dest, filepath.Join(dest, action.DestDir, filepath.Base(fi.Path))) {
		return reject(action, noteUnsafePath)
	}

	relDir, missingDirs, err := b.resolveDir(components)
	if err != nil {
		return reject(action, err.Error())
	}
	action.DestDir = relDir

	target := filepath.Join(dest, relDir, filepath.Base(fi.Path))
	if !isWithin(dest, target) {
		return reject(action, noteUnsafePath)
	}
	action.TargetPath = target

	if isSameFile(fi.Path, target) {
		action.Kind = types.OpNoop
		return action
	}

	occupant, reserved, err := b.lookup(target)
	if err != nil {
		return reject(action, err.Error())
	}

	if occupant != nil || reserved {
		strategy := b.plan.OnConflict
		if strategy == types.OnConflictReplace {
			// only a regular file that existed before the run and is not itself
			// being sorted by this plan may be displaced
			_, isSource := b.sources[pathKey(target)]
			if reserved || isSource || !occupant.Mode().IsRegular() {
				strategy = types.OnConflictRename
			}
		}

		switch strategy {
		case types.OnConflictSkip:
			action.Kind = types.OpSkip
			action.Note = noteAlreadyExists
			return action

		case types.OnConflictReplace:
			trashComponents := append([]string{journal.StateDir, journal.TrashDir, b.plan.RunID}, strings.Split(relDir, string(filepath.Separator))...)
			trashRel, trashDirs, err := b.resolveDir(trashComponents)
			if err != nil {
				return reject(action, err.Error())
			}
			action.Conflict = types.ConflictReplaced
			action.TrashPath = filepath.Join(dest, trashRel, filepath.Base(target))
			missingDirs = append(missingDirs, trashDirs...)

		default:
			target, err = b.freeName(target)
			if err != nil {
				return reject(action, err.Error())
			}
			action.Conflict = types.ConflictRenamed
			action.TargetPath = target
		}
	}

	b.reserved[pathKey(target)] = struct{}{}
	for _, dir := range missingDirs {
		b.dirs[pathKey(dir)] = dirState{path: dir, exists: false}
		b.plan.Mkdirs = append(b.plan.Mkdirs, dir)
	}

	action.Kind = types.OpMove
	if b.plan.FileAction == string(types.OpCopy) {
		action.Kind = types.OpCopy
	}

	return action
}

// resolveDir walks dest component by component and returns the directory that
// will really be used, relative to dest, and the directories not on disk yet.
// A folder that differs only in case, on disk or planned earlier, is reused:
// "images" goes into an existing "Images". An existing component that is not
// a real directory (a file or a symlink) makes the path unusable.
func (b *planBuilder) resolveDir(components []string) (string, []string, error) {
	var missing []string

	dir := b.plan.BaseDirDest
	parentExists := true
	rel := make([]string, 0, len(components))

	for _, component := range components {
		key := pathKey(filepath.Join(dir, component))

		state, known := b.dirs[key]
		if !known {
			state = dirState{path: filepath.Join(dir, component)}
			if parentExists {
				name, isDir, found, err := b.findEntry(dir, component)
				switch {
				case err != nil:
					return "", nil, pathError(err)
				case found && !isDir:
					return "", nil, errors.New(noteNotADirectory)
				case found:
					state = dirState{path: filepath.Join(dir, name), exists: true}
					b.dirs[key] = state
				}
			}
		}

		if !state.exists {
			if _, planned := b.dirs[key]; !planned {
				missing = append(missing, state.path)
			}
		}

		dir = state.path
		parentExists = state.exists
		rel = append(rel, filepath.Base(state.path))
	}

	return filepath.Join(rel...), missing, nil
}

// findEntry looks for name in dir: the exact spelling first, then any case.
func (b *planBuilder) findEntry(dir, name string) (actual string, isDir, found bool, err error) {
	entries, listed := b.listings[dir]
	if !listed {
		entries, err = os.ReadDir(dir)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", false, false, err
		}
		b.listings[dir] = entries
	}

	var folded fs.DirEntry
	for _, entry := range entries {
		if entry.Name() == name {
			return name, entry.IsDir(), true, nil
		}
		if folded == nil && strings.EqualFold(entry.Name(), name) {
			folded = entry
		}
	}
	if folded != nil {
		return folded.Name(), folded.IsDir(), true, nil
	}
	return "", false, false, nil
}

// lookup reports what occupies path: a file on disk, a reservation made by an
// earlier operation of this plan, or nothing.
func (b *planBuilder) lookup(path string) (occupant os.FileInfo, reserved bool, err error) {
	if _, reserved = b.reserved[pathKey(path)]; reserved {
		return nil, true, nil
	}

	// Lstat, so that a dangling symlink counts as occupied too
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		return info, false, nil
	case errors.Is(err, fs.ErrNotExist):
		return nil, false, nil
	default:
		return nil, false, pathError(err)
	}
}

// freeName finds the first free name_1.ext, name_2.ext, ... next to path.
func (b *planBuilder) freeName(path string) (string, error) {
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	if stem == "" {
		stem, ext = name, ""
	}

	key := pathKey(path)
	for i := max(b.nextIdx[key], 1); ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", stem, i, ext))

		occupant, reserved, err := b.lookup(candidate)
		if err != nil {
			return "", err
		}
		if occupant == nil && !reserved {
			b.nextIdx[key] = i + 1
			return candidate, nil
		}
	}
}

func reject(action types.Action, note string) types.Action {
	action.Kind = types.OpReject
	action.Note = note
	return action
}

// pathKey makes names that differ only by case collide on any file system.
func pathKey(path string) string {
	return strings.ToLower(path)
}

func pathError(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}

func isWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isSameFile(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	infoA, err := os.Lstat(a)
	if err != nil {
		return false
	}
	infoB, err := os.Lstat(b)
	if err != nil {
		return false
	}
	return os.SameFile(infoA, infoB)
}
