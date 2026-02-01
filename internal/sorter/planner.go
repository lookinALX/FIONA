package sorter

import (
	"FIONA/internal/rules"
	"FIONA/internal/scanner"
	"path/filepath"
	"strings"
)

type Action struct {
	SourcePath string
	DestDir    string
	DestPath   string
	FileSize   int64
}

type Plan struct {
	Actions   []Action
	BaseDir   string
	DirCounts map[string]int
	DirSizes  map[string]int64
}

func NewAction(fi *scanner.FileInfo, rule rules.Rule, destFullPath string) Action {
	destDir := rule.GetDestination(fi)
	destPath := filepath.Join(destFullPath, destDir)
	return Action{
		SourcePath: fi.Path,
		DestDir:    destDir,
		DestPath:   destPath,
		FileSize:   fi.Size,
	}
}

func NewPlan() Plan {
	return Plan{}
}

func (p *Plan) AddAction(action Action) {
	p.defineBaseDir(&action)
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

func (p *Plan) Summary() string {
	return ""
}

func (p *Plan) Print() {

}

func (p *Plan) defineBaseDir(newAction *Action) {
	actionSourceDir := filepath.Dir(newAction.SourcePath)
	if p.BaseDir == "" {
		p.BaseDir = actionSourceDir
		return
	}

	actionSourceSeparatorsCount := countSeparators(actionSourceDir)
	baseDirSeparatorsCount := countSeparators(p.BaseDir)

	if actionSourceSeparatorsCount < baseDirSeparatorsCount {
		p.BaseDir = actionSourceDir
	}

	if actionSourceSeparatorsCount == baseDirSeparatorsCount {
		p.BaseDir = getFirstCommonDir(actionSourceDir, p.BaseDir)
	}
}

func countSeparators(path string) int {
	return strings.Count(filepath.Clean(path), string(filepath.Separator))
}

func getFirstCommonDir(path1, path2 string) string {
	path1 = filepath.Clean(path1)
	path2 = filepath.Clean(path2)
	if path1 == path2 {
		return path1
	}

	if filepath.Dir(path1) == path1 || filepath.Dir(path2) == path2 {
		return "" // no common root
	}

	return getFirstCommonDir(filepath.Dir(path1), filepath.Dir(path2))
}
