package rules

import "FIONA/internal/scanner"

type ByExtensionRule struct {
	IsPrimary bool
}

func (r *ByExtensionRule) GetDestination(fi *scanner.FileInfo) string {
	return fi.Extension[1:]
}
