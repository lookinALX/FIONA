package rules

import (
	"FIONA/internal/scanner"
	"strings"
)

type ByExtensionRule struct {
	IsPrimary bool
}

func (r *ByExtensionRule) GetDestination(fi *scanner.FileInfo) string {
	if fi.Extension == "" {
		return "_no_ext"
	}
	return strings.TrimPrefix(fi.Extension, ".")
}
