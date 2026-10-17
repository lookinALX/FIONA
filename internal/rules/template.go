package rules

import (
	"fmt"
	"strings"

	"github.com/lookinalx/fiona/internal/scanner"
	"github.com/lookinalx/fiona/internal/suggest"
)

// Template is a destination path like "photos/{year}/{month}": text with
// placeholders, where "/" separates folders.
type Template struct {
	raw        string
	components [][]piece
}

// piece is either literal text or a placeholder.
type piece struct {
	text        string
	placeholder *Placeholder
}

// ParseTemplate checks the template and fails on unknown placeholders.
func ParseTemplate(raw string) (Template, error) {
	t := Template{raw: raw}

	if strings.TrimSpace(raw) == "" {
		return t, fmt.Errorf("empty template")
	}
	if strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, `\`) {
		return t, fmt.Errorf("template %q must be relative", raw)
	}

	for _, component := range strings.Split(raw, "/") {
		if component == "" {
			return t, fmt.Errorf("empty folder name in template %q", raw)
		}
		if component == "." || component == ".." {
			return t, fmt.Errorf("%q is not allowed in template %q", component, raw)
		}

		pieces, err := parseComponent(component)
		if err != nil {
			return t, err
		}
		t.components = append(t.components, pieces)
	}

	return t, nil
}

func parseComponent(s string) ([]piece, error) {
	var pieces []piece

	for s != "" {
		open := strings.IndexByte(s, '{')
		if closing := strings.IndexByte(s, '}'); closing >= 0 && (open < 0 || closing < open) {
			return nil, fmt.Errorf(`unexpected "}" in %q`, s)
		}
		if open < 0 {
			pieces = append(pieces, piece{text: s})
			break
		}
		if open > 0 {
			pieces = append(pieces, piece{text: s[:open]})
		}

		closing := strings.IndexByte(s[open:], '}')
		if closing < 0 {
			return nil, fmt.Errorf(`unclosed "{" in %q`, s)
		}
		name := s[open+1 : open+closing]
		ph, ok := placeholderByName[name]
		if !ok {
			return nil, unknownPlaceholder(name)
		}
		pieces = append(pieces, piece{placeholder: &ph})
		s = s[open+closing+1:]
	}

	return pieces, nil
}

func unknownPlaceholder(name string) error {
	if name == "" {
		return fmt.Errorf("empty placeholder {}")
	}
	if guess, ok := suggest.Closest(name, placeholderNames()); ok {
		return fmt.Errorf("unknown placeholder {%s}. Did you mean {%s}?", name, guess)
	}
	return fmt.Errorf("unknown placeholder {%s}. Available: {%s}", name, strings.Join(placeholderNames(), "}, {"))
}

// Render fills the placeholders for a file. Components are not sanitized:
// the planner does it for every folder name.
func (t Template) Render(fi *scanner.FileInfo) []string {
	out := make([]string, len(t.components))
	for i, pieces := range t.components {
		var b strings.Builder
		for _, p := range pieces {
			if p.placeholder != nil {
				b.WriteString(p.placeholder.Value(fi))
			} else {
				b.WriteString(p.text)
			}
		}
		out[i] = b.String()
	}
	return out
}

// Uses reports whether any of the placeholders appears in the template.
func (t Template) Uses(names ...string) bool {
	for _, pieces := range t.components {
		for _, p := range pieces {
			if p.placeholder == nil {
				continue
			}
			for _, name := range names {
				if p.placeholder.Name == name {
					return true
				}
			}
		}
	}
	return false
}

func (t Template) String() string {
	return t.raw
}
