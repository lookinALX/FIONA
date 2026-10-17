// Package config reads the user configuration and the built-in presets.
package config

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lookinalx/fiona/internal/rules"
	"github.com/lookinalx/fiona/internal/suggest"
	"github.com/lookinalx/fiona/internal/types"
)

const (
	// EnvDir overrides the configuration directory, mostly for tests.
	EnvDir   = "FIONA_CONFIG_DIR"
	FileName = "config.json"

	DefaultPreset = "downloads"
)

//go:embed presets.json
var presetsJSON []byte

// builtinOrder is how the built-in presets are listed.
var builtinOrder = []string{"downloads", "photos", "by-date", "by-type"}

var builtin = func() map[string]Preset {
	var presets map[string]Preset
	if err := json.Unmarshal(presetsJSON, &presets); err != nil {
		panic(err)
	}
	for _, name := range builtinOrder {
		rules.MustCompile(presets[name].Rules)
	}
	return presets
}()

type Config struct {
	DefaultPreset string            `json:"default_preset,omitempty"`
	Action        string            `json:"action,omitempty"`
	OnConflict    string            `json:"on_conflict,omitempty"`
	Presets       map[string]Preset `json:"presets,omitempty"`

	// Path is the file the configuration was read from; empty if there is none.
	Path string `json:"-"`

	sets map[string]*rules.Set
}

type Preset struct {
	Description string       `json:"description,omitempty"`
	Rules       []rules.Rule `json:"rules"`
}

// Error is a problem in the configuration file.
type Error struct {
	Path string
	Err  error
}

// Error names the file on every line, as each line is a separate problem.
func (e *Error) Error() string { return prefixLines(e.Path+": ", e.Err).Error() }
func (e *Error) Unwrap() error { return e.Err }

// Dir is where the configuration and the run history live.
func Dir() (string, error) {
	if dir := os.Getenv(EnvDir); dir != "" {
		return filepath.Abs(dir)
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot find the configuration directory: %w", err)
	}
	return filepath.Join(dir, "fiona"), nil
}

// FilePath is the configuration file, whether it exists or not.
func FilePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// Load reads the configuration file. A missing file is not an error.
func Load() (*Config, error) {
	path, err := FilePath()
	if err != nil {
		return nil, err
	}
	return LoadFile(path)
}

func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{sets: builtinSets()}, nil
	}
	if err != nil {
		return nil, &Error{Path: path, Err: err}
	}

	c := &Config{}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(c); err != nil {
		return nil, &Error{Path: path, Err: describeJSONError(data, err)}
	}
	if dec.More() {
		return nil, &Error{Path: path, Err: errors.New("unexpected data after the configuration object")}
	}
	c.Path = path

	if err := c.validate(); err != nil {
		return nil, &Error{Path: path, Err: err}
	}
	return c, nil
}

func builtinSets() map[string]*rules.Set {
	sets := make(map[string]*rules.Set, len(builtin))
	for name, preset := range builtin {
		sets[name] = rules.MustCompile(preset.Rules)
	}
	return sets
}

func (c *Config) validate() error {
	var errs []error

	c.sets = builtinSets()
	for _, name := range sortedKeys(c.Presets) {
		preset := c.Presets[name]
		if strings.TrimSpace(name) == "" {
			errs = append(errs, errors.New("a preset has an empty name"))
			continue
		}
		if len(preset.Rules) == 0 {
			errs = append(errs, fmt.Errorf("preset %q: no rules", name))
			continue
		}
		set, err := rules.Compile(preset.Rules)
		if err != nil {
			errs = append(errs, prefixLines(fmt.Sprintf("preset %q, ", name), err))
			continue
		}
		c.sets[name] = set
	}

	if c.DefaultPreset != "" {
		if _, ok := c.sets[c.DefaultPreset]; !ok {
			errs = append(errs, fmt.Errorf("default_preset: %w", c.unknownPreset(c.DefaultPreset)))
		}
	}
	if c.Action != "" && c.Action != string(types.OpMove) && c.Action != string(types.OpCopy) {
		errs = append(errs, fmt.Errorf("action %q must be move or copy", c.Action))
	}
	switch c.OnConflict {
	case "", types.OnConflictRename, types.OnConflictSkip, types.OnConflictReplace:
	default:
		errs = append(errs, fmt.Errorf("on_conflict %q must be rename, skip or replace", c.OnConflict))
	}

	return errors.Join(errs...)
}

// prefixLines puts prefix before every line of a joined error.
func prefixLines(prefix string, err error) error {
	lines := strings.Split(err.Error(), "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return errors.New(strings.Join(lines, "\n"))
}

// describeJSONError adds the line and column to a decoding error.
func describeJSONError(data []byte, err error) error {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntaxErr):
		// the offset is just past the character that broke the syntax
		line, col := position(data, max(syntaxErr.Offset-1, 0))
		return fmt.Errorf("line %d, column %d: %s", line, col, syntaxErr.Error())
	case errors.As(err, &typeErr):
		line, col := position(data, typeErr.Offset)
		return fmt.Errorf("line %d, column %d: %s must be %s, not %s", line, col, typeErr.Field, typeErr.Type, typeErr.Value)
	case errors.Is(err, io.EOF):
		return errors.New("the file is empty")
	default:
		return errors.New(strings.TrimPrefix(err.Error(), "json: "))
	}
}

func position(data []byte, offset int64) (line, col int) {
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	before := data[:offset]
	line = bytes.Count(before, []byte{'\n'}) + 1
	col = int(offset) - bytes.LastIndexByte(before, '\n')
	return line, col
}

// DefaultPresetName is the preset used when nothing else is chosen.
func (c *Config) DefaultPresetName() string {
	if c.DefaultPreset != "" {
		return c.DefaultPreset
	}
	return DefaultPreset
}

// Names lists the built-in presets first, then the user's own by name.
func (c *Config) Names() []string {
	names := append([]string(nil), builtinOrder...)
	for _, name := range sortedKeys(c.Presets) {
		if _, isBuiltin := builtin[name]; !isBuiltin {
			names = append(names, name)
		}
	}
	return names
}

// Preset returns a preset; one from the configuration replaces a built-in one.
func (c *Config) Preset(name string) (Preset, error) {
	if preset, ok := c.Presets[name]; ok {
		return preset, nil
	}
	if preset, ok := builtin[name]; ok {
		return preset, nil
	}
	return Preset{}, c.unknownPreset(name)
}

// Rules returns the checked rules of a preset.
func (c *Config) Rules(name string) (*rules.Set, error) {
	if set, ok := c.sets[name]; ok {
		return set, nil
	}
	return nil, c.unknownPreset(name)
}

// Origin tells where a preset comes from: "built-in", "config" or "config, replaces built-in".
func (c *Config) Origin(name string) string {
	_, inConfig := c.Presets[name]
	_, isBuiltin := builtin[name]
	switch {
	case inConfig && isBuiltin:
		return "config, replaces built-in"
	case inConfig:
		return "config"
	default:
		return "built-in"
	}
}

// UnknownPresetError names a preset that does not exist.
type UnknownPresetError struct {
	Name      string
	Guess     string
	Available []string
}

func (e *UnknownPresetError) Error() string {
	msg := fmt.Sprintf("unknown preset %q.", e.Name)
	if e.Guess != "" {
		msg += fmt.Sprintf(" Did you mean %q?", e.Guess)
	}
	return msg + "\nAvailable: " + strings.Join(e.Available, ", ")
}

func (c *Config) unknownPreset(name string) error {
	names := c.Names()
	guess, _ := suggest.Closest(name, names)
	return &UnknownPresetError{Name: name, Guess: guess, Available: names}
}

func sortedKeys(m map[string]Preset) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
