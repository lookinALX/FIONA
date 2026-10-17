package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lookinalx/fiona/internal/config"
	"github.com/lookinalx/fiona/internal/messages"
)

// chdir makes dir the working directory for the test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func resolved(t *testing.T, path string) string {
	t.Helper()
	r, err := ResolvePath(path)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func noConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.LoadFile(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestParseSort_Defaults(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	o, err := ParseSort(nil)
	if err != nil {
		t.Fatal(err)
	}
	if o.Source != resolved(t, dir) || o.Dest != o.Source {
		t.Errorf("source = %q, dest = %q; want both %q", o.Source, o.Dest, dir)
	}
	if o.Action != "move" || o.OnConflict != "rename" || o.Recursive || o.Hidden || o.DryRun || o.Yes || o.List {
		t.Errorf("defaults = %+v", o)
	}
}

func TestParseSort_Positional(t *testing.T) {
	src, dest := t.TempDir(), filepath.Join(t.TempDir(), "sorted")

	// A10: flags after the positional arguments are parsed too
	o, err := ParseSort([]string{src, dest, "-r", "--copy", "-ny"})
	if err != nil {
		t.Fatal(err)
	}
	if o.Source != resolved(t, src) || o.Dest != resolved(t, dest) {
		t.Errorf("source = %q, dest = %q", o.Source, o.Dest)
	}
	if !o.Recursive || o.Action != "copy" || !o.DryRun || !o.Yes {
		t.Errorf("flags are not parsed: %+v", o)
	}

	// dest defaults to source, not to the current folder
	o, err = ParseSort([]string{src})
	if err != nil {
		t.Fatal(err)
	}
	if o.Dest != o.Source {
		t.Errorf("dest = %q, want source %q", o.Dest, o.Source)
	}

	// -s and -d still work
	o, err = ParseSort([]string{"-s", src, "-d", dest})
	if err != nil || o.Source != resolved(t, src) || o.Dest != resolved(t, dest) {
		t.Errorf("ParseSort(-s -d) = %+v, %v", o, err)
	}
}

func TestParseSort_Errors(t *testing.T) {
	src := t.TempDir()
	file := filepath.Join(src, "file.txt")
	if err := os.WriteFile(file, nil, 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"three folders", []string{src, src, src}, []string{"unexpected arguments"}},
		{"source twice", []string{src, "-s", src}, []string{messages.ErrSourceTwice.Error()}},
		{"dest twice", []string{src, src, "-d", src}, []string{messages.ErrDestTwice.Error()}},
		{"copy and move", []string{src, "--copy", "-a", "move"}, []string{messages.ErrCopyAndAction.Error()}},
		{"typo in criteria", []string{src, "-c", "yaer"}, []string{`unknown criterion "yaer". Did you mean "year"?`}},
		{"then alone", []string{src, "-t", "month"}, []string{messages.ErrThenWithoutCriteria.Error()}},
		{"source is a file", []string{file}, []string{"source is not a folder"}},
		{"dest is a file", []string{src, file}, []string{"destination is not a folder"}},
		{"missing source", []string{filepath.Join(src, "missing")}, []string{"source folder doesn't exist"}},
		{"unknown flag", []string{src, "--force", "yes"}, []string{"unknown flag: --force"}},
		{
			"all problems at once",
			[]string{src, "-w", "0", "--on-conflict", "bogus", "-a", "fly", "--log", filepath.Join(src, "no")},
			[]string{"invalid number of workers", "invalid on-conflict", "invalid action", "log path doesn't exist"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSort(tt.args)
			var usage *UsageError
			if !errors.As(err, &usage) {
				t.Fatalf("error = %v, want a usage error", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error does not contain %q:\n%v", want, err)
				}
			}
		})
	}
}

func TestParseSort_DoesNotCreateDest(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "not", "created")
	if _, err := ParseSort([]string{t.TempDir(), dest}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(dest)); !os.IsNotExist(err) {
		t.Error("parsing created the destination")
	}
}

func TestRules_Priority(t *testing.T) {
	src := t.TempDir()
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"default_preset": "work", "presets": {"work": {"rules": [{"to": "work/{ext}"}]}}}`), 0644); err != nil {
		t.Fatal(err)
	}
	withDefault, err := config.LoadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		args   []string
		cfg    *config.Config
		source string
	}{
		{"--to wins", []string{src, "--to", "{year}", "-c", "ext", "-p", "photos"}, withDefault, `with --to "{year}"`},
		{"then -c and -t", []string{src, "-c", "year", "-t", "month", "-p", "photos"}, withDefault, "by year, then month"},
		{"then --preset", []string{src, "-p", "photos"}, withDefault, `with preset "photos"`},
		{"then default_preset", []string{src}, withDefault, `with preset "work" (default from config)`},
		{"then downloads", []string{src}, noConfig(t), `with preset "downloads"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, err := ParseSort(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			set, source, err := o.Rules(tt.cfg)
			if err != nil || set == nil {
				t.Fatalf("Rules() = %v, %v", set, err)
			}
			if source != tt.source {
				t.Errorf("source = %q, want %q", source, tt.source)
			}
		})
	}
}

func TestRules_Errors(t *testing.T) {
	src := t.TempDir()

	o, err := ParseSort([]string{src, "--to", "{yaer}/{month}"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = o.Rules(noConfig(t))
	var usage *UsageError
	if !errors.As(err, &usage) || err.Error() != "--to: unknown placeholder {yaer}. Did you mean {year}?" {
		t.Errorf("error = %v", err)
	}

	o, err = ParseSort([]string{src, "-p", "photso"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = o.Rules(noConfig(t))
	var unknown *config.UnknownPresetError
	if !errors.As(err, &usage) || !errors.As(err, &unknown) || unknown.Guess != "photos" {
		t.Errorf("error = %v", err)
	}
}

func TestApplyConfig(t *testing.T) {
	src := t.TempDir()
	cfg := &config.Config{Action: "copy", OnConflict: "skip"}

	o, err := ParseSort([]string{src})
	if err != nil {
		t.Fatal(err)
	}
	o.ApplyConfig(cfg)
	if o.Action != "copy" || o.OnConflict != "skip" {
		t.Errorf("config is not applied: %+v", o)
	}

	// the command line wins over the config
	o, err = ParseSort([]string{src, "-a", "move", "--on-conflict", "rename"})
	if err != nil {
		t.Fatal(err)
	}
	o.ApplyConfig(cfg)
	if o.Action != "move" || o.OnConflict != "rename" {
		t.Errorf("flags are overridden by the config: %+v", o)
	}
}

func TestParseUndo(t *testing.T) {
	dest := t.TempDir()
	journal := filepath.Join(dest, "run.jsonl")
	if err := os.WriteFile(journal, nil, 0644); err != nil {
		t.Fatal(err)
	}

	o, err := ParseUndo([]string{"20261010", "-y"})
	if err != nil || o.RunID != "20261010" || !o.Yes {
		t.Errorf("ParseUndo(run id) = %+v, %v", o, err)
	}

	o, err = ParseUndo([]string{"--dest", dest, "-n"})
	if err != nil || o.Dest != resolved(t, dest) || !o.DryRun {
		t.Errorf("ParseUndo(--dest) = %+v, %v", o, err)
	}

	o, err = ParseUndo(nil)
	if err != nil || o.RunID != "" || o.Dest != "" || o.LogPath != "" {
		t.Errorf("ParseUndo() = %+v, %v", o, err)
	}

	for _, args := range [][]string{
		{"20261010", "--dest", dest},
		{"--log", journal, "--dest", dest},
		{"--dest", filepath.Join(dest, "missing")},
		{"--log", dest},
		{"a", "b"},
	} {
		var usage *UsageError
		if _, err := ParseUndo(args); !errors.As(err, &usage) {
			t.Errorf("ParseUndo(%v) error = %v, want a usage error", args, err)
		}
	}
}

func TestPrompt(t *testing.T) {
	tests := []struct {
		input     string
		allowList bool
		want      Answer
	}{
		{"y\n", false, AnswerYes},
		{"YES\r\n", false, AnswerYes},
		{"n\n", false, AnswerNo},
		{"\n", false, AnswerNo},
		{"", false, AnswerNoInput},
		{"l\n", true, AnswerList},
		{"l\n", false, AnswerNo},
		{"y", false, AnswerYes}, // no newline before the end of input
	}

	for _, tt := range tests {
		var out strings.Builder
		got := NewPrompt(strings.NewReader(tt.input), &out, true).Ask("Go?", tt.allowList)
		if got != tt.want {
			t.Errorf("Ask(%q) = %v, want %v", tt.input, got, tt.want)
		}
		wantQuestion := "Go? [y/N] "
		if tt.allowList {
			wantQuestion = "Go? [y/N/l] "
		}
		if !strings.HasPrefix(out.String(), wantQuestion) {
			t.Errorf("question = %q, want %q", out.String(), wantQuestion)
		}
	}
}

func TestPrompt_AsksAgainWithoutLosingInput(t *testing.T) {
	var out strings.Builder
	p := NewPrompt(strings.NewReader("l\ny\n"), &out, true)
	if got := p.Ask("Go?", true); got != AnswerList {
		t.Fatalf("first answer = %v", got)
	}
	if got := p.Ask("Go?", true); got != AnswerYes {
		t.Fatalf("second answer = %v", got)
	}
}

func TestResolvePath(t *testing.T) {
	real := t.TempDir()
	resolvedReal, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"existing directory", real, resolvedReal},
		{"symlink to directory", link, resolvedReal},
		{"missing path below a symlink", filepath.Join(link, "not", "created"), filepath.Join(resolvedReal, "not", "created")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolvePath(tt.input)
			if err != nil {
				t.Fatalf("ResolvePath() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ResolvePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsInside(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "data", "downloads")

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"child", filepath.Join(root, "sorted"), true},
		{"deep child", filepath.Join(root, "a", "b"), true},
		{"same directory", root, false},
		{"parent", filepath.Dir(root), false},
		{"sibling", filepath.Join(filepath.Dir(root), "other"), false},
		{"sibling with the same prefix", root + "-old", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInside(root, tt.path); got != tt.want {
				t.Errorf("IsInside(%q, %q) = %v, want %v", root, tt.path, got, tt.want)
			}
		})
	}
}
