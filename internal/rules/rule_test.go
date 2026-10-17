package rules

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lookinalx/fiona/internal/scanner"
)

// file creates a real file, so that the category and dates come from the disk.
func file(t *testing.T, name string, size int, mtime time.Time) *scanner.FileInfo {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, make([]byte, size), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	fi, err := scanner.NewFileInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi
}

func TestTemplate_Render(t *testing.T) {
	mtime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.Local)
	photo := file(t, "IMG.JPG", 2048, mtime)
	photo.Tag = "cats"
	readme := file(t, "README", 0, mtime)

	tests := []struct {
		template string
		fi       *scanner.FileInfo
		want     []string
	}{
		{"{category}", photo, []string{"images"}},
		{"{ext}", photo, []string{"jpg"}},
		{"{ext}", readme, []string{"_no_ext"}},
		{"{year}/{month}", photo, []string{"2025", "01"}},
		{"{date}", photo, []string{"2025-01-15"}},
		{"{day}", photo, []string{"15"}},
		{"{size}", photo, []string{"tiny"}},
		{"{ml}", photo, []string{"cats"}},
		{"{ml}", readme, []string{"other"}},
		{"photos/{year}/{month}", photo, []string{"photos", "2025", "01"}},
		{"{year}-{month} trip", photo, []string{"2025-01 trip"}},
	}

	for _, tt := range tests {
		tmpl, err := ParseTemplate(tt.template)
		if err != nil {
			t.Fatalf("ParseTemplate(%q): %v", tt.template, err)
		}
		if got := tmpl.Render(tt.fi); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q for %s = %v, want %v", tt.template, tt.fi.Name, got, tt.want)
		}
	}
}

func TestParseTemplate_Errors(t *testing.T) {
	tests := []struct {
		template string
		want     string
	}{
		{"{yaer}", "unknown placeholder {yaer}. Did you mean {year}?"},
		{"{Year}", "Did you mean {year}?"},
		{"{nothing}", "Available: {category}"},
		{"{}", "empty placeholder"},
		{"{year", `unclosed "{"`},
		{"year}", `unexpected "}"`},
		{"", "empty template"},
		{"/abs/{year}", "must be relative"},
		{"a//b", "empty folder name"},
		{"photos/", "empty folder name"},
		{"../{year}", `".." is not allowed`},
	}

	for _, tt := range tests {
		_, err := ParseTemplate(tt.template)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("ParseTemplate(%q) error = %v, want %q", tt.template, err, tt.want)
		}
	}
}

func TestTemplate_Uses(t *testing.T) {
	tmpl, err := ParseTemplate("photos/{year}/{ml}")
	if err != nil {
		t.Fatal(err)
	}
	if !tmpl.Uses(PhML) || !tmpl.Uses(DatePlaceholders...) || tmpl.Uses(PhCategory) {
		t.Error("Uses reports wrong placeholders")
	}
}

func TestSet_FirstMatchWins(t *testing.T) {
	mtime := time.Date(2025, 3, 2, 10, 0, 0, 0, time.Local)
	set, err := Compile([]Rule{
		{Match: Match{Name: "invoice*", Ext: []string{"pdf"}}, To: "invoices/{year}"},
		{Match: Match{Category: []string{"images"}}, To: "photos/{year}/{month}"},
		{Match: Match{Ext: []string{".TXT"}}, To: "notes"},
		{To: "{category}"},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		want []string
	}{
		{"Invoice_42.PDF", []string{"invoices", "2025"}},
		{"report.pdf", []string{"documents"}},
		{"cat.png", []string{"photos", "2025", "03"}},
		{"todo.txt", []string{"notes"}},
		{"main.go", []string{"code"}},
	}
	for _, tt := range tests {
		got, ok := set.Route(file(t, tt.name, 10, mtime))
		if !ok || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Route(%s) = %v, %v; want %v", tt.name, got, ok, tt.want)
		}
	}
}

func TestSet_NoMatchLeavesFileInPlace(t *testing.T) {
	set := MustCompile([]Rule{{Match: Match{Category: []string{"images", "videos"}}, To: "{year}/{month}"}})

	if _, ok := set.Route(file(t, "report.pdf", 10, time.Now())); ok {
		t.Error("a PDF matched a rule for images and videos")
	}
	if _, ok := set.Route(file(t, "clip.MOV", 10, time.Now())); !ok {
		t.Error("a video did not match")
	}
}

func TestCompile_Errors(t *testing.T) {
	_, err := Compile([]Rule{
		{To: "{category}"},
		{To: "{yaer}"},
		{Match: Match{Category: []string{"imagez"}}, To: "x"},
		{Match: Match{Name: "[unclosed"}, To: "x"},
	})
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{
		"rule 2: unknown placeholder {yaer}. Did you mean {year}?",
		`rule 3: unknown category "imagez". Did you mean "images"?`,
		`rule 4: bad name pattern "[unclosed"`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not contain %q:\n%v", want, err)
		}
	}
	if strings.Contains(err.Error(), "rule 1") {
		t.Errorf("a valid rule is reported: %v", err)
	}

	if _, err := Compile(nil); err == nil {
		t.Error("no rules must be an error")
	}
}

func TestSet_Uses(t *testing.T) {
	set := MustCompile([]Rule{
		{Match: Match{Category: []string{"images"}}, To: "{ml}"},
		{To: "{category}"},
	})
	if !set.Uses(PhML) || !set.RuleUses(0, PhML) || set.RuleUses(1, PhML) {
		t.Error("Uses reports wrong rules")
	}
}

func TestSizeCategory(t *testing.T) {
	tests := map[int64]string{
		0: "tiny", 10_239: "tiny", 10_240: "small", 1_000_000: "medium",
		5_000_000: "large", 50_000_000: "huge", 200_000_000: "massive",
	}
	for size, want := range tests {
		if got := SizeCategory(size); got != want {
			t.Errorf("SizeCategory(%d) = %q, want %q", size, got, want)
		}
	}
}
