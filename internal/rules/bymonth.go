package rules

import (
	"github.com/lookinALX/FIONA/internal/scanner"
)

type ByMonthRule struct {
	IsPrimary bool
}

func (r *ByMonthRule) GetDestination(fi *scanner.FileInfo) string {
	if result := extractMonth(fi.Date); result != "" {
		return result
	}
	return "unknown month"
}

func extractMonth(date string) string {
	idx := -1
	for i, c := range date {
		if c == ' ' {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ""
	}
	return date[:idx]
}
