package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lookinalx/fiona/internal/scanner/exiftest"
)

func TestTaken(t *testing.T) {
	taken := time.Date(2019, 6, 15, 10, 30, 0, 0, time.Local)

	tests := []struct {
		name    string
		content []byte
	}{
		{"photo.jpg", exiftest.JPEG(taken)},
		{"photo.heic", exiftest.HEIC(taken)},
		{"photo.dng", exiftest.TIFF(taken)},
		{"no_extension", exiftest.JPEG(taken)},
	}

	dir := t.TempDir()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.name)
			if err := os.WriteFile(path, tt.content, 0644); err != nil {
				t.Fatal(err)
			}
			mtime := time.Date(2026, 3, 1, 12, 0, 0, 0, time.Local)
			if err := os.Chtimes(path, mtime, mtime); err != nil {
				t.Fatal(err)
			}

			fi, err := NewFileInfo(path)
			if err != nil {
				t.Fatal(err)
			}
			got := fi.Taken()
			if got == nil || !got.Equal(taken) {
				t.Fatalf("Taken() = %v, want %v", got, taken)
			}
			if !fi.Date().Equal(taken) {
				t.Errorf("Date() = %v, want the date taken %v", fi.Date(), taken)
			}
		})
	}
}

func TestTaken_FallsBackToModTime(t *testing.T) {
	dir := t.TempDir()
	mtime := time.Date(2026, 3, 1, 12, 0, 0, 0, time.Local)

	files := map[string][]byte{
		"plain.jpg":    {0xFF, 0xD8, 0xFF, 0xD9},       // JPEG without EXIF
		"broken.jpg":   {0xFF, 0xD8, 0xFF, 0xE1, 0xFF}, // truncated segment
		"report.pdf":   []byte("%PDF-1.7"),             // not an image: never read
		"garbage.heic": []byte("\x00\x00\x00\x10ftypheic\x00\x00\x00\xFFmeta"),
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
		fi, err := NewFileInfo(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fi.Taken(); got != nil {
			t.Errorf("%s: Taken() = %v, want nil", name, got)
		}
		if !fi.Date().Equal(mtime) {
			t.Errorf("%s: Date() = %v, want mtime %v", name, fi.Date(), mtime)
		}
	}
}

func TestParseExifDate(t *testing.T) {
	tests := []struct {
		in string
		ok bool
	}{
		{"2019:06:15 10:30:00\x00", true},
		{"2019-06-15 10:30:00", true},
		{"    :  :     :  :  ", false},
		{"0000:00:00 00:00:00", false},
		{"2019:06", false},
	}
	for _, tt := range tests {
		if _, ok := parseExifDate(tt.in); ok != tt.ok {
			t.Errorf("parseExifDate(%q) ok = %v, want %v", tt.in, ok, tt.ok)
		}
	}
}
