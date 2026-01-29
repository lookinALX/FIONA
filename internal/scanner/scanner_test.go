package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

type TestFileSystem struct {
	Root string
	t    *testing.T
}

func NewTestFileSystem(t *testing.T) *TestFileSystem {
	return &TestFileSystem{
		Root: t.TempDir(),
		t:    t,
	}
}

func (tfs *TestFileSystem) CreateFile(relativePath, content string) string {
	fullPath := filepath.Join(tfs.Root, relativePath)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		tfs.t.Fatalf("cannot create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		tfs.t.Fatalf("cannot create file: %v", err)
	}

	return fullPath
}

func (tfs *TestFileSystem) CreateDir(relativePath string) string {
	fullPath := filepath.Join(tfs.Root, relativePath)

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		tfs.t.Fatalf("cannot create directory %s: %v", fullPath, err)
	}

	return fullPath
}

func TestScan(t *testing.T) {
	tests := []struct {
		name               string
		setup              func(*TestFileSystem)
		expectedCountFiles int
		expectedCountDirs  int
	}{
		{
			name: "empty directory",
			setup: func(*TestFileSystem) {

			},
			expectedCountFiles: 0,
			expectedCountDirs:  1,
		},
		{
			name: "flat directory with files",
			setup: func(tfs *TestFileSystem) {
				tfs.CreateFile("file1.txt", "content")
				tfs.CreateFile("file2.jpg", "image")
				tfs.CreateFile("file3.pdf", "doc")
			},
			expectedCountFiles: 3,
			expectedCountDirs:  1,
		},
		{
			name: "nested directories",
			setup: func(tfs *TestFileSystem) {
				tfs.CreateFile("file.txt", "content")
				tfs.CreateFile("folder1/nestedFile.txt", "nested1")
				tfs.CreateFile("folder1/folder2/deep.txt", "deep")
			},
			expectedCountFiles: 3,
			expectedCountDirs:  3,
		},
		{
			name: "mixed content",
			setup: func(tfs *TestFileSystem) {
				tfs.CreateFile("file.txt", "content")
				tfs.CreateDir("empty_folder")
				tfs.CreateFile("docs/report.pdf", "report")
				tfs.CreateFile("images/photo.jpg", "photo")
			},
			expectedCountFiles: 3,
			expectedCountDirs:  4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tfs := NewTestFileSystem(t)
			tt.setup(tfs)

			scanner := NewScanner()
			files, err := scanner.Scan(tfs.Root)

			if err != nil {
				t.Fatalf("something went wrong: %v", err)
			}

			if len := len(files); len < tt.expectedCountFiles {
				t.Errorf("recieved file list with length less then %d", tt.expectedCountFiles)
			}

			if scanner.filesScanned != tt.expectedCountFiles {
				t.Errorf("scanner found %d files, expected %d", scanner.filesScanned, tt.expectedCountFiles)
			}

			if scanner.dirsScanned != tt.expectedCountDirs {
				t.Errorf("scanner found %d directories, expected %d", scanner.dirsScanned, tt.expectedCountDirs)
			}
		})
	}
}
