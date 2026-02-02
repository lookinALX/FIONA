package sorter

import (
	"FIONA/internal/cli"
	"FIONA/internal/rules"
	"FIONA/internal/scanner"
	"fmt"
	"path/filepath"
)

type Action struct {
	SourcePath string
	DestDir    string
	DestPath   string
	FileSize   int64
}

type Plan struct {
	Actions       []Action
	BaseDirSource string
	BaseDirDest   string
	fileAction    string
	DirCounts     map[string]int
	DirSizes      map[string]int64
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

func NewPlan(opt *cli.Opts) Plan {
	return Plan{
		Actions:       []Action{},
		BaseDirDest:   opt.Dest,
		BaseDirSource: opt.Source,
		fileAction:    opt.Action,
		DirCounts:     map[string]int{},
		DirSizes:      map[string]int64{},
	}
}

func (p *Plan) AddAction(action Action) {
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

func (p *Plan) Print(LongPrint bool) {
	fmt.Println("##################### DRY-RUN PLAN ####################")
	fmt.Println()
	fmt.Printf("Sort and %s files from directory %s to directory %s\n", p.fileAction, p.BaseDirSource, p.BaseDirDest)
	fmt.Println()
	fmt.Println("The following directories will be created or used if they already exist:")
	for key, value := range p.DirCounts {
		fmt.Printf("  - %s --> files: %d ; final size: %d\n", key, value, p.DirSizes[key])
	}
	fmt.Println()
	if LongPrint {
		dirForFile := p.Summary()
		fmt.Println("The following actions are planed and will be executed:")
		fmt.Println()
		for key, value := range dirForFile {
			fmt.Printf("Folder: %s\n", key)
			for _, file := range value {
				fmt.Printf("  - %s\n", file)
			}
		}
	}
}
