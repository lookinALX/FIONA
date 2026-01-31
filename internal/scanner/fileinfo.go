package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	Date      string
	Type      string
}

func NewFileInfo(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path is directory, not a file: %s", path)
	}

	name := info.Name()
	extension := filepath.Ext(name)
	size := info.Size()
	date := info.ModTime().Format("January 2006")
	mimeType := defineType(extension)

	return &FileInfo{
		Path:      path,
		Name:      name,
		Extension: extension,
		Size:      size,
		Date:      date,
		Type:      mimeType,
	}, nil
}

func defineType(extension string) string {
	ext := strings.ToLower(extension)
	if len(ext) > 0 && ext[0] == '.' {
		ext = ext[1:]
	}

	switch {
	case images[ext]:
		return "image"
	case documents[ext]:
		return "document"
	case videos[ext]:
		return "video"
	case audio[ext]:
		return "audio"
	case apps[ext]:
		return "application"
	case archives[ext]:
		return "archive"
	default:
		return "unknown"
	}
}
