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
	tmpFile := filepath.Join(os.TempDir(), "testfile.txt")
	err := os.WriteFile(tmpFile, []byte("Hello"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile)

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
}

func TestNewFileInfoDirError(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "testdir")
	err := os.Mkdir(tmpDir, 0755)
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.Remove(tmpDir)

	_, err = NewFileInfo(tmpDir)
	if err == nil {
		t.Errorf("expected error for directory, got nil")
	}
}
