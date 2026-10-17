package rules

import (
	"github.com/lookinalx/fiona/internal/scanner"
)

const (
	PhCategory = "category"
	PhExt      = "ext"
	PhYear     = "year"
	PhMonth    = "month"
	PhDay      = "day"
	PhDate     = "date"
	PhSize     = "size"
	PhML       = "ml"
)

// noExt is the folder name for files without an extension.
const noExt = "_no_ext"

// Placeholder is a value a template can put into a folder name.
type Placeholder struct {
	Name        string
	Description string
	Value       func(fi *scanner.FileInfo) string
}

// Placeholders are listed in the order they are shown to the user.
var Placeholders = []Placeholder{
	{PhCategory, "images, documents, archives, …", func(fi *scanner.FileInfo) string { return fi.Category() }},
	{PhExt, "extension in lower case, " + noExt + " if there is none", extValue},
	{PhYear, "2025: year taken for photos, otherwise year modified", func(fi *scanner.FileInfo) string { return fi.Date().Format("2006") }},
	{PhMonth, "01", func(fi *scanner.FileInfo) string { return fi.Date().Format("01") }},
	{PhDay, "15", func(fi *scanner.FileInfo) string { return fi.Date().Format("02") }},
	{PhDate, "2025-01-15", func(fi *scanner.FileInfo) string { return fi.Date().Format("2006-01-02") }},
	{PhSize, "tiny, small, medium, large, huge, massive", func(fi *scanner.FileInfo) string { return SizeCategory(fi.Size) }},
	{PhML, "tag of the ML classifier, category for non-images", mlValue},
}

// DatePlaceholders read the date taken from EXIF.
var DatePlaceholders = []string{PhYear, PhMonth, PhDay, PhDate}

var placeholderByName = func() map[string]Placeholder {
	m := make(map[string]Placeholder, len(Placeholders))
	for _, p := range Placeholders {
		m[p.Name] = p
	}
	return m
}()

func placeholderNames() []string {
	names := make([]string, len(Placeholders))
	for i, p := range Placeholders {
		names[i] = p.Name
	}
	return names
}

func extValue(fi *scanner.FileInfo) string {
	if fi.Ext == "" {
		return noExt
	}
	return fi.Ext
}

func mlValue(fi *scanner.FileInfo) string {
	if fi.Tag == "" {
		return fi.Category()
	}
	return fi.Tag
}

// SizeCategory names the size range a file falls into.
func SizeCategory(size int64) string {
	switch {
	case size < 10_240:
		return "tiny"
	case size < 102_400:
		return "small"
	case size < 1_048_576:
		return "medium"
	case size < 10_485_760:
		return "large"
	case size < 104_857_600:
		return "huge"
	default:
		return "massive"
	}
}
