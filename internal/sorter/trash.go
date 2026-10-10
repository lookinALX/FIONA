package sorter

import (
	"os"
	"path/filepath"
	"time"

	"github.com/lookinalx/fiona/internal/journal"
)

// TrashRetention is how long files displaced by the replace strategy are kept.
const TrashRetention = 5 * 24 * time.Hour

// CleanTrash removes trash folders of runs older than TrashRetention and
// returns how many were removed. Anything that is not a run folder is left alone.
func CleanTrash(dest string, now time.Time) (int, error) {
	stateDir := filepath.Join(dest, journal.StateDir)
	trashDir := filepath.Join(stateDir, journal.TrashDir)

	for _, dir := range []string{stateDir, trashDir} {
		if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
			return 0, nil
		}
	}

	entries, err := os.ReadDir(trashDir)
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		started, ok := journal.RunTime(entry.Name())
		if !ok || now.Sub(started) <= TrashRetention {
			continue
		}
		if err := os.RemoveAll(filepath.Join(trashDir, entry.Name())); err != nil {
			return removed, err
		}
		removed++
	}

	return removed, nil
}
