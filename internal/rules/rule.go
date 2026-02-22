package rules

import "github.com/lookinalx/fiona/internal/scanner"

type Rule interface {
	GetDestination(fi *scanner.FileInfo) string
}
