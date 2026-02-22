package sorter

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lookinALX/FIONA/internal/cli"
	"github.com/lookinALX/FIONA/internal/journal"
	"github.com/lookinALX/FIONA/internal/types"
)

type Plan struct {
	Actions       []types.Action
	BaseDirSource string
	BaseDirDest   string
	FileAction    string
	DirCounts     map[string]int
	DirSizes      map[string]int64
}

func NewPlan(opt *cli.Opts) Plan {
	return Plan{
		Actions:       []types.Action{},
		BaseDirDest:   opt.DestPath,
		BaseDirSource: opt.SourcePath,
		FileAction:    opt.FileAction,
		DirCounts:     map[string]int{},
		DirSizes:      map[string]int64{},
	}
}

type UndoPlan struct {
	UndoActions []types.UndoAction
	FileAction  string
}

func NewUndoPlan(jrn *journal.Journal) *UndoPlan {
	plan := UndoPlan{
		UndoActions: []types.UndoAction{},
		FileAction:  jrn.FileAction,
	}

	for _, entry := range jrn.Logs {
		if entry.Status == "failed" {
			continue
		}
		plan.UndoActions = append(
			plan.UndoActions,
			types.UndoAction{
				SourcePath:       filepath.Join(entry.DestPath, filepath.Base(entry.SourcePath)),
				DestPath:         entry.SourcePath,
				LastActionStatus: entry.Status},
		)
	}

	return &plan
}

func (p *Plan) AddAction(action types.Action) {
	p.Actions = append(p.Actions, action)

	if p.DirCounts == nil {
		p.DirCounts = make(map[string]int)
	}
	if p.DirSizes == nil {
		p.DirSizes = make(map[string]int64)
	}

	p.DirCounts[action.DestDir]++
	p.DirSizes[action.DestDir] += action.FileSize
}

func (p *Plan) Summary() map[string][]string {
	dirForFile := make(map[string][]string)
	for _, action := range p.Actions {
		dirForFile[action.DestPath] = append(dirForFile[action.DestPath], action.SourcePath)
	}
	return dirForFile
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

func (p *Plan) Print(longPrint bool) {
	const separator = "================================================"

	fmt.Println(separator)
	fmt.Println("              DRY-RUN PLAN")
	fmt.Println(separator)
	fmt.Printf("\n  FileAction:      %s\n", p.FileAction)
	fmt.Printf("  Source:      %s\n", p.BaseDirSource)
	fmt.Printf("  Destination: %s\n\n", p.BaseDirDest)
	fmt.Println(separator)

	// ── empty plan ───────────────────────────────────────────────────────────
	if len(p.Actions) == 0 {
		fmt.Printf("\n  No files to process.\n")
		fmt.Println(separator)
		return
	}

	// ── totals ───────────────────────────────────────────────────────────────
	var totalFiles int
	var totalSize int64
	for dir, count := range p.DirCounts {
		totalFiles += count
		totalSize += p.DirSizes[dir]
	}

	fmt.Printf("\n  Total: %d %s, %s\n\n", totalFiles, pluralize(totalFiles), formatSize(totalSize))
	fmt.Println(separator)
	fmt.Println()

	// ── group and sort ─────────────────────────────────────────────────────────
	groups := primaryGroups(p.DirCounts)

	primaries := make([]string, 0, len(groups))
	for k := range groups {
		primaries = append(primaries, k)
	}
	sort.Strings(primaries)

	// summary for longPrint
	var summary map[string][]string
	if longPrint {
		summary = p.Summary()
		for k := range summary {
			sort.Strings(summary[k])
		}
	}

	// ── tree ─────────────────────────────────────────────────────────────────
	for i, primary := range primaries {
		isLastPrimary := i == len(primaries)-1
		secondaries := groups[primary]
		hasSecondaries := !(len(secondaries) == 1 && secondaries[0] == "")

		if !hasSecondaries {
			p.printLeaf(primary, longPrint, summary)
		} else {
			p.printBranch(primary, secondaries, longPrint, summary)
		}

		if !isLastPrimary {
			fmt.Println()
		}
	}

	fmt.Println()
	fmt.Println(separator)
}

func (p *Plan) printLeaf(primary string, longPrint bool, summary map[string][]string) {
	count := p.DirCounts[primary]

	fmt.Printf("  📁 %s/  (%d %s, %s)\n",
		primary, count, pluralize(count), formatSize(p.DirSizes[primary]))

	if !longPrint {
		return
	}

	destPath := filepath.Join(p.BaseDirDest, primary)
	files := summary[destPath]

	for i, file := range files {
		conn := "  ├── "
		if i == len(files)-1 {
			conn = "  └── "
		}
		fmt.Printf("%s%s\n", conn, filepath.Base(file))
	}
}

func (p *Plan) printBranch(primary string, secondaries []string, longPrint bool, summary map[string][]string) {
	var primaryTotal int
	var primarySize int64
	for _, sec := range secondaries {
		fd := filepath.Join(primary, sec)
		primaryTotal += p.DirCounts[fd]
		primarySize += p.DirSizes[fd]
	}

	fmt.Printf("  📁 %s/  (%d %s, %s)\n",
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

		fmt.Printf("%s📂 %s/  (%d %s, %s)\n",
			secConn, sec, count, pluralize(count), formatSize(p.DirSizes[fullDir]))

		if !longPrint {
			continue
		}

		destPath := filepath.Join(p.BaseDirDest, fullDir)
		files := summary[destPath]

		for j, file := range files {
			fileConn := childPrefix + "├── "
			if j == len(files)-1 {
				fileConn = childPrefix + "└── "
			}
			fmt.Printf("%s%s\n", fileConn, filepath.Base(file))
		}
	}
}
