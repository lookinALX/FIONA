package sorter

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/lookinalx/fiona/internal/scanner"
	"github.com/lookinalx/fiona/internal/types"
)

func (p *Plan) Stats() PlanStats {
	stats := PlanStats{Unmatched: len(p.Unmatched)}
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

// Folder is one line of the plan overview.
type Folder struct {
	Name     string
	Files    int
	Size     int64
	Exists   bool       // the folder is already on disk, files join its content
	Children []Folder   // the second level; deeper levels are counted into it
	Exts     []ExtCount // most frequent extensions first
}

type ExtCount struct {
	Ext   string // without the dot, empty for files without an extension
	Count int
}

// Folders summarises the plan two levels deep. Top-level folders that are all
// categories keep the category order, anything else is sorted by name.
func (p *Plan) Folders() []Folder {
	newDirs := make(map[string]struct{}, len(p.Mkdirs))
	for _, dir := range p.Mkdirs {
		newDirs[pathKey(dir)] = struct{}{}
	}
	exists := func(rel string) bool {
		_, isNew := newDirs[pathKey(filepath.Join(p.BaseDirDest, rel))]
		return !isNew
	}

	tops := map[string]*Folder{}
	children := map[string]map[string]*Folder{}
	exts := map[string]map[string]int{}

	for _, action := range p.Actions {
		if !action.Executable() {
			continue
		}
		parts := strings.SplitN(action.DestDir, string(filepath.Separator), 3)

		top, ok := tops[parts[0]]
		if !ok {
			top = &Folder{Name: parts[0], Exists: exists(parts[0])}
			tops[parts[0]] = top
			children[parts[0]] = map[string]*Folder{}
			exts[parts[0]] = map[string]int{}
		}
		top.Files++
		top.Size += action.Size
		exts[parts[0]][extOf(action.SourcePath)]++

		if len(parts) > 1 {
			child, ok := children[parts[0]][parts[1]]
			if !ok {
				child = &Folder{Name: parts[1], Exists: exists(filepath.Join(parts[0], parts[1]))}
				children[parts[0]][parts[1]] = child
			}
			child.Files++
			child.Size += action.Size
		}
	}

	folders := make([]Folder, 0, len(tops))
	for name, top := range tops {
		for _, child := range children[name] {
			top.Children = append(top.Children, *child)
		}
		sort.Slice(top.Children, func(i, j int) bool { return top.Children[i].Name < top.Children[j].Name })

		for ext, count := range exts[name] {
			top.Exts = append(top.Exts, ExtCount{Ext: ext, Count: count})
		}
		sort.Slice(top.Exts, func(i, j int) bool {
			if top.Exts[i].Count != top.Exts[j].Count {
				return top.Exts[i].Count > top.Exts[j].Count
			}
			return top.Exts[i].Ext < top.Exts[j].Ext
		})

		folders = append(folders, *top)
	}

	sortFolders(folders)
	return folders
}

func sortFolders(folders []Folder) {
	order := make(map[string]int, len(scanner.Categories))
	for i, category := range scanner.Categories {
		order[category] = i
	}

	allCategories := true
	for _, f := range folders {
		if _, ok := order[strings.ToLower(f.Name)]; !ok {
			allCategories = false
			break
		}
	}

	sort.Slice(folders, func(i, j int) bool {
		if allCategories {
			return order[strings.ToLower(folders[i].Name)] < order[strings.ToLower(folders[j].Name)]
		}
		return folders[i].Name < folders[j].Name
	})
}

func extOf(path string) string {
	ext := filepath.Ext(filepath.Base(path))
	if ext == filepath.Base(path) {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}
