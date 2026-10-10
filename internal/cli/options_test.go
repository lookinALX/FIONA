package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lookinalx/fiona/internal/messages"
	"github.com/lookinalx/fiona/internal/rules"
)

func TestValidatePrimaryCriteriaFlag(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		// valid cases
		{"valid mimetype flag", CritMIMEType, nil},
		{"valid extension flag", CritExtension, nil},
		{"valid year flag", CritYear, nil},
		{"valid month flag", CritMonth, nil},
		{"valid date flag", CritDate, nil},
		{"valid size flag", CritSize, nil},
		// invalid cases
		{"invalid criteria", "invalid", messages.ErrInvalidPrimaryCriteria},
		{"empty string", "", messages.ErrEmptyPrimaryCriteria},
		{"random string", "foobar", messages.ErrInvalidPrimaryCriteria},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePrimaryCriteriaFlag(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validatePrimaryCriteriaFlag() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSecondaryCriteriaFlag(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		// valid cases
		{"valid mimetype flag", CritMIMEType, nil},
		{"valid extension flag", CritExtension, nil},
		{"valid year flag", CritYear, nil},
		{"valid month flag", CritMonth, nil},
		{"valid date flag", CritDate, nil},
		{"valid size flag", CritSize, nil},
		{"empty string", "", nil},
		// invalid cases
		{"invalid criteria", "invalid", messages.ErrInvalidSecondaryCriteria},
		{"random string", "foobar", messages.ErrInvalidSecondaryCriteria},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSecondaryCriteriaFlag(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateSecondaryCriteriaFlag() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateActionFlag(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		// valid cases
		{"valid move flag", "move", nil},
		{"valid copy flag", "copy", nil},
		// invalid cases
		{"invalid input string", "foobar", messages.ErrInvalidAction},
		{"empty action", "", messages.ErrInvalidAction},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateActionFlag(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateActionFlag() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSourceFlag(t *testing.T) {
	tmpDir := t.TempDir()

	tmpFile := filepath.Join(tmpDir, "testfile.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("faild to create temp file: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		// valid cases
		{"valid directory", tmpDir, nil},
		// invalid cases
		{"empty string", "", messages.ErrEmptySource},
		{"non-existent path", "/path/that/does/not/exist", messages.ErrSourceNotExists},
		{"file instead of directory", tmpFile, messages.ErrSourceIsNotDir},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSourceFlag(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateSourceFlag() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDestFlag(t *testing.T) {
	tmpDir := t.TempDir()

	tmpFile := filepath.Join(tmpDir, "testfile.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("faild to create temp file: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		// valid cases
		{"valid directory", tmpDir, nil},
		{"non-existent path is created later", filepath.Join(tmpDir, "does", "not", "exist"), nil},
		// invalid cases
		{"empty string", "", messages.ErrEmptyDest},
		{"file instead of directory", tmpFile, messages.ErrDestIsNotDir},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDestFlag(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateDestFlag() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateLogPathFlag(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("faild to create temp file: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		// valid cases
		{"valid directory", tmpDir, nil},
		{"empty path means the default location", "", nil},
		// invalid cases
		{"non-existent path", "/path/that/does/not/exist", messages.ErrLogPathNotExists},
		{"file instead of directory", tmpFile, messages.ErrLogPathIsNotDir},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLogFlag(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateLogFlag() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateOnConflictFlag(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		// valid cases
		{"valid replace", "replace", nil},
		{"valid rename", "rename", nil},
		{"valid skip", "skip", nil},
		//invalid cases
		{"invalid on-conflict", "invalid", messages.ErrInvalidOnConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOnConflictFlag(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateOnConflictFlag() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPathExists(t *testing.T) {
	tmpDir := t.TempDir()

	tmpFile := filepath.Join(tmpDir, "testfile.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	nonExistentPath := filepath.Join(tmpDir, "does_not_exist")

	tests := []struct {
		name       string
		input      string
		wantExists bool
		wantIsDir  bool
		wantErr    bool
	}{
		{"existing directory", tmpDir, true, true, false},
		{"existing file", tmpFile, true, false, false},
		{"non-existent path", nonExistentPath, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, exists, err := pathExists(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
			}

			if exists != tt.wantExists {
				t.Errorf("got exists %v, want %v", exists, tt.wantExists)
			}

			if exists && info.IsDir() != tt.wantIsDir {
				t.Errorf("got isDir %v, want %v", info.IsDir(), tt.wantIsDir)
			}
		})
	}
}

func TestValidateWorkersFlag(t *testing.T) {
	tests := []struct {
		name    string
		input   int
		want    int
		wantErr error
	}{
		{"one worker", 1, 1, nil},
		{"max workers", maxWorkers, maxWorkers, nil},
		{"more than max is cut", maxWorkers + 10, maxWorkers, nil},
		{"zero", 0, 0, messages.ErrInvalidWorkers},
		{"negative", -3, -3, messages.ErrInvalidWorkers},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workers := tt.input
			err := validateWorkersFlag(&workers)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateWorkersFlag() error = %v, wantErr %v", err, tt.wantErr)
			}
			if workers != tt.want {
				t.Errorf("workers = %d, want %d", workers, tt.want)
			}
		})
	}
}

func TestValidateOptions(t *testing.T) {
	tmpDir := t.TempDir()
	MlConfigPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(MlConfigPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	invalidOption := "invalid"

	optValid := Opts{
		Sort:             SortOption{CritMIMEType, CritEmpty},
		SourcePath:       tmpDir,
		DestPath:         tmpDir,
		LogPath:          tmpDir,
		MlConfigPath:     MlConfigPath,
		FileAction:       "move",
		ConflictStrategy: "replace",
		Workers:          1,
	}

	optPrimaryInvalid := optValid
	optPrimaryInvalid.Sort.Primary = invalidOption

	optSecondaryInvalid := optValid
	optSecondaryInvalid.Sort.Secondary = invalidOption

	optActionInvalid := optValid
	optActionInvalid.FileAction = invalidOption

	optSourceInvalid := optValid
	optSourceInvalid.SourcePath = ""

	optDestInvalid := optValid
	optDestInvalid.DestPath = ""

	optLogPathInvalid := optValid
	optLogPathInvalid.LogPath = filepath.Join(tmpDir, "missing")

	optOnConflictInvalid := optValid
	optOnConflictInvalid.ConflictStrategy = ""

	optWorkersInvalid := optValid
	optWorkersInvalid.Workers = 0

	optMlConfigInvalid := optValid
	optMlConfigInvalid.Sort.Secondary = "ml"

	tests := []struct {
		name    string
		input   Opts
		wantErr error
	}{
		// valid case
		{"all valid", optValid, nil},
		// invalid cases
		{"primary invalid", optPrimaryInvalid, messages.ErrInvalidPrimaryCriteria},
		{"secondary invalid", optSecondaryInvalid, messages.ErrInvalidSecondaryCriteria},
		{"action invalid", optActionInvalid, messages.ErrInvalidAction},
		{"source invalid", optSourceInvalid, messages.ErrEmptySource},
		{"destination invalid", optDestInvalid, messages.ErrEmptyDest},
		{"log path invalid", optLogPathInvalid, messages.ErrLogPathNotExists},
		{"on conflict invalid", optOnConflictInvalid, messages.ErrInvalidOnConflict},
		{"workers invalid", optWorkersInvalid, messages.ErrInvalidWorkers},
		{"ml config path invalid", optMlConfigInvalid, messages.ErrConfigMLIsNotJson},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.validateSortOptions()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateSortOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateOptions_ReportsAllErrors(t *testing.T) {
	opts := Opts{
		Sort:             SortOption{"bogus", "bogus"},
		SourcePath:       "",
		DestPath:         filepath.Join(t.TempDir(), "missing"),
		LogPath:          filepath.Join(t.TempDir(), "missing"),
		FileAction:       "delete",
		ConflictStrategy: "bogus",
		Workers:          0,
	}

	err := opts.validateSortOptions()

	for _, want := range []error{
		messages.ErrInvalidPrimaryCriteria,
		messages.ErrInvalidSecondaryCriteria,
		messages.ErrInvalidAction,
		messages.ErrEmptySource,
		messages.ErrInvalidOnConflict,
		messages.ErrInvalidWorkers,
		messages.ErrLogPathNotExists,
	} {
		if !errors.Is(err, want) {
			t.Errorf("validateSortOptions() error does not contain %q:\n%v", want, err)
		}
	}
}

func TestOpts_ParseSortFlagsToRules(t *testing.T) {
	tests := []struct {
		name      string
		input     Opts
		wantErr   error
		wantRules []rules.Rule
	}{
		{
			name: "invalid rule",
			input: Opts{
				Sort: SortOption{
					Primary:   CritEmpty,
					Secondary: CritEmpty,
				},
			},
			wantErr:   messages.ErrUnknownRuleName,
			wantRules: nil,
		},
		{
			name: "valid rule without secondary",
			input: Opts{
				Sort: SortOption{
					Primary:   CritMIMEType,
					Secondary: CritEmpty,
				},
			},
			wantErr: nil,
			wantRules: []rules.Rule{
				&rules.ByTypeRule{IsPrimary: true},
			},
		},
		{
			name: "valid rule with secondary",
			input: Opts{
				Sort: SortOption{
					Primary:   CritMIMEType,
					Secondary: CritYear,
				},
			},
			wantErr: nil,
			wantRules: []rules.Rule{
				&rules.ByTypeRule{IsPrimary: true},
				&rules.ByYearRule{IsPrimary: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRules, err := tt.input.ParseSortFlagsToRules()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ParseSortFlagsToRules() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(gotRules, tt.wantRules) {
				t.Errorf("ParseSortFlagsToRules() gotRules = %v, want %v", gotRules, tt.wantRules)
			}
		})
	}
}

func TestValidateLogUndoFlag(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_log.json")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"valid file", tmpFile, nil},
		{"empty string", "", messages.ErrLogPathIsEmpty},
		{"non-existent path", "/path/that/does/not/exist.json", messages.ErrLogPathNotExists},
		{"directory instead of file", tmpDir, messages.ErrLogUndoPathIsDir},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLogUndoFlag(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("got error %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseSortFlags_Defaults(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	resolvedCwd, err := ResolvePath(cwd)
	if err != nil {
		t.Fatal(err)
	}

	var opts Opts
	if err := opts.ParseSortFlags(nil); err != nil {
		t.Fatalf("ParseSortFlags() error = %v", err)
	}

	want := Opts{
		Sort:             SortOption{Primary: CritMIMEType},
		SourcePath:       resolvedCwd,
		DestPath:         resolvedCwd,
		FileAction:       "copy",
		ConflictStrategy: ConflictRename,
		Workers:          maxWorkers,
	}
	if opts != want {
		t.Errorf("ParseSortFlags() = %+v, want %+v", opts, want)
	}
}

func TestParseSortFlags_AllFlags(t *testing.T) {
	source, logDir := t.TempDir(), t.TempDir()
	dest := filepath.Join(source, "sorted")

	short := []string{"-c", "extension", "-t", "year", "-s", source, "-d", dest, "-a", "move", "-n", "-y", "-r", "-w", "1",
		"--on-conflict", "replace", "--hidden", "--log", logDir}
	long := []string{"--criteria", "extension", "--then", "year", "--source", source, "--dest", dest, "--action", "move",
		"--dry-run", "--yes", "--recursive", "--workers", "1", "--on-conflict", "replace", "--hidden", "--log", logDir}

	resolvedSource, err := ResolvePath(source)
	if err != nil {
		t.Fatal(err)
	}
	want := Opts{
		Sort:             SortOption{Primary: CritExtension, Secondary: CritYear},
		SourcePath:       resolvedSource,
		DestPath:         filepath.Join(resolvedSource, "sorted"),
		LogPath:          logDir,
		FileAction:       "move",
		ConflictStrategy: ConflictReplace,
		DryRun:           true,
		Yes:              true,
		Recursive:        true,
		Hidden:           true,
		Workers:          1,
	}

	for _, args := range [][]string{short, long} {
		var opts Opts
		if err := opts.ParseSortFlags(args); err != nil {
			t.Fatalf("ParseSortFlags(%v) error = %v", args, err)
		}
		if opts != want {
			t.Errorf("ParseSortFlags(%v) = %+v, want %+v", args, opts, want)
		}
	}
}

func TestParseSortFlags_ValidatesEverythingBeforeSideEffects(t *testing.T) {
	source := t.TempDir()
	dest := filepath.Join(t.TempDir(), "not", "created")

	var opts Opts
	err := opts.ParseSortFlags([]string{"-s", source, "-d", dest, "-w", "0", "--on-conflict", "bogus", "--log", filepath.Join(source, "missing")})

	for _, want := range []error{messages.ErrInvalidWorkers, messages.ErrInvalidOnConflict, messages.ErrLogPathNotExists} {
		if !errors.Is(err, want) {
			t.Errorf("ParseSortFlags() error does not contain %q:\n%v", want, err)
		}
	}
	if _, statErr := os.Stat(filepath.Dir(dest)); !os.IsNotExist(statErr) {
		t.Error("destination directory was created although the options are invalid")
	}
}

func TestParseSortFlags_DoesNotCreateDest(t *testing.T) {
	source := t.TempDir()
	dest := filepath.Join(t.TempDir(), "not", "created")

	var opts Opts
	if err := opts.ParseSortFlags([]string{"-s", source, "-d", dest}); err != nil {
		t.Fatalf("ParseSortFlags() error = %v", err)
	}

	if _, err := os.Stat(filepath.Dir(dest)); !os.IsNotExist(err) {
		t.Error("destination directory was created while parsing flags")
	}
	if filepath.Base(opts.DestPath) != "created" || !filepath.IsAbs(opts.DestPath) {
		t.Errorf("DestPath = %q", opts.DestPath)
	}
}

func TestParseSortFlags_RejectedArguments(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{"removed force flag", []string{"--force", "yes"}, nil},
		{"unknown flag", []string{"--bogus"}, nil},
		{"negative workers", []string{"-w", "-3"}, messages.ErrInvalidWorkers},
		{"workers is not a number", []string{"-w", "many"}, nil},
		{"positional argument", []string{"somewhere"}, messages.ErrUnexpectedArguments},
		{"flags after a positional argument", []string{"-a", "move", "somewhere", "-r"}, messages.ErrUnexpectedArguments},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts Opts
			err := opts.ParseSortFlags(tt.args)
			if err == nil {
				t.Fatalf("ParseSortFlags(%v) succeeded, want error", tt.args)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("ParseSortFlags(%v) error = %v, want %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestParseUndoFlags(t *testing.T) {
	tmpDir := t.TempDir()

	validLogPath := filepath.Join(tmpDir, "run.jsonl")
	if err := os.WriteFile(validLogPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create temp log file: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{"valid log path", []string{"--log", validLogPath}, nil},
		{"valid dest", []string{"--dest", tmpDir}, nil},
		{"all other flags", []string{"-d", tmpDir, "-n", "-y", "-w", "1"}, nil},
		{"non-existent log path", []string{"--log", filepath.Join(tmpDir, "missing.jsonl")}, messages.ErrLogPathNotExists},
		{"directory instead of file", []string{"--log", tmpDir}, messages.ErrLogUndoPathIsDir},
		{"non-existent dest", []string{"--dest", filepath.Join(tmpDir, "missing")}, messages.ErrDestNotExists},
		{"file instead of dest directory", []string{"--dest", validLogPath}, messages.ErrDestIsNotDir},
		{"log and dest together", []string{"--log", validLogPath, "--dest", tmpDir}, messages.ErrLogAndDestExclusive},
		{"zero workers", []string{"--log", validLogPath, "-w", "0"}, messages.ErrInvalidWorkers},
		{"positional argument", []string{"20261010-153012-a1b2"}, messages.ErrUnexpectedArguments},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &Opts{}
			err := opts.ParseUndoFlags(tt.args)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ParseUndoFlags() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseUndoFlags_LogPath(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "run.jsonl")
	if err := os.WriteFile(logPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := &Opts{}
	if err := opts.ParseUndoFlags([]string{"--log", logPath}); err != nil {
		t.Fatalf("ParseUndoFlags() error = %v", err)
	}

	if opts.LogPath != logPath || opts.DestPath != "" {
		t.Errorf("LogPath = %q, DestPath = %q", opts.LogPath, opts.DestPath)
	}
}

func TestParseUndoFlags_DefaultsToCurrentDirectory(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	resolvedCwd, err := ResolvePath(cwd)
	if err != nil {
		t.Fatal(err)
	}

	opts := &Opts{}
	if err := opts.ParseUndoFlags(nil); err != nil {
		t.Fatalf("ParseUndoFlags() with default should not error: %v", err)
	}

	if opts.DestPath != resolvedCwd || opts.LogPath != "" {
		t.Errorf("DestPath = %q, LogPath = %q; want %q and empty", opts.DestPath, opts.LogPath, resolvedCwd)
	}
	if opts.DryRun || opts.Yes || opts.Workers != maxWorkers {
		t.Errorf("unexpected defaults: %+v", opts)
	}
}

func TestConfirm(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"y", "y\n", true},
		{"upper case y", "Y\n", true},
		{"yes", "yes\n", true},
		{"mixed case yes", "YeS\n", true},
		{"windows line ending", "y\r\n", true},
		{"spaces around", "  yes \t\n", true},
		{"no newline before end of input", "y", true},
		{"n", "n\n", false},
		{"no", "no\n", false},
		{"empty line", "\n", false},
		{"end of input", "", false},
		{"something else", "yep\n", false},
		{"only the first line counts", "n\ny\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer

			if got := Confirm(strings.NewReader(tt.input), &out, "Continue?"); got != tt.want {
				t.Errorf("Confirm(%q) = %v, want %v", tt.input, got, tt.want)
			}
			if !strings.Contains(out.String(), "Continue? [y/N]") {
				t.Errorf("prompt = %q", out.String())
			}
		})
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
