package rules

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/lookinalx/fiona/internal/scanner"
	"github.com/lookinalx/fiona/internal/suggest"
)

// Rule sends the files that match it to the folder made from the template.
// The first rule that matches wins; a file no rule matches stays in place.
type Rule struct {
	Match Match  `json:"match,omitzero"`
	To    string `json:"to"`
}

// Match is a condition. An empty field matches anything; filled fields must
// all match. Inside a list one value is enough.
type Match struct {
	Category []string `json:"category,omitempty"`
	Ext      []string `json:"ext,omitempty"`  // without the dot, case does not matter
	Name     string   `json:"name,omitempty"` // glob on the file name, case does not matter
}

// Set is a list of checked rules, ready to route files.
type Set struct {
	rules []compiled
}

type compiled struct {
	categories map[string]struct{}
	exts       map[string]struct{}
	name       string
	to         Template
}

// Compile checks every rule and reports all problems at once, numbered from 1.
func Compile(rs []Rule) (*Set, error) {
	if len(rs) == 0 {
		return nil, errors.New("no rules")
	}

	set := &Set{}
	var errs []error
	for i, r := range rs {
		c, err := compile(r)
		if err != nil {
			errs = append(errs, fmt.Errorf("rule %d: %w", i+1, err))
			continue
		}
		set.rules = append(set.rules, c)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return set, nil
}

// MustCompile is for rules known to be valid, like the built-in presets.
func MustCompile(rs []Rule) *Set {
	set, err := Compile(rs)
	if err != nil {
		panic(err)
	}
	return set
}

func compile(r Rule) (compiled, error) {
	var c compiled

	to, err := ParseTemplate(r.To)
	if err != nil {
		return c, err
	}
	c.to = to

	if len(r.Match.Category) > 0 {
		c.categories = make(map[string]struct{})
		for _, category := range r.Match.Category {
			category = strings.ToLower(strings.TrimSpace(category))
			if !isCategory(category) {
				return c, unknownCategory(category)
			}
			c.categories[category] = struct{}{}
		}
	}

	if len(r.Match.Ext) > 0 {
		c.exts = make(map[string]struct{})
		for _, ext := range r.Match.Ext {
			c.exts[strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))] = struct{}{}
		}
	}

	if r.Match.Name != "" {
		c.name = strings.ToLower(r.Match.Name)
		if _, err := path.Match(c.name, ""); err != nil {
			return c, fmt.Errorf("bad name pattern %q", r.Match.Name)
		}
	}

	return c, nil
}

func isCategory(name string) bool {
	for _, category := range scanner.Categories {
		if category == name {
			return true
		}
	}
	return false
}

func unknownCategory(name string) error {
	available := strings.Join(scanner.Categories, ", ")
	if guess, ok := suggest.Closest(name, scanner.Categories); ok {
		return fmt.Errorf("unknown category %q. Did you mean %q? Available: %s", name, guess, available)
	}
	return fmt.Errorf("unknown category %q. Available: %s", name, available)
}

func (c compiled) matches(fi *scanner.FileInfo) bool {
	if c.exts != nil {
		if _, ok := c.exts[fi.Ext]; !ok {
			return false
		}
	}
	if c.name != "" {
		if ok, _ := path.Match(c.name, strings.ToLower(fi.Name)); !ok {
			return false
		}
	}
	// the category is checked last: it may read the file
	if c.categories != nil {
		if _, ok := c.categories[fi.Category()]; !ok {
			return false
		}
	}
	return true
}

// Match returns the index of the first rule that matches the file.
func (s *Set) Match(fi *scanner.FileInfo) (int, bool) {
	for i, c := range s.rules {
		if c.matches(fi) {
			return i, true
		}
	}
	return -1, false
}

// Route returns the folder names for a file, or false if no rule matches it.
func (s *Set) Route(fi *scanner.FileInfo) ([]string, bool) {
	i, ok := s.Match(fi)
	if !ok {
		return nil, false
	}
	return s.rules[i].to.Render(fi), true
}

// RuleUses reports whether rule i uses any of the placeholders.
func (s *Set) RuleUses(i int, names ...string) bool {
	return s.rules[i].to.Uses(names...)
}

// Uses reports whether any rule uses any of the placeholders.
func (s *Set) Uses(names ...string) bool {
	for i := range s.rules {
		if s.RuleUses(i, names...) {
			return true
		}
	}
	return false
}

// Len is the number of rules.
func (s *Set) Len() int {
	return len(s.rules)
}

// Matches reports whether some rule matches the file.
func (s *Set) Matches(fi *scanner.FileInfo) bool {
	_, ok := s.Match(fi)
	return ok
}
