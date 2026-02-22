package rules

import "github.com/lookinALX/FIONA/internal/scanner"

type Rule interface {
	GetDestination(fi *scanner.FileInfo) string
}
