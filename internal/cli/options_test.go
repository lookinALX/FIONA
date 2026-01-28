package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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
		{"invalid criteria", "invalid", ErrInvalidPrimaryCriteria},
		{"empty string", "", ErrEmptyPrimaryCriteria},
		{"random string", "foobar", ErrInvalidPrimaryCriteria},
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
		{"invalid criteria", "invalid", ErrInvalidSecondaryCriteria},
		{"random string", "foobar", ErrInvalidSecondaryCriteria},
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
		{"invalid input string", "foobar", ErrInvalidAction},
		{"empty action", "", ErrInvalidAction},
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
		{"empty string", "", ErrEmptySource},
		{"non-existent path", "/path/that/does/not/exist", ErrSourceNotExists},
		{"file instead of directory", tmpFile, ErrSourceIsNotDir},
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
		{"empty string", "", ErrEmptyDest},
		{"non-existent path", "/path/that/does/not/exist", ErrDestNotExists},
		{"file instead of directory", tmpFile, ErrDestIsNotDir},
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
		"move",
		true,
	}

	optPrimaryInvalid := optValid
	optPrimaryInvalid.Sort.Primary = invalidOption

	optSecondaryInvalid := optValid
	optSecondaryInvalid.Sort.Secondary = invalidOption

	optActionInvalid := optValid
	optActionInvalid.Action = invalidOption

	optSourceInvalid := optValid
	optSourceInvalid.Source = ""

	optDestInvalid := optValid
	optDestInvalid.Dest = ""

	tests := []struct {
		name    string
		input   Opts
		wantErr error
	}{
		// valid case
		{"all valid", optValid, nil},
		// invalid cases
		{"primary invalid", optPrimaryInvalid, ErrInvalidPrimaryCriteria},
		{"secondary invalid", optSecondaryInvalid, ErrInvalidSecondaryCriteria},
		{"action invalid", optActionInvalid, ErrInvalidAction},
		{"source invalid", optSourceInvalid, ErrEmptySource},
		{"destination invalid", optDestInvalid, ErrEmptyDest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.validateOptions()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
