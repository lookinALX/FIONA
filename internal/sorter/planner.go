package sorter

import (
	"errors"
	"fmt"
	"io"
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

const separator = "================================================"

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

type PlanStats struct {
	Files    int // operations that change the disk
	Size     int64
	Renamed  int
	Replaced int
	Skipped  int
	Noop     int
	Rejected int
}

type Plan struct {
	Actions       []types.Action
	Mkdirs        []string // directories that do not exist yet, parents first
	Skipped       []scanner.SkippedEntry
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
	rules    []rules.Rule
	reserved map[string]struct{} // lower-cased targets taken by earlier operations
	sources  map[string]struct{} // lower-cased paths of all files of this plan
	dirs     map[string]bool     // directory path -> exists on disk
	nextIdx  map[string]int      // lower-cased target -> next suffix to try on rename
}

// NewPlan resolves every conflict in a single thread, so that each operation
// gets a unique target path and workers have nothing to race for.
func NewPlan(files []*scanner.FileInfo, rls []rules.Rule, skipped []scanner.SkippedEntry, params PlanParams) *Plan {
	b := planBuilder{
		plan: &Plan{
			Actions:       []types.Action{},
			Skipped:       skipped,
			BaseDirSource: params.Source,
			BaseDirDest:   params.Dest,
			FileAction:    params.FileAction,
			OnConflict:    params.OnConflict,
			RunID:         params.RunID,
			DirCounts:     map[string]int{},
			DirSizes:      map[string]int64{},
		},
		rules:    rls,
		reserved: map[string]struct{}{},
		sources:  map[string]struct{}{},
		dirs:     map[string]bool{},
		nextIdx:  map[string]int{},
	}

	sorted := make([]*scanner.FileInfo, len(files))
	copy(sorted, files)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	for _, fi := range sorted {
		b.sources[pathKey(fi.Path)] = struct{}{}
	}

	for seq, fi := range sorted {
		action := b.planFile(fi)
		action.Seq = seq
		b.plan.Actions = append(b.plan.Actions, action)

		if action.Executable() {
			b.plan.DirCounts[action.DestDir]++
			b.plan.DirSizes[action.DestDir] += action.Size
		}
	}

	return b.plan
}

func (b *planBuilder) planFile(fi *scanner.FileInfo) types.Action {
	dest := b.plan.BaseDirDest
	action := types.Action{
		SourcePath: fi.Path,
		Size:       fi.Size,
		ModTime:    fi.ModTime,
	}

	components := make([]string, 0, len(b.rules))
	for _, rule := range b.rules {
		components = append(components, rules.SanitizeComponent(rule.GetDestination(fi)))
	}
	relDir := filepath.Join(components...)
	action.DestDir = relDir

	target := filepath.Join(dest, relDir, filepath.Base(fi.Path))
	if !filepath.IsLocal(relDir) || !isWithin(dest, target) {
		return reject(action, noteUnsafePath)
	}
	action.TargetPath = target

	missingDirs, err := b.missingDirs(relDir)
	if err != nil {
		return reject(action, err.Error())
	}

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
			trashRel := filepath.Join(journal.StateDir, journal.TrashDir, b.plan.RunID, relDir)
			trashDirs, err := b.missingDirs(trashRel)
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
		b.dirs[dir] = true
		b.plan.Mkdirs = append(b.plan.Mkdirs, dir)
	}

	action.Kind = types.OpMove
	if b.plan.FileAction == string(types.OpCopy) {
		action.Kind = types.OpCopy
	}

	return action
}

// missingDirs walks dest/relDir component by component and returns the
// directories that are not on disk yet. An existing component that is not a
// real directory (a file or a symlink) makes the path unusable.
func (b *planBuilder) missingDirs(relDir string) ([]string, error) {
	var missing []string

	dir := b.plan.BaseDirDest
	for _, component := range strings.Split(relDir, string(filepath.Separator)) {
		dir = filepath.Join(dir, component)

		exists, known := b.dirs[dir]
		if !known {
			info, err := os.Lstat(dir)
			switch {
			case err == nil && !info.IsDir():
				return nil, errors.New(noteNotADirectory)
			case err == nil:
				exists = true
			case !errors.Is(err, fs.ErrNotExist):
				return nil, pathError(err)
			}
			if exists {
				b.dirs[dir] = true
			}
		}

		if !exists {
			missing = append(missing, dir)
		}
	}

	return missing, nil
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

func (p *Plan) Stats() PlanStats {
	var stats PlanStats
	for _, action := range p.Actions {
		switch action.Kind {
		case types.OpSkip:
			stats.Skipped++
		case types.OpNoop:
			stats.Noop++
		case types.OpReject:
			stats.Rejected++
		default:
			stats.Files++
			stats.Size += action.Size
			switch action.Conflict {
			case types.ConflictRenamed:
				stats.Renamed++
			case types.ConflictReplaced:
				stats.Replaced++
			}
		}
	}
	return stats
}

// Summary groups planned operations by destination directory.
func (p *Plan) Summary() map[string][]types.Action {
	byDir := make(map[string][]types.Action)
	for _, action := range p.Actions {
		if action.Executable() {
			byDir[action.DestDir] = append(byDir[action.DestDir], action)
		}
	}
	for dir := range byDir {
		actions := byDir[dir]
		sort.SliceStable(actions, func(i, j int) bool { return actions[i].TargetPath < actions[j].TargetPath })
	}
	return byDir
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func pluralize(count int) string {
	if count == 1 {
		return "file"
	}
	return "files"
}

func primaryGroups(dirCounts map[string]int) map[string][]string {
	groups := make(map[string][]string)

	for dir := range dirCounts {
		parts := strings.SplitN(dir, string(filepath.Separator), 2)
		primary := parts[0]
		secondary := ""
		if len(parts) > 1 {
			secondary = parts[1]
		}
		groups[primary] = append(groups[primary], secondary)
	}

	for k := range groups {
		sort.Strings(groups[k])
	}

	return groups
}

func (p *Plan) Print(w io.Writer, longPrint bool) {
	stats := p.Stats()

	fmt.Fprintln(w, separator)
	fmt.Fprintln(w, "                 SORT PLAN")
	fmt.Fprintln(w, separator)
	fmt.Fprintf(w, "\n  Action:      %s\n", p.FileAction)
	fmt.Fprintf(w, "  Source:      %s\n", p.BaseDirSource)
	fmt.Fprintf(w, "  Destination: %s\n", p.BaseDirDest)
	fmt.Fprintf(w, "  On conflict: %s\n\n", p.OnConflict)
	fmt.Fprintln(w, separator)

	if stats.Files == 0 {
		fmt.Fprintf(w, "\n  No files to process.\n\n")
	} else {
		fmt.Fprintf(w, "\n  Total: %d %s, %s\n\n", stats.Files, pluralize(stats.Files), formatSize(stats.Size))
		fmt.Fprintln(w, separator)
		fmt.Fprintln(w)
		p.printTree(w, longPrint)
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, separator)
	fmt.Fprintf(w, "  Renamed: %d\n", stats.Renamed)
	fmt.Fprintf(w, "  Replaced: %d\n", stats.Replaced)
	fmt.Fprintf(w, "  Skipped: %d\n", stats.Skipped)
	fmt.Fprintf(w, "  Already in place: %d\n", stats.Noop)
	fmt.Fprintf(w, "  Rejected: %d\n", stats.Rejected)
	for _, action := range p.Actions {
		if action.Kind == types.OpReject {
			fmt.Fprintf(w, "    %s: %s\n", action.SourcePath, action.Note)
		}
	}
	fmt.Fprintf(w, "  Skipped by scanner: %s\n", formatSkipped(p.Skipped))
	fmt.Fprintln(w, separator)
}

func (p *Plan) printTree(w io.Writer, longPrint bool) {
	groups := primaryGroups(p.DirCounts)

	primaries := make([]string, 0, len(groups))
	for k := range groups {
		primaries = append(primaries, k)
	}
	sort.Strings(primaries)

	var summary map[string][]types.Action
	if longPrint {
		summary = p.Summary()
	}

	for i, primary := range primaries {
		secondaries := groups[primary]
		hasSecondaries := !(len(secondaries) == 1 && secondaries[0] == "")

		if !hasSecondaries {
			p.printLeaf(w, primary, summary)
		} else {
			p.printBranch(w, primary, secondaries, summary)
		}

		if i != len(primaries)-1 {
			fmt.Fprintln(w)
		}
	}
}

func (p *Plan) printLeaf(w io.Writer, primary string, summary map[string][]types.Action) {
	count := p.DirCounts[primary]

	fmt.Fprintf(w, "  📁 %s/  (%d %s, %s)\n",
		primary, count, pluralize(count), formatSize(p.DirSizes[primary]))

	printFiles(w, "  ", summary[primary])
}

func (p *Plan) printBranch(w io.Writer, primary string, secondaries []string, summary map[string][]types.Action) {
	var primaryTotal int
	var primarySize int64
	for _, sec := range secondaries {
		fd := filepath.Join(primary, sec)
		primaryTotal += p.DirCounts[fd]
		primarySize += p.DirSizes[fd]
	}

	fmt.Fprintf(w, "  📁 %s/  (%d %s, %s)\n",
		primary, primaryTotal, pluralize(primaryTotal), formatSize(primarySize))

	for i, sec := range secondaries {
		isLast := i == len(secondaries)-1
		fullDir := filepath.Join(primary, sec)
		count := p.DirCounts[fullDir]

		secConn := "  ├── "
		childPrefix := "  │   "
		if isLast {
			secConn = "  └── "
			childPrefix = "      "
		}

		fmt.Fprintf(w, "%s📂 %s/  (%d %s, %s)\n",
			secConn, sec, count, pluralize(count), formatSize(p.DirSizes[fullDir]))

		printFiles(w, childPrefix, summary[fullDir])
	}
}

func printFiles(w io.Writer, prefix string, actions []types.Action) {
	for i, action := range actions {
		conn := prefix + "├── "
		if i == len(actions)-1 {
			conn = prefix + "└── "
		}

		name := filepath.Base(action.TargetPath)
		switch action.Conflict {
		case types.ConflictRenamed:
			name += fmt.Sprintf("  (renamed from %s)", filepath.Base(action.SourcePath))
		case types.ConflictReplaced:
			name += "  (replaces existing file)"
		}

		fmt.Fprintf(w, "%s%s\n", conn, name)
	}
}

// formatSkipped renders scanner skips like "3 (2 symlinks, 1 no access)".
func formatSkipped(skipped []scanner.SkippedEntry) string {
	if len(skipped) == 0 {
		return "0"
	}

	counts := make(map[string]int)
	for _, entry := range skipped {
		counts[entry.Reason]++
	}

	reasons := make([]string, 0, len(counts))
	for reason := range counts {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)

	parts := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		label := reason
		if counts[reason] != 1 && reason != scanner.ReasonNoAccess {
			label += "s"
		}
		parts = append(parts, fmt.Sprintf("%d %s", counts[reason], label))
	}

	return fmt.Sprintf("%d (%s)", len(skipped), strings.Join(parts, ", "))
}
