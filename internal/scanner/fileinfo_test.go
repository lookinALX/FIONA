package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCategoryByExtension(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"a.jpg", "images"},
		{"A.PNG", "images"},
		{"IMG_0001.HEIC", "images"},
		{"raw.cr2", "images"},
		{"report.pdf", "documents"},
		{"notes.md", "documents"},
		{"data.csv", "documents"},
		{"movie.mkv", "videos"},
		{"song.wav", "audio"},
		{"setup.exe", "installers"},
		{"app.dmg", "installers"},
		{"main.go", "code"},
		{"backup.tar.gz", "archives"},
		{"thing.xyz", "other"},
	}

	dir := t.TempDir()
	for _, tt := range tests {
		// content that says nothing, so that only the extension decides
		path := filepath.Join(dir, tt.name)
		if err := os.WriteFile(path, []byte{0x00, 0x01, 0x02, 0x03}, 0644); err != nil {
			t.Fatal(err)
		}
		fi, err := NewFileInfo(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fi.Category(); got != tt.want {
			t.Errorf("Category(%s) = %q; want %q", tt.name, got, tt.want)
		}
	}
}

func TestCategoryByContent(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	tests := []struct {
		name    string
		content []byte
		want    string
	}{
		{"noext", png, "images"},
		{"picture.unknown", png, "images"},
		{"README", []byte("Read me first\n"), "documents"},
		{"doc", []byte("%PDF-1.7\n"), "documents"},
		{"bundle", []byte("PK\x03\x04\x14\x00"), "archives"},
		{"bundle7", []byte("7z\xBC\xAF\x27\x1C\x00\x04"), "archives"},
		{"blob", []byte{0x00, 0x01, 0x02, 0xFE}, "other"},
		{"empty", nil, "other"},
	}

	dir := t.TempDir()
	for _, tt := range tests {
		path := filepath.Join(dir, tt.name)
		if err := os.WriteFile(path, tt.content, 0644); err != nil {
			t.Fatal(err)
		}
		fi, err := NewFileInfo(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fi.Category(); got != tt.want {
			t.Errorf("Category(%s) = %q; want %q", tt.name, got, tt.want)
		}
	}
}

func TestExtension(t *testing.T) {
	tests := map[string]string{
		"a.JPG":        "jpg",
		"b.tar.gz":     "gz",
		"README":       "",
		".bashrc":      "",
		".config.json": "json",
		"trailing.":    "",
	}
	for name, want := range tests {
		if got := extension(name); got != want {
			t.Errorf("extension(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestNewFileInfo(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "testfile.txt")
	err := os.WriteFile(tmpFile, []byte("Hello"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	fi, err := NewFileInfo(tmpFile)
	if err != nil {
		t.Fatalf("NewFileInfo returned error: %v", err)
	}

	if fi.Name != "testfile.txt" {
		t.Errorf("Name = %q; want %q", fi.Name, "testfile.txt")
	}

	if fi.Ext != "txt" {
		t.Errorf("Ext = %q; want %q", fi.Ext, "txt")
	}

	if fi.Size != 5 {
		t.Errorf("Size = %d; want %d", fi.Size, 5)
	}

	if fi.Category() != "documents" {
		t.Errorf("Category = %q; want %q", fi.Category(), "documents")
	}

	if fi.ModTime.IsZero() {
		t.Errorf("ModTime is zero")
	}
}

func TestNewFileInfoDirError(t *testing.T) {
	_, err := NewFileInfo(t.TempDir())
	if err == nil {
		t.Errorf("expected error for directory, got nil")
	}
}

func TestNewFileInfoSymlinkError(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("Hello"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	if _, err := NewFileInfo(link); err == nil {
		t.Errorf("expected error for symlink, got nil")
	}
}
