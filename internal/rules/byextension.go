package rules

import (
	"strings"

	"github.com/lookinalx/fiona/internal/scanner"
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
