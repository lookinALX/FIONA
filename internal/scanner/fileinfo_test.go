package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefineType(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".jpg", "images"},
		{"PNG", "images"},
		{".pdf", "documents"},
		{"docx", "documents"},
		{".mp4", "videos"},
		{"wav", "audios"},
		{"exe", "applications"},
		{"zip", "archives"},
		{".unknown", "unknown"},
	}

	for _, test := range tests {
		got := defineType(test.ext)
		if got != test.expected {
			t.Errorf("defineMIMEtype(%q) = %q; want %q", test.ext, got, test.expected)
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

	if fi.Extension != ".txt" {
		t.Errorf("Extension = %q; want %q", fi.Extension, ".txt")
	}

	if fi.Size != 5 {
		t.Errorf("Size = %d; want %d", fi.Size, 5)
	}

	if fi.Type != "documents" {
		t.Errorf("Type = %q; want %q", fi.Type, "documents")
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
