package rules

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxComponentBytes = 120
	fallbackComponent = "_other"
)

var windowsReservedNames = map[string]struct{}{
	"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
	"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {}, "COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
	"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {}, "LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
}

// SanitizeComponent turns a folder name returned by a rule into a single safe path component.
func SanitizeComponent(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || unicode.IsControl(r) || strings.ContainsRune(`<>:"|?*`, r) {
			return '_'
		}
		return r
	}, s)

	s = trimComponent(s)
	if s == "" || s == "." || s == ".." {
		return fallbackComponent
	}

	if _, reserved := windowsReservedNames[strings.ToUpper(s)]; reserved {
		s = "_" + s
	}

	if len(s) > maxComponentBytes {
		cut := maxComponentBytes
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = trimComponent(s[:cut])
		if s == "" {
			return fallbackComponent
		}
	}

	return s
}

func trimComponent(s string) string {
	return strings.TrimRight(strings.Trim(s, " "), ". ")
}
