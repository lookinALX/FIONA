package rules

import "github.com/lookinALX/FIONA/internal/scanner"

type ByTypeRule struct {
	IsPrimary bool
}

func (r *ByTypeRule) GetDestination(fi *scanner.FileInfo) string {
	return fi.Type
}
