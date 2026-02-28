package rules

import "github.com/lookinalx/fiona/internal/scanner"

type ByMLRule struct {
	IsPrimary bool
}

func (r *ByMLRule) GetDestination(fi *scanner.FileInfo) string {
	return fi.Tag
}
