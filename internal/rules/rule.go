package rules

import "FIONA/internal/scanner"

type Rule interface {
	GetDestination(fi *scanner.FileInfo) string
}
