package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFile_MissingIsNotAnError(t *testing.T) {
	c, err := LoadFile(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatal(err)
	}
	if c.Path != "" || c.DefaultPresetName() != "downloads" {
		t.Errorf("config = %+v", c)
	}
	for _, name := range []string{"downloads", "photos", "by-date", "by-type"} {
		if _, err := c.Rules(name); err != nil {
			t.Errorf("built-in preset %s: %v", name, err)
		}
	}
}

func TestLoadFile_UserPresets(t *testing.T) {
	path := writeConfig(t, `{
		"default_preset": "work",
		"action": "copy",
		"on_conflict": "skip",
		"presets": {
			"work": {
				"description": "Invoices by year, photos by month, the rest by type",
				"rules": [
					{ "match": { "name": "invoice*", "ext": ["pdf"] }, "to": "invoices/{year}" },
					{ "match": { "category": ["images"] }, "to": "photos/{year}/{month}" },
					{ "to": "{category}" }
				]
			},
			"photos": { "rules": [ { "to": "pictures/{year}" } ] }
		}
	}`)

	c, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Path != path || c.DefaultPresetName() != "work" || c.Action != "copy" || c.OnConflict != "skip" {
		t.Errorf("config = %+v", c)
	}

	set, err := c.Rules("work")
	if err != nil || set.Len() != 3 {
		t.Fatalf("work: %v, %v", set, err)
	}

	// a preset from the config replaces the built-in one
	photos, err := c.Preset("photos")
	if err != nil || photos.Rules[0].To != "pictures/{year}" {
		t.Errorf("photos = %+v, %v", photos, err)
	}
	if got := c.Origin("photos"); got != "config, replaces built-in" {
		t.Errorf("Origin(photos) = %q", got)
	}
	if got := c.Origin("work"); got != "config" {
		t.Errorf("Origin(work) = %q", got)
	}

	if want := []string{"downloads", "photos", "by-date", "by-type", "work"}; !reflect.DeepEqual(c.Names(), want) {
		t.Errorf("Names() = %v, want %v", c.Names(), want)
	}
}

func TestLoadFile_Errors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "unknown placeholder",
			content: `{"presets": {"work": {"rules": [{"to": "{category}"}, {"to": "{yaer}"}]}}}`,
			want:    []string{`preset "work", rule 2: unknown placeholder {yaer}. Did you mean {year}?`},
		},
		{
			name:    "syntax error",
			content: "{\n  \"default_preset\": \"work\",\n  \"action\" \"move\"\n}",
			want:    []string{"line 3, column 12"},
		},
		{
			name:    "wrong type",
			content: "{\n  \"presets\": {\"x\": {\"rules\": [{\"to\": 5}]}}\n}",
			want:    []string{"line 2", "rules.to must be string"},
		},
		{
			name:    "unknown field",
			content: `{"preset": {}}`,
			want:    []string{`unknown field "preset"`},
		},
		{
			name:    "unknown default preset",
			content: `{"default_preset": "photso"}`,
			want:    []string{`default_preset: unknown preset "photso". Did you mean "photos"?`},
		},
		{
			name:    "bad values",
			content: `{"action": "mov", "on_conflict": "overwrite"}`,
			want:    []string{`action "mov" must be move or copy`, `on_conflict "overwrite" must be rename, skip or replace`},
		},
		{
			name:    "no rules",
			content: `{"presets": {"empty": {"rules": []}}}`,
			want:    []string{`preset "empty": no rules`},
		},
		{
			name:    "empty file",
			content: ``,
			want:    []string{"the file is empty"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.content)
			_, err := LoadFile(path)

			var cfgErr *Error
			if !errors.As(err, &cfgErr) {
				t.Fatalf("error = %v, want a config error", err)
			}
			if !strings.HasPrefix(err.Error(), path+": ") {
				t.Errorf("error does not name the file: %v", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error does not contain %q:\n%v", want, err)
				}
			}
		})
	}
}

func TestUnknownPreset(t *testing.T) {
	c, err := LoadFile(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.Rules("photso")
	var unknown *UnknownPresetError
	if !errors.As(err, &unknown) || unknown.Guess != "photos" {
		t.Fatalf("error = %v", err)
	}
	if want := "unknown preset \"photso\". Did you mean \"photos\"?\nAvailable: downloads, photos, by-date, by-type"; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvDir, dir)

	got, err := Dir()
	if err != nil || got != dir {
		t.Errorf("Dir() = %q, %v; want %q", got, err, dir)
	}
}
