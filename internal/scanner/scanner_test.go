package scanner

import (
	"os"
	"path/filepath"
	"sort"
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

// scanNames returns scanned file paths relative to the root, sorted.
func scanNames(t *testing.T, root string, opt Options) ([]string, *Result) {
	t.Helper()

	res, err := Scan(root, opt)
	if err != nil {
		t.Fatalf("something went wrong: %v", err)
	}

	names := make([]string, 0, len(res.Files))
	for _, f := range res.Files {
		rel, err := filepath.Rel(root, f.Path)
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, filepath.ToSlash(rel))
	}
	sort.Strings(names)

	return names, res
}

func assertNames(t *testing.T, got []string, want ...string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("scanned %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("scanned %v, want %v", got, want)
		}
	}
}

func TestScan(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*TestFileSystem)
		opt       Options
		wantFiles []string
	}{
		{
			name:      "empty directory",
			setup:     func(*TestFileSystem) {},
			wantFiles: nil,
		},
		{
			name: "flat directory with files",
			setup: func(tfs *TestFileSystem) {
				tfs.CreateFile("file1.txt", "content")
				tfs.CreateFile("file2.jpg", "image")
				tfs.CreateFile("file3.pdf", "doc")
			},
			wantFiles: []string{"file1.txt", "file2.jpg", "file3.pdf"},
		},
		{
			name: "nested directories are not scanned by default",
			setup: func(tfs *TestFileSystem) {
				tfs.CreateFile("file.txt", "content")
				tfs.CreateFile("folder1/nestedFile.txt", "nested1")
				tfs.CreateFile("folder1/folder2/deep.txt", "deep")
			},
			wantFiles: []string{"file.txt"},
		},
		{
			name: "nested directories with recursive",
			setup: func(tfs *TestFileSystem) {
				tfs.CreateFile("file.txt", "content")
				tfs.CreateFile("folder1/nestedFile.txt", "nested1")
				tfs.CreateFile("folder1/folder2/deep.txt", "deep")
			},
			opt:       Options{Recursive: true},
			wantFiles: []string{"file.txt", "folder1/folder2/deep.txt", "folder1/nestedFile.txt"},
		},
		{
			name: "mixed content",
			setup: func(tfs *TestFileSystem) {
				tfs.CreateFile("file.txt", "content")
				tfs.CreateDir("empty_folder")
				tfs.CreateFile("docs/report.pdf", "report")
				tfs.CreateFile("images/photo.jpg", "photo")
			},
			opt:       Options{Recursive: true},
			wantFiles: []string{"docs/report.pdf", "file.txt", "images/photo.jpg"},
		},
		{
			name: "hidden files and directories are skipped by default",
			setup: func(tfs *TestFileSystem) {
				tfs.CreateFile("file.txt", "content")
				tfs.CreateFile(".hidden", "secret")
				tfs.CreateFile(".config/settings.json", "{}")
			},
			opt:       Options{Recursive: true},
			wantFiles: []string{"file.txt"},
		},
		{
			name: "hidden files and directories with hidden",
			setup: func(tfs *TestFileSystem) {
				tfs.CreateFile("file.txt", "content")
				tfs.CreateFile(".hidden", "secret")
				tfs.CreateFile(".config/settings.json", "{}")
			},
			opt:       Options{Recursive: true, IncludeHidden: true},
			wantFiles: []string{".config/settings.json", ".hidden", "file.txt"},
		},
		{
			name: "fiona state directory is always excluded",
			setup: func(tfs *TestFileSystem) {
				tfs.CreateFile("file.txt", "content")
				tfs.CreateFile(".fiona/runs/run.jsonl", "{}")
				tfs.CreateFile(".fiona/trash/run/old.txt", "old")
			},
			opt:       Options{Recursive: true, IncludeHidden: true},
			wantFiles: []string{"file.txt"},
		},
		{
			name: "bundles are opaque",
			setup: func(tfs *TestFileSystem) {
				tfs.CreateFile("file.txt", "content")
				tfs.CreateFile("Foo.app/Contents/Info.plist", "plist")
				tfs.CreateFile("Lib.framework/Lib", "bin")
				tfs.CreateFile("Res.bundle/res.png", "png")
				tfs.CreateFile("Photos.photoslibrary/db", "db")
				tfs.CreateFile("App.xcodeproj/project.pbxproj", "proj")
			},
			opt:       Options{Recursive: true},
			wantFiles: []string{"file.txt"},
		},
		{
			name: "projects are opaque",
			setup: func(tfs *TestFileSystem) {
				tfs.CreateFile("file.txt", "content")
				tfs.CreateFile("gitrepo/.git/HEAD", "ref")
				tfs.CreateFile("gitrepo/main.c", "code")
				tfs.CreateFile("hgrepo/.hg/store", "store")
				tfs.CreateFile("hgrepo/readme.txt", "readme")
				tfs.CreateFile("svnrepo/.svn/entries", "entries")
				tfs.CreateFile("svnrepo/readme.txt", "readme")
				tfs.CreateFile("gomodule/go.mod", "module x")
				tfs.CreateFile("gomodule/main.go", "package main")
				tfs.CreateFile("jsapp/package.json", "{}")
				tfs.CreateFile("jsapp/index.js", "code")
				tfs.CreateFile("built/node_modules/dep/index.js", "code")
				tfs.CreateFile("built/bundle.js", "code")
				tfs.CreateFile("plain/nested/gomodule/go.mod", "module y")
				tfs.CreateFile("plain/nested/notes.txt", "notes")
			},
			opt:       Options{Recursive: true, IncludeHidden: true},
			wantFiles: []string{"file.txt", "plain/nested/notes.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tfs := NewTestFileSystem(t)
			tt.setup(tfs)

			names, res := scanNames(t, tfs.Root, tt.opt)

			assertNames(t, names, tt.wantFiles...)
			if len(res.Skipped) != 0 {
				t.Errorf("unexpected skipped entries: %v", res.Skipped)
			}
		})
	}
}

func TestScan_ExcludesDest(t *testing.T) {
	tfs := NewTestFileSystem(t)
	tfs.CreateFile("file.txt", "content")
	tfs.CreateFile("sub/other.txt", "content")
	dest := tfs.CreateDir("sorted")
	tfs.CreateFile("sorted/documents/old.txt", "already sorted")

	names, _ := scanNames(t, tfs.Root, Options{Recursive: true, Exclude: []string{dest}})

	assertNames(t, names, "file.txt", "sub/other.txt")
}

func TestScan_SymlinksAreNotFollowed(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	tfs := NewTestFileSystem(t)
	tfs.CreateFile("file.txt", "content")
	links := map[string]string{
		"to_file":    secret,
		"to_dir":     outside,
		"to_nowhere": filepath.Join(outside, "missing"),
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(tfs.Root, name)); err != nil {
			t.Skipf("cannot create symlink: %v", err)
		}
	}

	names, res := scanNames(t, tfs.Root, Options{Recursive: true})

	assertNames(t, names, "file.txt")
	if len(res.Skipped) != len(links) {
		t.Fatalf("skipped %v, want %d symlinks", res.Skipped, len(links))
	}
	for _, entry := range res.Skipped {
		if entry.Reason != ReasonSymlink {
			t.Errorf("%s: reason = %q, want %q", entry.Path, entry.Reason, ReasonSymlink)
		}
		if _, ok := links[filepath.Base(entry.Path)]; !ok {
			t.Errorf("unexpected skipped entry %s", entry.Path)
		}
	}
}

func TestScan_UnreadableSubdirectoryIsNotFatal(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can read any directory")
	}

	tfs := NewTestFileSystem(t)
	tfs.CreateFile("file.txt", "content")
	tfs.CreateFile("open/other.txt", "content")
	locked := tfs.CreateDir("locked")
	tfs.CreateFile("locked/inside.txt", "content")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0755)

	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("file system does not enforce directory permissions")
	}

	names, res := scanNames(t, tfs.Root, Options{Recursive: true})

	assertNames(t, names, "file.txt", "open/other.txt")
	if len(res.Skipped) != 1 || res.Skipped[0].Path != locked || res.Skipped[0].Reason != ReasonNoAccess {
		t.Errorf("skipped = %v, want %s with reason %q", res.Skipped, locked, ReasonNoAccess)
	}
}

func TestScan_RootErrorIsFatal(t *testing.T) {
	if _, err := Scan(filepath.Join(t.TempDir(), "missing"), Options{}); err == nil {
		t.Error("expected error for a root that does not exist")
	}
}

func TestScan_FillsFileInfo(t *testing.T) {
	tfs := NewTestFileSystem(t)
	path := tfs.CreateFile("photo.jpg", "12345")

	res, err := Scan(tfs.Root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 1 {
		t.Fatalf("scanned %d files, want 1", len(res.Files))
	}

	fi := res.Files[0]
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}

	if fi.Path != path || fi.Name != "photo.jpg" || fi.Extension != ".jpg" || fi.Size != 5 || fi.Type != "images" {
		t.Errorf("unexpected file info: %+v", fi)
	}
	if !fi.ModTime.Equal(info.ModTime()) {
		t.Errorf("ModTime = %v, want %v", fi.ModTime, info.ModTime())
	}
}
