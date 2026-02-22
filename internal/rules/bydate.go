package rules

import "github.com/lookinalx/fiona/internal/scanner"

type ByDateRule struct {
	IsPrimary bool
}

func (r *ByDateRule) GetDestination(fi *scanner.FileInfo) string {
	return fi.Date
}
