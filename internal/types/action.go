package types

import (
	"path/filepath"

	"github.com/lookinALX/FIONA/internal/rules"
	"github.com/lookinALX/FIONA/internal/scanner"
)

type Action struct {
	SourcePath string
	DestDir    string
	DestPath   string
	FileSize   int64
}

func NewAction(fi *scanner.FileInfo, rules []rules.Rule, destFullPath string) Action {
	destDirPrimary := rules[0].GetDestination(fi)
	destDirSecondary := ""
	if len(rules) > 1 {
		destDirSecondary = rules[1].GetDestination(fi)
	}
	destPath := filepath.Join(destFullPath, destDirPrimary, destDirSecondary)
	return Action{
		SourcePath: fi.Path,
		DestDir:    filepath.Join(destDirPrimary, destDirSecondary),
		DestPath:   destPath,
		FileSize:   fi.Size,
	}
}

type UndoAction struct {
	SourcePath       string
	DestPath         string
	LastActionStatus string
}
