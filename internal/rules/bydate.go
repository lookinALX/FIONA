package rules

import "FIONA/internal/scanner"

type ByDateRule struct {
	IsPrimary bool
}

func (r *ByDateRule) GetDestination(fi *scanner.FileInfo) string {
	return fi.Date
}
