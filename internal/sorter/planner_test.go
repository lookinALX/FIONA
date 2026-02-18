package sorter

import (
	"FIONA/internal/cli"
	"FIONA/internal/rules"
	"FIONA/internal/scanner"
	"FIONA/internal/types"
	"path/filepath"
	"reflect"
	"testing"
)

// ─── Test helpers ────────────────────────────────────────────────────────────

func makeFileInfo(name string, ext string, fileType string, size int64) *scanner.FileInfo {
	return &scanner.FileInfo{
		Path:      filepath.Join("user", "source", name),
		Name:      name,
		Extension: ext,
		Size:      size,
		Date:      "January 2025",
		Type:      fileType,
	}
}

func defaultOpts() *cli.Opts {
	return &cli.Opts{
		SourcePath: filepath.Join("user", "source"),
		DestPath:   filepath.Join("user", "dest"),
	}
}

// ─── NewAction ───────────────────────────────────────────────────────────────

func TestNewAction_PrimaryOnly(t *testing.T) {
	fi := makeFileInfo("photo.jpg", ".jpg", "images", 1024)
	destFullPath := filepath.Join("user", "dest")

	ruleSet := []rules.Rule{
		&rules.ByTypeRule{IsPrimary: true},
	}

	action := types.NewAction(fi, ruleSet, destFullPath)

	wantDestDir := "images"
	wantDestPath := filepath.Join("user", "dest", "images")

	if action.SourcePath != fi.Path {
		t.Errorf("SourcePath = %q, want %q", action.SourcePath, fi.Path)
	}
	if action.DestDir != wantDestDir {
		t.Errorf("DestDir = %q, want %q", action.DestDir, wantDestDir)
	}
	if action.DestPath != wantDestPath {
		t.Errorf("DestPath = %q, want %q", action.DestPath, wantDestPath)
	}
	if action.FileSize != fi.Size {
		t.Errorf("FileSize = %d, want %d", action.FileSize, fi.Size)
	}
}

func TestNewAction_PrimaryAndSecondary(t *testing.T) {
	fi := makeFileInfo("photo.jpg", ".jpg", "images", 2048)
	destFullPath := filepath.Join("user", "dest")

	ruleSet := []rules.Rule{
		&rules.ByTypeRule{IsPrimary: true},
		&rules.ByExtensionRule{IsPrimary: false},
	}

	action := types.NewAction(fi, ruleSet, destFullPath)

	wantDestDir := filepath.Join("images", "jpg")
	wantDestPath := filepath.Join("user", "dest", "images", "jpg")

	if action.DestDir != wantDestDir {
		t.Errorf("DestDir = %q, want %q", action.DestDir, wantDestDir)
	}
	if action.DestPath != wantDestPath {
		t.Errorf("DestPath = %q, want %q", action.DestPath, wantDestPath)
	}
}

func TestNewAction_SecondaryPrimary(t *testing.T) {
	// Поменяем порядок: primary = extension, secondary = type
	fi := makeFileInfo("report.pdf", ".pdf", "documents", 4096)
	destFullPath := filepath.Join("user", "dest")

	ruleSet := []rules.Rule{
		&rules.ByExtensionRule{IsPrimary: true},
		&rules.ByTypeRule{IsPrimary: false},
	}

	action := types.NewAction(fi, ruleSet, destFullPath)

	wantDestDir := filepath.Join("pdf", "documents")
	wantDestPath := filepath.Join("user", "dest", "pdf", "documents")

	if action.DestDir != wantDestDir {
		t.Errorf("DestDir = %q, want %q", action.DestDir, wantDestDir)
	}
	if action.DestPath != wantDestPath {
		t.Errorf("DestPath = %q, want %q", action.DestPath, wantDestPath)
	}
}

// ─── AddAction ───────────────────────────────────────────────────────────────

func TestAddAction_SingleAction(t *testing.T) {
	plan := NewPlan(defaultOpts())

	action := types.Action{
		SourcePath: filepath.Join("user", "source", "photo.jpg"),
		DestDir:    "images",
		DestPath:   filepath.Join("user", "dest", "images"),
		FileSize:   1024,
	}

	plan.AddAction(action)

	if len(plan.Actions) != 1 {
		t.Fatalf("len(Actions) = %d, want 1", len(plan.Actions))
	}
	if plan.DirCounts["images"] != 1 {
		t.Errorf("DirCounts[images] = %d, want 1", plan.DirCounts["images"])
	}
	if plan.DirSizes["images"] != 1024 {
		t.Errorf("DirSizes[images] = %d, want 1024", plan.DirSizes["images"])
	}
}

func TestAddAction_MultipleToSameDir(t *testing.T) {
	plan := NewPlan(defaultOpts())

	actions := []types.Action{
		{SourcePath: filepath.Join("user", "source", "photo1.jpg"), DestDir: "images", DestPath: filepath.Join("user", "dest", "images"), FileSize: 1024},
		{SourcePath: filepath.Join("user", "source", "photo2.jpg"), DestDir: "images", DestPath: filepath.Join("user", "dest", "images"), FileSize: 2048},
		{SourcePath: filepath.Join("user", "source", "photo3.jpg"), DestDir: "images", DestPath: filepath.Join("user", "dest", "images"), FileSize: 512},
	}

	for _, a := range actions {
		plan.AddAction(a)
	}

	if len(plan.Actions) != 3 {
		t.Fatalf("len(Actions) = %d, want 3", len(plan.Actions))
	}
	if plan.DirCounts["images"] != 3 {
		t.Errorf("DirCounts[images] = %d, want 3", plan.DirCounts["images"])
	}
	if plan.DirSizes["images"] != 1024+2048+512 {
		t.Errorf("DirSizes[images] = %d, want %d", plan.DirSizes["images"], 1024+2048+512)
	}
}

func TestAddAction_MultipleDirs_PrimaryOnly(t *testing.T) {
	plan := NewPlan(defaultOpts())

	actions := []types.Action{
		{SourcePath: filepath.Join("user", "source", "photo.jpg"), DestDir: "images", DestPath: filepath.Join("user", "dest", "images"), FileSize: 1024},
		{SourcePath: filepath.Join("user", "source", "report.pdf"), DestDir: "documents", DestPath: filepath.Join("user", "dest", "documents"), FileSize: 2048},
		{SourcePath: filepath.Join("user", "source", "song.mp3"), DestDir: "audios", DestPath: filepath.Join("user", "dest", "audios"), FileSize: 4096},
	}

	for _, a := range actions {
		plan.AddAction(a)
	}

	wantCounts := map[string]int{
		"images":    1,
		"documents": 1,
		"audios":    1,
	}
	wantSizes := map[string]int64{
		"images":    1024,
		"documents": 2048,
		"audios":    4096,
	}

	if !reflect.DeepEqual(plan.DirCounts, wantCounts) {
		t.Errorf("DirCounts = %v, want %v", plan.DirCounts, wantCounts)
	}
	if !reflect.DeepEqual(plan.DirSizes, wantSizes) {
		t.Errorf("DirSizes = %v, want %v", plan.DirSizes, wantSizes)
	}
}

func TestAddAction_MultipleDirs_PrimaryAndSecondary(t *testing.T) {
	// primary = type, secondary = extension
	// images/jpg/, images/png/, documents/pdf/
	plan := NewPlan(defaultOpts())

	actions := []types.Action{
		{SourcePath: filepath.Join("user", "source", "photo.jpg"), DestDir: filepath.Join("images", "jpg"), DestPath: filepath.Join("user", "dest", "images", "jpg"), FileSize: 1024},
		{SourcePath: filepath.Join("user", "source", "logo.png"), DestDir: filepath.Join("images", "png"), DestPath: filepath.Join("user", "dest", "images", "png"), FileSize: 2048},
		{SourcePath: filepath.Join("user", "source", "banner.jpg"), DestDir: filepath.Join("images", "jpg"), DestPath: filepath.Join("user", "dest", "images", "jpg"), FileSize: 512},
		{SourcePath: filepath.Join("user", "source", "report.pdf"), DestDir: filepath.Join("documents", "pdf"), DestPath: filepath.Join("user", "dest", "documents", "pdf"), FileSize: 4096},
	}

	for _, a := range actions {
		plan.AddAction(a)
	}

	wantCounts := map[string]int{
		filepath.Join("images", "jpg"):    2,
		filepath.Join("images", "png"):    1,
		filepath.Join("documents", "pdf"): 1,
	}
	wantSizes := map[string]int64{
		filepath.Join("images", "jpg"):    1024 + 512,
		filepath.Join("images", "png"):    2048,
		filepath.Join("documents", "pdf"): 4096,
	}

	if !reflect.DeepEqual(plan.DirCounts, wantCounts) {
		t.Errorf("DirCounts = %v, want %v", plan.DirCounts, wantCounts)
	}
	if !reflect.DeepEqual(plan.DirSizes, wantSizes) {
		t.Errorf("DirSizes = %v, want %v", plan.DirSizes, wantSizes)
	}
}

// ─── Summary ─────────────────────────────────────────────────────────────────

func TestSummary_PrimaryOnly(t *testing.T) {
	plan := NewPlan(defaultOpts())

	actions := []types.Action{
		{SourcePath: filepath.Join("user", "source", "photo.jpg"), DestDir: "images", DestPath: filepath.Join("user", "dest", "images"), FileSize: 1024},
		{SourcePath: filepath.Join("user", "source", "logo.png"), DestDir: "images", DestPath: filepath.Join("user", "dest", "images"), FileSize: 2048},
		{SourcePath: filepath.Join("user", "source", "report.pdf"), DestDir: "documents", DestPath: filepath.Join("user", "dest", "documents"), FileSize: 4096},
	}

	for _, a := range actions {
		plan.AddAction(a)
	}

	summary := plan.Summary()

	imagesKey := filepath.Join("user", "dest", "images")
	docsKey := filepath.Join("user", "dest", "documents")

	if len(summary[imagesKey]) != 2 {
		t.Errorf("summary[images] has %d files, want 2", len(summary[imagesKey]))
	}
	if len(summary[docsKey]) != 1 {
		t.Errorf("summary[documents] has %d files, want 1", len(summary[docsKey]))
	}
}

func TestSummary_PrimaryAndSecondary(t *testing.T) {
	plan := NewPlan(defaultOpts())

	actions := []types.Action{
		{SourcePath: filepath.Join("user", "source", "photo.jpg"), DestDir: filepath.Join("images", "jpg"), DestPath: filepath.Join("user", "dest", "images", "jpg"), FileSize: 1024},
		{SourcePath: filepath.Join("user", "source", "banner.jpg"), DestDir: filepath.Join("images", "jpg"), DestPath: filepath.Join("user", "dest", "images", "jpg"), FileSize: 512},
		{SourcePath: filepath.Join("user", "source", "logo.png"), DestDir: filepath.Join("images", "png"), DestPath: filepath.Join("user", "dest", "images", "png"), FileSize: 2048},
	}

	for _, a := range actions {
		plan.AddAction(a)
	}

	summary := plan.Summary()

	jpgKey := filepath.Join("user", "dest", "images", "jpg")
	pngKey := filepath.Join("user", "dest", "images", "png")

	if len(summary[jpgKey]) != 2 {
		t.Errorf("summary[images/jpg] has %d files, want 2", len(summary[jpgKey]))
	}
	if len(summary[pngKey]) != 1 {
		t.Errorf("summary[images/png] has %d files, want 1", len(summary[pngKey]))
	}
}

// ─── Edge cases ──────────────────────────────────────────────────────────────

func TestNewAction_UnknownFileType(t *testing.T) {
	// Файл с неизвестным расширением → type = "unknown"
	fi := makeFileInfo("config.xyz", ".xyz", "unknown", 100)
	destFullPath := filepath.Join("user", "dest")

	ruleSet := []rules.Rule{
		&rules.ByTypeRule{IsPrimary: true},
		&rules.ByExtensionRule{IsPrimary: false},
	}

	action := types.NewAction(fi, ruleSet, destFullPath)

	wantDestDir := filepath.Join("unknown", "xyz")
	wantDestPath := filepath.Join("user", "dest", "unknown", "xyz")

	if action.DestDir != wantDestDir {
		t.Errorf("DestDir = %q, want %q", action.DestDir, wantDestDir)
	}
	if action.DestPath != wantDestPath {
		t.Errorf("DestPath = %q, want %q", action.DestPath, wantDestPath)
	}
}

func TestAddAction_EmptyPlan(t *testing.T) {
	plan := NewPlan(defaultOpts())

	if len(plan.Actions) != 0 {
		t.Errorf("new plan has %d actions, want 0", len(plan.Actions))
	}
	if len(plan.DirCounts) != 0 {
		t.Errorf("new plan DirCounts has %d entries, want 0", len(plan.DirCounts))
	}
	if len(plan.DirSizes) != 0 {
		t.Errorf("new plan DirSizes has %d entries, want 0", len(plan.DirSizes))
	}
}

func TestSummary_EmptyPlan(t *testing.T) {
	plan := NewPlan(defaultOpts())
	summary := plan.Summary()

	if len(summary) != 0 {
		t.Errorf("summary of empty plan has %d entries, want 0", len(summary))
	}
}
