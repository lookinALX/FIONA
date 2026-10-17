package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FileInfo struct {
	Path    string
	Name    string
	Ext     string // lower case, without the dot; empty if there is none
	Size    int64
	ModTime time.Time
	Tag     string // set by the ML classifier

	categoryOnce sync.Once
	category     string
	takenOnce    sync.Once
	taken        *time.Time
}

func NewFileInfo(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path is directory, not a file: %s", path)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("path is not a regular file: %s", path)
	}

	return newFileInfo(path, info), nil
}

func newFileInfo(path string, info os.FileInfo) *FileInfo {
	name := info.Name()
	return &FileInfo{
		Path:    path,
		Name:    name,
		Ext:     extension(name),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}
}

// extension returns the extension of name in lower case without the dot.
// A name like ".bashrc" has none.
func extension(name string) string {
	ext := filepath.Ext(name)
	if ext == name {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}

// Category is decided by the extension; the content is read only when the
// extension is missing or unknown. The result is cached.
func (fi *FileInfo) Category() string {
	fi.categoryOnce.Do(func() {
		fi.category = categoryByExt(fi.Ext)
		if fi.category == CategoryOther && fi.Size > 0 {
			fi.category = categoryByContent(fi.Path)
		}
	})
	return fi.category
}

// Taken is the date the photo was taken, read from EXIF. Only images are
// read, and only once; nil if the date is unknown.
func (fi *FileInfo) Taken() *time.Time {
	fi.takenOnce.Do(func() {
		if fi.Category() != CategoryImages {
			return
		}
		if t, ok := readTaken(fi.Path); ok {
			fi.taken = &t
		}
	})
	return fi.taken
}

// Date is what date placeholders use: the date taken if known, otherwise the
// modification time. Both are in local time.
func (fi *FileInfo) Date() time.Time {
	if taken := fi.Taken(); taken != nil {
		return taken.Local()
	}
	return fi.ModTime.Local()
}
