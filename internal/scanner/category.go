package scanner

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	CategoryImages     = "images"
	CategoryVideos     = "videos"
	CategoryAudio      = "audio"
	CategoryDocuments  = "documents"
	CategoryArchives   = "archives"
	CategoryInstallers = "installers"
	CategoryCode       = "code"
	CategoryOther      = "other"
)

// Categories lists every category in the order they are shown to the user.
var Categories = []string{
	CategoryImages, CategoryVideos, CategoryAudio, CategoryDocuments,
	CategoryArchives, CategoryInstallers, CategoryCode, CategoryOther,
}

// CategoryExtensions maps every category but "other" to its extensions.
var CategoryExtensions = map[string][]string{
	CategoryImages:     {"jpg", "jpeg", "png", "gif", "bmp", "webp", "svg", "heic", "heif", "avif", "tif", "tiff", "ico", "psd", "cr2", "cr3", "nef", "arw", "dng", "raf", "orf", "rw2"},
	CategoryVideos:     {"mp4", "mov", "avi", "mkv", "webm", "flv", "wmv", "m4v", "mpg", "mpeg", "3gp", "mts"},
	CategoryAudio:      {"mp3", "wav", "flac", "aac", "ogg", "m4a", "opus", "wma", "aiff"},
	CategoryDocuments:  {"pdf", "doc", "docx", "odt", "rtf", "txt", "md", "pages", "xls", "xlsx", "ods", "csv", "numbers", "ppt", "pptx", "odp", "key", "epub", "fb2", "mobi", "djvu"},
	CategoryArchives:   {"zip", "rar", "7z", "tar", "gz", "tgz", "bz2", "xz", "zst"},
	CategoryInstallers: {"dmg", "pkg", "exe", "msi", "deb", "rpm", "apk", "appimage", "iso"},
	CategoryCode:       {"go", "py", "js", "ts", "jsx", "tsx", "java", "c", "h", "cpp", "hpp", "rs", "rb", "php", "sh", "html", "css", "json", "xml", "yaml", "yml", "toml", "sql", "ipynb"},
}

var categoryOfExt = func() map[string]string {
	m := make(map[string]string)
	for category, exts := range CategoryExtensions {
		for _, ext := range exts {
			m[ext] = category
		}
	}
	return m
}()

// sniffLen is how much of a file http.DetectContentType looks at.
const sniffLen = 512

// categoryByExt expects the extension in lower case, without the dot.
func categoryByExt(ext string) string {
	if category, ok := categoryOfExt[ext]; ok {
		return category
	}
	return CategoryOther
}

// categoryByContent looks at the first bytes of a file whose extension says nothing.
func categoryByContent(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return CategoryOther
	}
	defer f.Close()

	head := make([]byte, sniffLen)
	n, err := io.ReadFull(f, head)
	if err != nil && n == 0 {
		return CategoryOther
	}
	return categoryBySniff(head[:n])
}

func categoryBySniff(head []byte) string {
	// not known to http.DetectContentType
	if bytes.HasPrefix(head, []byte("7z\xBC\xAF\x27\x1C")) {
		return CategoryArchives
	}

	mime := http.DetectContentType(head)
	if i := strings.IndexByte(mime, ';'); i >= 0 {
		mime = mime[:i]
	}

	switch {
	case strings.HasPrefix(mime, "image/"):
		return CategoryImages
	case strings.HasPrefix(mime, "video/"):
		return CategoryVideos
	case strings.HasPrefix(mime, "audio/"), mime == "application/ogg":
		return CategoryAudio
	case mime == "application/pdf", mime == "application/postscript", strings.HasPrefix(mime, "text/"):
		return CategoryDocuments
	case mime == "application/zip", mime == "application/x-gzip", mime == "application/x-rar-compressed":
		return CategoryArchives
	default:
		return CategoryOther
	}
}
