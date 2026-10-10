package sorter

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lookinalx/fiona/internal/rules"
	"github.com/lookinalx/fiona/internal/scanner"
	"github.com/lookinalx/fiona/internal/types"
)

// ─── Test helpers ────────────────────────────────────────────────────────────

const testRunID = "20261010-153012-a1b2"

func makeFileInfo(path string, fileType string, size int64) *scanner.FileInfo {
	return &scanner.FileInfo{
		Path:      path,
		Name:      filepath.Base(path),
		Extension: filepath.Ext(path),
		Size:      size,
		Date:      "January 2025",
		Type:      fileType,
	}
}

func byType() []rules.Rule {
	return []rules.Rule{&rules.ByTypeRule{IsPrimary: true}}
}

func params(source, dest, onConflict string) PlanParams {
	return PlanParams{
		Source:     source,
		Dest:       dest,
		FileAction: "move",
		OnConflict: onConflict,
		RunID:      testRunID,
	}
}

// ─── Targets ─────────────────────────────────────────────────────────────────

func TestNewPlan_PrimaryOnly(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	fi := makeFileInfo(filepath.Join(source, "photo.jpg"), "images", 1024)

	plan := NewPlan([]*scanner.FileInfo{fi}, byType(), nil, params(source, dest, types.OnConflictRename))

	if len(plan.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(plan.Actions))
	}
	action := plan.Actions[0]

	if action.Kind != types.OpMove {
		t.Errorf("Kind = %q, want %q", action.Kind, types.OpMove)
	}
	if action.SourcePath != fi.Path {
		t.Errorf("SourcePath = %q, want %q", action.SourcePath, fi.Path)
	}
	if action.DestDir != "images" {
		t.Errorf("DestDir = %q, want %q", action.DestDir, "images")
	}
	if want := filepath.Join(dest, "images", "photo.jpg"); action.TargetPath != want {
		t.Errorf("TargetPath = %q, want %q", action.TargetPath, want)
	}
	if action.Size != fi.Size {
		t.Errorf("Size = %d, want %d", action.Size, fi.Size)
	}
	if action.Conflict != "" || action.TrashPath != "" || action.Note != "" {
		t.Errorf("unexpected conflict data: %+v", action)
	}
}

func TestNewPlan_PrimaryAndSecondary(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	fi := makeFileInfo(filepath.Join(source, "photo.jpg"), "images", 2048)

	ruleSet := []rules.Rule{
		&rules.ByTypeRule{IsPrimary: true},
		&rules.ByExtensionRule{IsPrimary: false},
	}

	plan := NewPlan([]*scanner.FileInfo{fi}, ruleSet, nil, params(source, dest, types.OnConflictRename))
	action := plan.Actions[0]

	if want := filepath.Join("images", "jpg"); action.DestDir != want {
		t.Errorf("DestDir = %q, want %q", action.DestDir, want)
	}
	if want := filepath.Join(dest, "images", "jpg", "photo.jpg"); action.TargetPath != want {
		t.Errorf("TargetPath = %q, want %q", action.TargetPath, want)
	}
}

func TestNewPlan_CopyAction(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	fi := makeFileInfo(filepath.Join(source, "photo.jpg"), "images", 1)

	p := params(source, dest, types.OnConflictRename)
	p.FileAction = "copy"
	plan := NewPlan([]*scanner.FileInfo{fi}, byType(), nil, p)

	if plan.Actions[0].Kind != types.OpCopy {
		t.Errorf("Kind = %q, want %q", plan.Actions[0].Kind, types.OpCopy)
	}
}

func TestNewPlan_SortedBySourcePath(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	files := []*scanner.FileInfo{
		makeFileInfo(filepath.Join(source, "c.jpg"), "images", 1),
		makeFileInfo(filepath.Join(source, "a.jpg"), "images", 1),
		makeFileInfo(filepath.Join(source, "b.jpg"), "images", 1),
	}

	plan := NewPlan(files, byType(), nil, params(source, dest, types.OnConflictRename))

	for i, name := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		action := plan.Actions[i]
		if action.Seq != i || filepath.Base(action.SourcePath) != name {
			t.Errorf("action %d = seq %d, %s; want seq %d, %s", i, action.Seq, action.SourcePath, i, name)
		}
	}
}

func TestNewPlan_EmptyPlan(t *testing.T) {
	plan := NewPlan(nil, byType(), nil, params(t.TempDir(), t.TempDir(), types.OnConflictRename))

	if len(plan.Actions) != 0 || len(plan.Mkdirs) != 0 {
		t.Errorf("expected empty plan, got %+v", plan)
	}
	if stats := plan.Stats(); stats != (PlanStats{}) {
		t.Errorf("Stats() = %+v, want zero", stats)
	}
}

// ─── Sanitizing and bounds ───────────────────────────────────────────────────

func TestNewPlan_SanitizesRuleComponents(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	fi := makeFileInfo(filepath.Join(source, "photo.jpg"), "images", 1)
	fi.Tag = "../../x"

	plan := NewPlan([]*scanner.FileInfo{fi}, []rules.Rule{&rules.ByMLRule{IsPrimary: true}}, nil,
		params(source, dest, types.OnConflictRename))
	action := plan.Actions[0]

	if action.Kind != types.OpMove {
		t.Fatalf("Kind = %q (%s), want %q", action.Kind, action.Note, types.OpMove)
	}
	if want := filepath.Join(dest, ".._.._x", "photo.jpg"); action.TargetPath != want {
		t.Errorf("TargetPath = %q, want %q", action.TargetPath, want)
	}
}

func TestNewPlan_RejectsWhenDestDirIsFile(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(dest, "images"), []byte("I am a file"), 0644)
	fi := makeFileInfo(filepath.Join(source, "photo.jpg"), "images", 1)

	plan := NewPlan([]*scanner.FileInfo{fi}, byType(), nil, params(source, dest, types.OnConflictRename))
	action := plan.Actions[0]

	if action.Kind != types.OpReject || action.Note == "" {
		t.Errorf("Kind = %q with note %q, want %q with a reason", action.Kind, action.Note, types.OpReject)
	}
	if len(plan.Mkdirs) != 0 {
		t.Errorf("Mkdirs = %v, want none", plan.Mkdirs)
	}
}

func TestNewPlan_RejectsWhenDestDirIsSymlink(t *testing.T) {
	source, dest, outside := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dest, "images")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	fi := makeFileInfo(filepath.Join(source, "photo.jpg"), "images", 1)

	plan := NewPlan([]*scanner.FileInfo{fi}, byType(), nil, params(source, dest, types.OnConflictRename))

	if plan.Actions[0].Kind != types.OpReject {
		t.Errorf("Kind = %q, want %q: a symlink must not lead the write out of dest", plan.Actions[0].Kind, types.OpReject)
	}
}

// ─── Noop ────────────────────────────────────────────────────────────────────

func TestNewPlan_NoopWhenFileIsInPlace(t *testing.T) {
	dest := t.TempDir()
	path := filepath.Join(dest, "images", "photo.jpg")
	createTestFile(t, path, []byte("photo"), 0644)
	fi := makeFileInfo(path, "images", 5)

	for _, strategy := range []string{types.OnConflictRename, types.OnConflictSkip, types.OnConflictReplace} {
		plan := NewPlan([]*scanner.FileInfo{fi}, byType(), nil, params(dest, dest, strategy))
		action := plan.Actions[0]

		if action.Kind != types.OpNoop {
			t.Errorf("%s: Kind = %q, want %q", strategy, action.Kind, types.OpNoop)
		}
		if action.TargetPath != path || action.TrashPath != "" {
			t.Errorf("%s: unexpected action %+v", strategy, action)
		}
	}
}

func TestNewPlan_NoopForHardLink(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	original := filepath.Join(source, "photo.jpg")
	createTestFile(t, original, []byte("photo"), 0644)
	if err := os.MkdirAll(filepath.Join(dest, "images"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(original, filepath.Join(dest, "images", "photo.jpg")); err != nil {
		t.Skipf("cannot create hard link: %v", err)
	}

	plan := NewPlan([]*scanner.FileInfo{makeFileInfo(original, "images", 5)}, byType(), nil,
		params(source, dest, types.OnConflictReplace))

	if plan.Actions[0].Kind != types.OpNoop {
		t.Errorf("Kind = %q, want %q", plan.Actions[0].Kind, types.OpNoop)
	}
}

// ─── Conflicts ───────────────────────────────────────────────────────────────

func TestNewPlan_RenameOnExistingFile(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(dest, "images", "photo.jpg"), []byte("old"), 0644)
	createTestFile(t, filepath.Join(dest, "images", "photo_1.jpg"), []byte("old"), 0644)
	fi := makeFileInfo(filepath.Join(source, "photo.jpg"), "images", 1)

	plan := NewPlan([]*scanner.FileInfo{fi}, byType(), nil, params(source, dest, types.OnConflictRename))
	action := plan.Actions[0]

	if action.Kind != types.OpMove || action.Conflict != types.ConflictRenamed {
		t.Errorf("Kind = %q, Conflict = %q", action.Kind, action.Conflict)
	}
	if want := filepath.Join(dest, "images", "photo_2.jpg"); action.TargetPath != want {
		t.Errorf("TargetPath = %q, want %q", action.TargetPath, want)
	}
}

func TestNewPlan_RenameWithinPlan(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	files := []*scanner.FileInfo{
		makeFileInfo(filepath.Join(source, "a", "IMG_0001.jpg"), "images", 1),
		makeFileInfo(filepath.Join(source, "b", "IMG_0001.jpg"), "images", 1),
		makeFileInfo(filepath.Join(source, "c", "IMG_0001.jpg"), "images", 1),
	}

	// every strategy must keep all three files: a target reserved by this plan is never replaced
	for _, strategy := range []string{types.OnConflictRename, types.OnConflictReplace} {
		plan := NewPlan(files, byType(), nil, params(source, dest, strategy))

		var targets []string
		for _, action := range plan.Actions {
			if action.Kind != types.OpMove || action.TrashPath != "" {
				t.Errorf("%s: unexpected action %+v", strategy, action)
			}
			targets = append(targets, filepath.Base(action.TargetPath))
		}

		want := []string{"IMG_0001.jpg", "IMG_0001_1.jpg", "IMG_0001_2.jpg"}
		if !reflect.DeepEqual(targets, want) {
			t.Errorf("%s: targets = %v, want %v", strategy, targets, want)
		}
	}
}

func TestNewPlan_NamesDifferingByCaseCollide(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	files := []*scanner.FileInfo{
		makeFileInfo(filepath.Join(source, "a", "Photo.JPG"), "images", 1),
		makeFileInfo(filepath.Join(source, "b", "photo.jpg"), "images", 1),
	}

	plan := NewPlan(files, byType(), nil, params(source, dest, types.OnConflictRename))

	if got := filepath.Base(plan.Actions[0].TargetPath); got != "Photo.JPG" {
		t.Errorf("first target = %q, want %q", got, "Photo.JPG")
	}
	if got := filepath.Base(plan.Actions[1].TargetPath); got != "photo_1.jpg" {
		t.Errorf("second target = %q, want %q", got, "photo_1.jpg")
	}
}

func TestNewPlan_UniqueTargetsForManyFiles(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()

	const numFiles = 200
	files := make([]*scanner.FileInfo, 0, numFiles)
	for i := 0; i < numFiles; i++ {
		files = append(files, makeFileInfo(filepath.Join(source, strings.Repeat("d", i+1), "same.txt"), "documents", 1))
	}

	plan := NewPlan(files, byType(), nil, params(source, dest, types.OnConflictRename))

	seen := make(map[string]struct{})
	for _, action := range plan.Actions {
		if action.Kind != types.OpMove {
			t.Fatalf("unexpected action %+v", action)
		}
		if _, dup := seen[action.TargetPath]; dup {
			t.Fatalf("target %s is planned twice", action.TargetPath)
		}
		seen[action.TargetPath] = struct{}{}
	}
	if len(seen) != numFiles {
		t.Errorf("planned %d unique targets, want %d", len(seen), numFiles)
	}
}

func TestNewPlan_Skip(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(dest, "images", "old.jpg"), []byte("old"), 0644)
	files := []*scanner.FileInfo{
		makeFileInfo(filepath.Join(source, "a", "new.jpg"), "images", 1),
		makeFileInfo(filepath.Join(source, "b", "new.jpg"), "images", 1),
		makeFileInfo(filepath.Join(source, "old.jpg"), "images", 1),
	}

	plan := NewPlan(files, byType(), nil, params(source, dest, types.OnConflictSkip))

	wantKinds := []types.OpKind{types.OpMove, types.OpSkip, types.OpSkip}
	for i, action := range plan.Actions {
		if action.Kind != wantKinds[i] {
			t.Errorf("action %d: Kind = %q, want %q", i, action.Kind, wantKinds[i])
		}
		if action.Kind == types.OpSkip && action.Note == "" {
			t.Errorf("action %d: skip without a note", i)
		}
	}
	if stats := plan.Stats(); stats.Files != 1 || stats.Skipped != 2 {
		t.Errorf("Stats() = %+v", stats)
	}
}

func TestNewPlan_ReplaceMovesExistingFileToTrash(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(dest, "images", "photo.jpg"), []byte("old"), 0644)
	fi := makeFileInfo(filepath.Join(source, "photo.jpg"), "images", 1)

	plan := NewPlan([]*scanner.FileInfo{fi}, byType(), nil, params(source, dest, types.OnConflictReplace))
	action := plan.Actions[0]

	if action.Kind != types.OpMove || action.Conflict != types.ConflictReplaced {
		t.Errorf("Kind = %q, Conflict = %q", action.Kind, action.Conflict)
	}
	if want := filepath.Join(dest, "images", "photo.jpg"); action.TargetPath != want {
		t.Errorf("TargetPath = %q, want %q", action.TargetPath, want)
	}
	if want := filepath.Join(dest, ".fiona", "trash", testRunID, "images", "photo.jpg"); action.TrashPath != want {
		t.Errorf("TrashPath = %q, want %q", action.TrashPath, want)
	}

	wantMkdirs := []string{
		filepath.Join(dest, ".fiona"),
		filepath.Join(dest, ".fiona", "trash"),
		filepath.Join(dest, ".fiona", "trash", testRunID),
		filepath.Join(dest, ".fiona", "trash", testRunID, "images"),
	}
	if !reflect.DeepEqual(plan.Mkdirs, wantMkdirs) {
		t.Errorf("Mkdirs = %v, want %v", plan.Mkdirs, wantMkdirs)
	}
}

func TestNewPlan_ReplaceFallsBackToRename(t *testing.T) {
	t.Run("directory in the way", func(t *testing.T) {
		source, dest := t.TempDir(), t.TempDir()
		if err := os.MkdirAll(filepath.Join(dest, "images", "photo.jpg"), 0755); err != nil {
			t.Fatal(err)
		}
		fi := makeFileInfo(filepath.Join(source, "photo.jpg"), "images", 1)

		plan := NewPlan([]*scanner.FileInfo{fi}, byType(), nil, params(source, dest, types.OnConflictReplace))
		action := plan.Actions[0]

		if action.Conflict != types.ConflictRenamed || action.TrashPath != "" || filepath.Base(action.TargetPath) != "photo_1.jpg" {
			t.Errorf("unexpected action %+v", action)
		}
	})

	t.Run("symlink in the way", func(t *testing.T) {
		source, dest := t.TempDir(), t.TempDir()
		if err := os.MkdirAll(filepath.Join(dest, "images"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(dest, "missing"), filepath.Join(dest, "images", "photo.jpg")); err != nil {
			t.Skipf("cannot create symlink: %v", err)
		}
		fi := makeFileInfo(filepath.Join(source, "photo.jpg"), "images", 1)

		plan := NewPlan([]*scanner.FileInfo{fi}, byType(), nil, params(source, dest, types.OnConflictReplace))
		action := plan.Actions[0]

		if action.Conflict != types.ConflictRenamed || action.TrashPath != "" {
			t.Errorf("a dangling symlink must count as occupied and must not be replaced: %+v", action)
		}
	})

	t.Run("file of this plan in the way", func(t *testing.T) {
		// 2025/a.jpg is itself sorted by this plan, so it must not be sent to trash
		dest := t.TempDir()
		occupant := filepath.Join(dest, "2025", "a.jpg")
		incoming := filepath.Join(dest, "inbox", "a.jpg")
		createTestFile(t, occupant, []byte("occupant"), 0644)
		createTestFile(t, incoming, []byte("incoming"), 0644)

		occupantInfo := makeFileInfo(occupant, "images", 8)
		occupantInfo.Date = "March 2024"
		incomingInfo := makeFileInfo(incoming, "images", 8)
		incomingInfo.Date = "March 2025"

		plan := NewPlan([]*scanner.FileInfo{occupantInfo, incomingInfo}, []rules.Rule{&rules.ByYearRule{IsPrimary: true}}, nil,
			params(dest, dest, types.OnConflictReplace))

		for _, action := range plan.Actions {
			if action.TrashPath != "" || action.Conflict == types.ConflictReplaced {
				t.Errorf("a file of this plan is going to be displaced: %+v", action)
			}
		}
		if got := plan.Actions[1].TargetPath; got != filepath.Join(dest, "2025", "a_1.jpg") {
			t.Errorf("incoming target = %q, want a_1.jpg next to the occupant", got)
		}
	})
}

func TestFreeName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"photo.jpg", "photo_1.jpg"},
		{"README", "README_1"},
		{"archive.tar.gz", "archive.tar_1.gz"},
		{".bashrc", ".bashrc_1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, dest := t.TempDir(), t.TempDir()
			createTestFile(t, filepath.Join(dest, "unknown", tt.name), []byte("old"), 0644)
			fi := makeFileInfo(filepath.Join(source, tt.name), "unknown", 1)

			plan := NewPlan([]*scanner.FileInfo{fi}, byType(), nil, params(source, dest, types.OnConflictRename))

			if got := filepath.Base(plan.Actions[0].TargetPath); got != tt.want {
				t.Errorf("renamed to %q, want %q", got, tt.want)
			}
		})
	}
}

// ─── Mkdirs ──────────────────────────────────────────────────────────────────

func TestNewPlan_MkdirsListsOnlyMissingDirectoriesOnce(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(dest, "images"), 0755); err != nil {
		t.Fatal(err)
	}

	files := []*scanner.FileInfo{
		makeFileInfo(filepath.Join(source, "a.jpg"), "images", 1),
		makeFileInfo(filepath.Join(source, "b.png"), "images", 1),
		makeFileInfo(filepath.Join(source, "c.pdf"), "documents", 1),
		makeFileInfo(filepath.Join(source, "d.pdf"), "documents", 1),
	}
	ruleSet := []rules.Rule{
		&rules.ByTypeRule{IsPrimary: true},
		&rules.ByExtensionRule{IsPrimary: false},
	}

	plan := NewPlan(files, ruleSet, nil, params(source, dest, types.OnConflictRename))

	want := []string{
		filepath.Join(dest, "images", "jpg"),
		filepath.Join(dest, "images", "png"),
		filepath.Join(dest, "documents"),
		filepath.Join(dest, "documents", "pdf"),
	}
	if !reflect.DeepEqual(plan.Mkdirs, want) {
		t.Errorf("Mkdirs = %v, want %v", plan.Mkdirs, want)
	}
}

func TestNewPlan_MkdirsNeverContainsDest(t *testing.T) {
	source := t.TempDir()
	dest := filepath.Join(t.TempDir(), "not", "created", "yet")
	fi := makeFileInfo(filepath.Join(source, "a.jpg"), "images", 1)

	plan := NewPlan([]*scanner.FileInfo{fi}, byType(), nil, params(source, dest, types.OnConflictRename))

	if want := []string{filepath.Join(dest, "images")}; !reflect.DeepEqual(plan.Mkdirs, want) {
		t.Errorf("Mkdirs = %v, want %v", plan.Mkdirs, want)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("planning created the destination directory")
	}
}

// ─── Summary, stats, print ───────────────────────────────────────────────────

func TestSummary_GroupsByDestDir(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	files := []*scanner.FileInfo{
		makeFileInfo(filepath.Join(source, "photo1.jpg"), "images", 100),
		makeFileInfo(filepath.Join(source, "photo2.png"), "images", 200),
		makeFileInfo(filepath.Join(source, "report.pdf"), "documents", 300),
	}

	plan := NewPlan(files, byType(), nil, params(source, dest, types.OnConflictRename))
	summary := plan.Summary()

	if len(summary) != 2 || len(summary["images"]) != 2 || len(summary["documents"]) != 1 {
		t.Errorf("Summary() = %v", summary)
	}
	if plan.DirCounts["images"] != 2 || plan.DirSizes["images"] != 300 {
		t.Errorf("images: count = %d, size = %d", plan.DirCounts["images"], plan.DirSizes["images"])
	}
	if stats := plan.Stats(); stats.Files != 3 || stats.Size != 600 {
		t.Errorf("Stats() = %+v", stats)
	}
}

func TestPrint_ShowsTotals(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	createTestFile(t, filepath.Join(dest, "images", "photo.jpg"), []byte("old"), 0644)
	createTestFile(t, filepath.Join(dest, "documents"), []byte("I am a file"), 0644)
	inPlace := filepath.Join(dest, "images", "placed.png")
	createTestFile(t, inPlace, []byte("placed"), 0644)

	files := []*scanner.FileInfo{
		makeFileInfo(filepath.Join(source, "photo.jpg"), "images", 100),
		makeFileInfo(filepath.Join(source, "clip.mp4"), "videos", 200),
		makeFileInfo(filepath.Join(source, "report.pdf"), "documents", 300),
		makeFileInfo(inPlace, "images", 6),
	}
	skipped := []scanner.SkippedEntry{
		{Path: filepath.Join(source, "link1"), Reason: scanner.ReasonSymlink},
		{Path: filepath.Join(source, "link2"), Reason: scanner.ReasonSymlink},
		{Path: filepath.Join(source, "locked"), Reason: scanner.ReasonNoAccess},
	}

	plan := NewPlan(files, byType(), skipped, params(source, dest, types.OnConflictRename))

	var out bytes.Buffer
	plan.Print(&out, true)
	text := out.String()

	for _, want := range []string{
		"Total: 2 files, 300 B",
		"photo_1.jpg  (renamed from photo.jpg)",
		"clip.mp4",
		"Renamed: 1",
		"Replaced: 0",
		"Skipped: 0",
		"Already in place: 1",
		"Rejected: 1",
		"Skipped by scanner: 3 (1 no access, 2 symlinks)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("plan does not contain %q:\n%s", want, text)
		}
	}
}

func TestPrint_EmptyPlan(t *testing.T) {
	plan := NewPlan(nil, byType(), nil, params(t.TempDir(), t.TempDir(), types.OnConflictRename))

	var out bytes.Buffer
	plan.Print(&out, true)

	if !strings.Contains(out.String(), "No files to process.") || !strings.Contains(out.String(), "Skipped by scanner: 0") {
		t.Errorf("unexpected plan:\n%s", out.String())
	}
}
