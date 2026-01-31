package scanner

import (
	"fmt"
	"io/fs"
	"path/filepath"
)

type Scanner struct {
	filesScanned int
	dirsScanned  int
}

func NewScanner() Scanner {
	return Scanner{filesScanned: 0, dirsScanned: 0}
}

func (s *Scanner) Scan(dir string) ([]*FileInfo, error) {
	var files []*FileInfo
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			s.dirsScanned++
			return nil
		}

		file, err := NewFileInfo(path)
		if err != nil {
			return fmt.Errorf("failed to create FileInfo for %s: %w", path, err)
		}

		files = append(files, file)
		s.filesScanned++

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("scanner: can not scan directory: %w", err)
	}

	return files, nil
}
