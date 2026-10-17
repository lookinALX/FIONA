package tests

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lookinalx/fiona/internal/config"
	"github.com/lookinalx/fiona/internal/scanner/exiftest"
)

// checksums maps every file under root, FIONA's state left out, to its SHA-256.
func checksums(t *testing.T, root string) map[string]string {
	t.Helper()
	sums := make(map[string]string)
	for name, content := range snapshot(t, root) {
		sum := sha256.Sum256([]byte(content))
		sums[name] = hex.EncodeToString(sum[:])
	}
	return sums
}

func setMtime(t *testing.T, path string, at time.Time) {
	t.Helper()
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

func writeUserConfig(t *testing.T, content string) {
	t.Helper()
	createTestFile(t, filepath.Join(configDir(t), config.FileName), []byte(content))
}

// linesBefore returns the output lines before the one that contains marker.
func linesBefore(output, marker string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, marker) {
			break
		}
		lines = append(lines, line)
	}
	return lines
}

// A1, A2: a bare folder is sorted after one question and undone with fiona undo.
func TestA1_A2_BareFolderAndUndo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "photo.jpg"), []byte("photo"))
	createTestFile(t, filepath.Join(dir, "report.pdf"), []byte("report"))
	createTestFile(t, filepath.Join(dir, "song.mp3"), []byte("song"))
	createTestFile(t, filepath.Join(dir, "inbox", "keep.jpg"), []byte("stays in its folder"))
	before := checksums(t, dir)

	res := mustFiona(t, 0, "y\n", dir)

	assertOutput(t, res.stdout, `with preset "downloads"`, "Move 3 files", "? [y/N/l]", "✓ Moved 3 files", "Undo: fiona undo")
	assertSnapshot(t, dir, map[string]string{
		"images/photo.jpg":     "photo",
		"documents/report.pdf": "report",
		"audio/song.mp3":       "song",
		"inbox/keep.jpg":       "stays in its folder",
	})

	undo := mustFiona(t, 0, "y\n", "undo")

	assertOutput(t, undo.stdout, "Undo the run from today ", "(moved 3 files in ", "✓ Restored 3 files, removed 3 empty folders.")
	if got := checksums(t, dir); !equalMaps(got, before) {
		t.Errorf("after undo:\n got %v\nwant %v", got, before)
	}
}

func equalMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// A3, A4: categories by extension and, without one, by content.
func TestA3_A4_Categories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for _, name := range []string{"main.go", "notes.md", "IMG_0001.heic", "data.csv", "app.dmg"} {
		createTestFile(t, filepath.Join(dir, name), []byte{0, 1, 2, 3})
	}
	createTestFile(t, filepath.Join(dir, "picture"), []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"))

	mustFiona(t, 0, "", dir, "-y")

	assertSnapshot(t, dir, map[string]string{
		"code/main.go":         "\x00\x01\x02\x03",
		"documents/notes.md":   "\x00\x01\x02\x03",
		"images/IMG_0001.heic": "\x00\x01\x02\x03",
		"documents/data.csv":   "\x00\x01\x02\x03",
		"installers/app.dmg":   "\x00\x01\x02\x03",
		"images/picture":       "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR",
	})
}

// A5: extensions are lower case in folder names.
func TestA5_ExtensionCase(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "IMG.JPG"), []byte("upper"))
	createTestFile(t, filepath.Join(dir, "b.jpg"), []byte("lower"))

	res := mustFiona(t, 0, "", "sort", dir, "--to", "{ext}", "-y")

	assertOutput(t, res.stdout, `with --to "{ext}"`)
	assertSnapshot(t, dir, map[string]string{"jpg/IMG.JPG": "upper", "jpg/b.jpg": "lower"})
}

// A6: -c and -t become a template; month is a number.
func TestA6_CriteriaAsTemplate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "report.pdf")
	createTestFile(t, path, []byte("report"))
	setMtime(t, path, time.Date(2025, 1, 15, 12, 0, 0, 0, time.Local))

	res := mustFiona(t, 0, "", "sort", dir, "-c", "year", "-t", "month", "-y")

	assertOutput(t, res.stdout, "by year, then month")
	assertSnapshot(t, dir, map[string]string{"2025/01/report.pdf": "report"})
}

// A7: the photos preset uses the date taken and leaves other files alone.
func TestA7_PhotosPresetUsesDateTaken(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	photo := filepath.Join(dir, "IMG_0001.jpg")
	jpeg := exiftest.JPEG(time.Date(2019, 6, 15, 10, 30, 0, 0, time.Local))
	createTestFile(t, photo, jpeg)
	setMtime(t, photo, time.Date(2026, 3, 1, 12, 0, 0, 0, time.Local))
	createTestFile(t, filepath.Join(dir, "report.pdf"), []byte("report"))

	res := mustFiona(t, 0, "", "--preset", "photos", dir, "-y")

	assertOutput(t, res.stdout, `with preset "photos"`, "2019/", "Left as is: 1 file that matches no rule")
	assertSnapshot(t, dir, map[string]string{
		"2019/06/IMG_0001.jpg": string(jpeg),
		"report.pdf":           "report",
	})
}

// A8: a preset of the config, each file by the first rule that matches.
func TestA8_UserPreset(t *testing.T) {
	t.Parallel()

	writeUserConfig(t, `{
		"default_preset": "work",
		"presets": {
			"work": {
				"description": "Invoices by year, photos by month, the rest by type",
				"rules": [
					{ "match": { "name": "invoice*", "ext": ["pdf"] }, "to": "invoices/{year}" },
					{ "match": { "category": ["images"] }, "to": "photos/{year}/{month}" },
					{ "to": "{category}" }
				]
			}
		}
	}`)

	dir := t.TempDir()
	march := time.Date(2025, 3, 2, 12, 0, 0, 0, time.Local)
	for name, content := range map[string]string{"Invoice-42.PDF": "invoice", "report.pdf": "report", "cat.png": "cat", "main.go": "code"} {
		path := filepath.Join(dir, name)
		createTestFile(t, path, []byte(content))
		setMtime(t, path, march)
	}

	res := mustFiona(t, 0, "", dir, "-y")

	assertOutput(t, res.stdout, `with preset "work" (default from config)`)
	assertSnapshot(t, dir, map[string]string{
		"invoices/2025/Invoice-42.PDF": "invoice",
		"documents/report.pdf":         "report",
		"photos/2025/03/cat.png":       "cat",
		"code/main.go":                 "code",
	})

	presets := mustFiona(t, 0, "", "presets")
	assertOutput(t, presets.stdout, "work", "Invoices by year", "default, config")
}

// A9: a typo in a preset is reported with the preset and the rule, nothing is touched.
func TestA9_BrokenPreset(t *testing.T) {
	t.Parallel()

	writeUserConfig(t, `{"presets": {"work": {"rules": [{"to": "{category}"}, {"to": "{yaer}"}]}}}`)
	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "photo.jpg"), []byte("photo"))

	res := mustFiona(t, 2, "", dir, "-y")

	assertOutput(t, res.stderr, `preset "work", rule 2: unknown placeholder {yaer}. Did you mean {year}?`, config.FileName)
	assertSnapshot(t, dir, map[string]string{"photo.jpg": "photo"})
}

// A10: flags after the positional argument are parsed.
func TestA10_FlagsAfterFolder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "sub", "photo.jpg"), []byte("photo"))

	mustFiona(t, 0, "", "sort", dir, "-r", "--copy", "-y")

	assertSnapshot(t, dir, map[string]string{"sub/photo.jpg": "photo", "images/photo.jpg": "photo"})
}

// A14, A15: fiona undo works like Ctrl+Z, fiona history shows the result.
func TestA14_A15_UndoTwiceAndHistory(t *testing.T) {
	t.Parallel()

	first, second := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(first, "a.jpg"), []byte("a"))
	createTestFile(t, filepath.Join(second, "b.pdf"), []byte("b"))

	mustFiona(t, 0, "", first, "-y")
	// run identifiers and history entries have a resolution of one second
	time.Sleep(1100 * time.Millisecond)
	mustFiona(t, 0, "", second, "-y")

	mustFiona(t, 0, "", "undo", "-y")
	assertSnapshot(t, second, map[string]string{"b.pdf": "b"})
	assertSnapshot(t, first, map[string]string{"images/a.jpg": "a"})

	mustFiona(t, 0, "", "undo", "-y")
	assertSnapshot(t, first, map[string]string{"a.jpg": "a"})

	res := mustFiona(t, 0, "", "history")
	lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
	if len(lines) != 3 {
		t.Fatalf("history:\n%s", res.stdout)
	}
	for _, line := range lines[1:] {
		if !strings.HasSuffix(line, "undone") {
			t.Errorf("run is not undone in the history: %q", line)
		}
	}
}

// undo by a prefix of the run id
func TestUndo_ByRunID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "a.jpg"), []byte("a"))
	mustFiona(t, 0, "", dir, "-y")
	run := lastRun(t, dir)

	mustFiona(t, 0, "", "undo", run.Header.RunID[:17], "-y")
	assertSnapshot(t, dir, map[string]string{"a.jpg": "a"})

	res := mustFiona(t, 1, "", "undo", run.Header.RunID, "-y")
	assertOutput(t, res.stderr, "already undone")
}

// A17: the overview fits on a screen, whatever the number of files.
func TestA17_CompactOverview(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for i := 0; i < 300; i++ {
		createTestFile(t, filepath.Join(dir, fmt.Sprintf("photo%03d.jpg", i)), []byte("p"))
	}
	for i := 0; i < 50; i++ {
		createTestFile(t, filepath.Join(dir, fmt.Sprintf("doc%02d.pdf", i)), []byte("d"))
	}

	res := mustFiona(t, 0, "n\n", dir)

	overview := linesBefore(res.stdout, "? [y/N/l]")
	if len(overview) > 10 {
		t.Errorf("overview has %d lines:\n%s", len(overview), strings.Join(overview, "\n"))
	}
	if strings.Contains(res.stdout, "photo001.jpg") {
		t.Errorf("files are listed without --list:\n%s", res.stdout)
	}
	assertOutput(t, res.stdout, "images/", "300 files", "documents/", "50 files", "Move 350 files")
}

// A18: every file with --list, or after the answer l.
func TestA18_List(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "photo.jpg"), []byte("photo"))
	createTestFile(t, filepath.Join(dir, "report.pdf"), []byte("report"))

	res := mustFiona(t, 0, "", dir, "--list", "-n")
	assertOutput(t, res.stdout, "photo.jpg → images/photo.jpg", "report.pdf → documents/report.pdf")

	res = mustFiona(t, 0, "l\nn\n", dir)
	if n := strings.Count(res.stdout, "? [y/N/l]"); n != 2 {
		t.Errorf("question asked %d times, want 2:\n%s", n, res.stdout)
	}
	assertOutput(t, res.stdout, "photo.jpg → images/photo.jpg", "Cancelled: nothing is changed.")
	assertSnapshot(t, dir, map[string]string{"photo.jpg": "photo", "report.pdf": "report"})
}

// A19: no zero counters, the result is one line.
func TestA19_NoZeroCounters(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "photo.jpg"), []byte("photo"))

	res := mustFiona(t, 0, "", dir, "-y")

	for _, line := range strings.Split(res.stdout, "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), ": 0") {
			t.Errorf("zero counter: %q", line)
		}
	}
	result := strings.Split(strings.TrimSpace(res.stdout), "\n")
	if last := result[len(result)-1]; !strings.HasPrefix(last, "✓ Moved 1 file in ") {
		t.Errorf("last line = %q", last)
	}
}

// A20: a folder that differs only in case is reused.
func TestA20_ExistingFolderInAnotherCase(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "Images", "old.jpg"), []byte("old"))
	createTestFile(t, filepath.Join(dir, "photo.jpg"), []byte("photo"))

	res := mustFiona(t, 0, "", dir, "-y")

	assertOutput(t, res.stdout, "Images/", "exists")
	assertSnapshot(t, dir, map[string]string{"Images/old.jpg": "old", "Images/photo.jpg": "photo"})
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == "images" {
			t.Error("a second folder images/ was created")
		}
	}
}

// A21: nothing to undo is not an error and names the last run.
func TestA21_NothingToUndo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "photo.jpg"), []byte("photo"))
	mustFiona(t, 0, "", dir, "-y")
	mustFiona(t, 0, "", "undo", "-y")

	res := mustFiona(t, 0, "", "undo", "-y")
	assertOutput(t, res.stdout, "Nothing to undo: the last run in ", "(today ", "is already undone. See fiona history.")
}

// A22: a typo in the preset gets a suggestion and exit code 2.
func TestA22_PresetTypo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "photo.jpg"), []byte("photo"))

	res := mustFiona(t, 2, "", "-p", "photso", dir)

	assertOutput(t, res.stderr, `fiona: unknown preset "photso". Did you mean "photos"?`, "Available: downloads, photos, by-date, by-type")
	assertSnapshot(t, dir, map[string]string{"photo.jpg": "photo"})
}

// A23: output to a pipe has no colours and no progress bar.
func TestA23_PlainOutputToPipe(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for i := 0; i < 50; i++ {
		createTestFile(t, filepath.Join(dir, fmt.Sprintf("f%02d.jpg", i)), []byte("x"))
	}

	res := mustFiona(t, 0, "", dir, "-y")

	if strings.Contains(res.stdout, "\x1b[") || strings.Contains(res.stdout, "\r") {
		t.Errorf("escape codes in piped output: %q", res.stdout)
	}
}

func TestUnknownCommand(t *testing.T) {
	t.Parallel()

	res := mustFiona(t, 2, "", "srot")
	assertOutput(t, res.stderr, `unknown command or folder "srot". Did you mean "sort"?`)

	// without arguments and without a terminal: the short help
	res = mustFiona(t, 2, "")
	assertOutput(t, res.stderr, "fiona sort [FLAGS] [SOURCE] [DEST]")
}

func TestHelpIsGenerated(t *testing.T) {
	t.Parallel()

	res := mustFiona(t, 0, "", "--help")
	assertOutput(t, res.stdout, "--preset", "--to", "--ml-config", "--list", "{year}", "installers", "fiona tui", "Exit codes")
}

func TestPresetsShow(t *testing.T) {
	t.Parallel()

	res := mustFiona(t, 0, "", "presets", "show", "photos")
	assertOutput(t, res.stdout, `"presets"`, `"photos"`, `"to": "{year}/{month}"`)

	res = mustFiona(t, 2, "", "presets", "show", "photso")
	assertOutput(t, res.stderr, `Did you mean "photos"?`)
}
