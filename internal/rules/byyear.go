package rules

import (
	"unicode"

	"github.com/lookinALX/FIONA/internal/scanner"
)

type ByYearRule struct {
	IsPrimary bool
}

func (r *ByYearRule) GetDestination(fi *scanner.FileInfo) string {
	if result := extractYear(fi.Date); result != "" {
		return result
	}
	return "unknown year"
}

func extractYear(date string) string {
	idx := -1
	for i, c := range date {
		if unicode.IsDigit(c) {
			idx = i
			break
		}
	}
	if idx != -1 {
		return date[idx:]
	}
	return ""
}
