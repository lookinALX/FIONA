package cli

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
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
		// invalid cases
		{"empty string", "", messages.ErrEmptyDest},
		{"non-existent path", "/path/that/does/not/exist", messages.ErrDestNotExists},
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
		// invalid cases
		{"empty path", "", messages.ErrLogPathIsEmpty},
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

func TestValidateOptions(t *testing.T) {
	tmpDir := t.TempDir()

	invalidOption := "invalid"

	optValid := Opts{
		SortOption{CritMIMEType, CritEmpty},
		tmpDir,
		tmpDir,
		tmpDir,
		"move",
		"replace",
		"N",
		false,
		1,
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
	optLogPathInvalid.LogPath = ""

	optOnConflictInvalid := optValid
	optOnConflictInvalid.ConflictStrategy = ""

	optWorkersInvalid := optValid
	optWorkersInvalid.Workers = -1

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
		{"log path invalid", optLogPathInvalid, messages.ErrLogPathIsEmpty},
		{"on conflict invalid", optOnConflictInvalid, messages.ErrInvalidOnConflict},
		{"workers invalid", optWorkersInvalid, messages.ErrInvalidWorkers},
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

func TestParseUndoFlags(t *testing.T) {
	tmpDir := t.TempDir()

	validLogPath := filepath.Join(tmpDir, "fiona_logs.json")
	if err := os.WriteFile(validLogPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create temp log file: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		setup   func()
		wantErr bool
	}{
		{
			name: "valid log path",
			args: []string{"cmd", "--log", validLogPath},
			setup: func() {
				flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
			},
			wantErr: false,
		},
		{
			name: "non-existent log path",
			args: []string{"cmd", "--log", "/does/not/exist.json"},
			setup: func() {
				flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
			},
			wantErr: true,
		},
		{
			name: "directory instead of file",
			args: []string{"cmd", "--log", tmpDir},
			setup: func() {
				flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()

			if tt.setup != nil {
				tt.setup()
			}

			os.Args = tt.args

			opts := &Opts{}
			err := opts.ParseUndoFlags()

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseUndoFlags() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && opts.LogPath == "" {
				t.Error("LogPath should be set when no error")
			}
		})
	}
}

func TestParseUndoFlagsDefaultValue(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	defaultLogPath := filepath.Join(cwd, "fiona_logs.json")

	if err := os.WriteFile(defaultLogPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create default log file: %v", err)
	}
	defer os.Remove(defaultLogPath)

	os.Args = []string{"cmd"}

	opts := &Opts{}
	err = opts.ParseUndoFlags()

	if err != nil {
		t.Errorf("ParseUndoFlags() with default should not error: %v", err)
	}

	if opts.LogPath != defaultLogPath {
		t.Errorf("expected default LogPath %s, got %s", defaultLogPath, opts.LogPath)
	}
}

func TestParseUndoFlagsEmptyLogPath(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	os.Args = []string{"cmd", "--log", ""}

	opts := &Opts{}
	err := opts.ParseUndoFlags()

	if !errors.Is(err, messages.ErrLogPathIsEmpty) {
		t.Errorf("expected ErrLogPathIsEmpty, got %v", err)
	}
}
