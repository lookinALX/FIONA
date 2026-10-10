package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	images = map[string]bool{
		"jpg": true, "jpeg": true, "png": true, "gif": true, "bmp": true, "svg": true, "webp": true,
	}
	documents = map[string]bool{
		"txt": true, "pdf": true, "doc": true, "docx": true, "xls": true, "xlsx": true, "ppt": true, "pptx": true,
	}
	videos = map[string]bool{
		"mp4": true, "avi": true, "mkv": true, "mov": true, "flv": true, "webm": true,
	}
	audio = map[string]bool{
		"mp3": true, "wav": true, "flac": true, "aac": true, "ogg": true,
	}
	apps = map[string]bool{
		"exe": true, "msi": true, "apk": true, "deb": true, "rpm": true,
	}
	archives = map[string]bool{
		"zip": true, "rar": true, "7z": true, "tar": true, "gz": true,
	}
)

type FileInfo struct {
	Path      string
	Name      string
	Extension string
	Size      int64
	ModTime   time.Time
	Date      string
	Type      string
	Tag       string
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
	extension := filepath.Ext(name)

	return &FileInfo{
		Path:      path,
		Name:      name,
		Extension: extension,
		Size:      info.Size(),
		ModTime:   info.ModTime(),
		Date:      info.ModTime().Format("January 2006"),
		Type:      defineType(extension),
	}
}

func defineType(extension string) string {
	ext := strings.ToLower(extension)
	if len(ext) > 0 && ext[0] == '.' {
		ext = ext[1:]
	}

	switch {
	case images[ext]:
		return "images"
	case documents[ext]:
		return "documents"
	case videos[ext]:
		return "videos"
	case audio[ext]:
		return "audios"
	case apps[ext]:
		return "applications"
	case archives[ext]:
		return "archives"
	default:
		return "unknown"
	}
}
