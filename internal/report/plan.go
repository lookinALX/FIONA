package report

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lookinalx/fiona/internal/scanner"
	"github.com/lookinalx/fiona/internal/sorter"
	"github.com/lookinalx/fiona/internal/types"
)

const (
	maxFolders  = 12 // top-level folders shown before "… and N more"
	maxChildren = 12
	maxProblems = 5
	topExts     = 3
)

// PlanView is what the overview of a plan needs besides the plan itself.
type PlanView struct {
	Plan       *sorter.Plan
	RuleSource string // like `with preset "downloads"`
	Recursive  bool
}

// Header is the first line: what is sorted and by which rules.
func Header(v PlanView) string {
	p := v.Plan
	line := "Sorting " + Path(p.BaseDirSource)
	if v.RuleSource != "" {
		line += " " + v.RuleSource
	}
	if p.BaseDirDest != p.BaseDirSource {
		line += " into " + Path(p.BaseDirDest)
	}
	return line
}

// Empty reports whether the scan found no files at all.
func Empty(p *sorter.Plan) bool {
	return len(p.Actions) == 0 && len(p.Unmatched) == 0
}

// NothingToSort explains an empty scan.
func NothingToSort(v PlanView) string {
	p := v.Plan
	msg := "Nothing to sort in " + Path(p.BaseDirSource) + ": "
	switch {
	case len(p.Kept) > 0 && !v.Recursive:
		msg += fmt.Sprintf("no files at the top level, only %s. Use -r to include the files inside them.", Count(len(p.Kept), "folder"))
	case len(p.Kept) > 0:
		msg += fmt.Sprintf("no files, only %s kept whole (bundles and projects).", Count(len(p.Kept), "folder"))
	default:
		msg += "no files."
	}
	return msg
}

// Overview lists the folders and the notes of a plan, without the header.
func Overview(v PlanView, st Styles) []string {
	var lines []string

	if folders := v.Plan.Folders(); len(folders) > 0 {
		lines = append(lines, folderLines(folders, st)...)
	}
	if notes := Notes(v.Plan, st); len(notes) > 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, notes...)
	}
	return lines
}

func folderLines(folders []sorter.Folder, st Styles) []string {
	shown, hidden := pickFolders(folders)

	type row struct {
		name, note string
		files      int
		size       int64
	}
	var rows []row
	for _, f := range shown {
		rows = append(rows, row{f.Name + "/", folderNote(f), f.Files, f.Size})

		children := f.Children
		if len(children) > maxChildren {
			children = children[:maxChildren]
		}
		for _, c := range children {
			note := ""
			if c.Exists && !f.Exists {
				note = "exists"
			}
			rows = append(rows, row{"  " + c.Name + "/", note, c.Files, c.Size})
		}
		if more := len(f.Children) - len(children); more > 0 {
			rows = append(rows, row{name: fmt.Sprintf("  … and %d more", more)})
		}
	}

	counts := make([]int, 0, len(rows))
	for _, r := range rows {
		counts = append(counts, r.files)
	}
	files := filesColumn(counts)

	var t table
	for i, r := range rows {
		if r.files == 0 {
			t.add(r.name, "", "", "")
			continue
		}
		t.add(r.name, files[i], Size(r.size), r.note)
	}
	lines := t.render("  ", st)

	if len(hidden) > 0 {
		n := 0
		for _, f := range hidden {
			n += f.Files
		}
		lines = append(lines, fmt.Sprintf("  … and %d more %s, %s", len(hidden), Plural(len(hidden), "folder"), Count(n, "file")))
	}
	return lines
}

// pickFolders keeps the biggest folders when there are too many, in their order.
func pickFolders(folders []sorter.Folder) (shown, hidden []sorter.Folder) {
	if len(folders) <= maxFolders {
		return folders, nil
	}
	idx := make([]int, len(folders))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return folders[idx[a]].Files > folders[idx[b]].Files })

	keep := make(map[int]bool, maxFolders)
	for _, i := range idx[:maxFolders] {
		keep[i] = true
	}
	for i, f := range folders {
		if keep[i] {
			shown = append(shown, f)
		} else {
			hidden = append(hidden, f)
		}
	}
	return shown, hidden
}

func folderNote(f sorter.Folder) string {
	var notes []string
	if f.Exists {
		notes = append(notes, "exists")
	}
	if strings.EqualFold(f.Name, scanner.CategoryOther) && len(f.Children) == 0 && len(f.Exts) > 0 {
		var exts []string
		for i, e := range f.Exts {
			if i == topExts {
				exts = append(exts, "…")
				break
			}
			name := "." + e.Ext
			if e.Ext == "" {
				name = "no extension"
			}
			exts = append(exts, fmt.Sprintf("%s %d", name, e.Count))
		}
		notes = append(notes, strings.Join(exts, ", "))
	}
	return strings.Join(notes, " · ")
}

// Notes are the counters of a plan that are not zero, with reasons.
func Notes(p *sorter.Plan, st Styles) []string {
	stats := p.Stats()
	var lines []string
	add := func(n int, label string) {
		if n > 0 {
			lines = append(lines, fmt.Sprintf("  %s: %d", label, n))
		}
	}

	add(stats.Renamed, "Renamed on conflict")
	add(stats.Replaced, "Replaced, old files go to trash")
	add(stats.Skipped, "Skipped, already exist")
	add(stats.Noop, "Already in place")
	add(stats.Rejected, "Rejected")

	var rejected []string
	for _, a := range p.Actions {
		if a.Kind == types.OpReject {
			rejected = append(rejected, fmt.Sprintf("%s: %s", relTo(p.BaseDirSource, a.SourcePath), a.Note))
		}
	}
	lines = append(lines, problemLines(rejected, st)...)

	if len(p.Skipped) > 0 {
		lines = append(lines, "  Skipped by scanner: "+skippedSummary(p.Skipped))
	}

	var left []string
	if n := len(keptOutsidePlan(p)); n > 0 {
		left = append(left, Count(n, "folder"))
	}
	if n := len(p.Unmatched); n > 0 {
		verb := "match"
		if n == 1 {
			verb = "matches"
		}
		left = append(left, fmt.Sprintf("%s that %s no rule", Count(n, "file"), verb))
	}
	if len(left) > 0 {
		lines = append(lines, "  Left as is: "+strings.Join(left, ", "))
	}

	return lines
}

// keptOutsidePlan leaves out the folders the plan sorts files into: when
// sorting a folder into itself they are found by the scan as well.
func keptOutsidePlan(p *sorter.Plan) []string {
	targets := make(map[string]bool)
	for _, a := range p.Actions {
		if a.Executable() || a.Kind == types.OpNoop || a.Kind == types.OpSkip {
			top := strings.SplitN(a.DestDir, string(filepath.Separator), 2)[0]
			targets[strings.ToLower(filepath.Join(p.BaseDirDest, top))] = true
		}
	}
	var kept []string
	for _, dir := range p.Kept {
		if !targets[strings.ToLower(dir)] {
			kept = append(kept, dir)
		}
	}
	return kept
}

// problemLines lists the first few problems, indented under their counter.
func problemLines(problems []string, st Styles) []string {
	var lines []string
	for i, p := range problems {
		if i == maxProblems {
			lines = append(lines, st.dim(fmt.Sprintf("    … and %d more (see --list)", len(problems)-maxProblems)))
			break
		}
		lines = append(lines, st.dim("    "+p))
	}
	return lines
}

// skippedSummary renders scanner skips like "3 (2 symlinks, 1 no access)".
func skippedSummary(skipped []scanner.SkippedEntry) string {
	counts := make(map[string]int)
	for _, entry := range skipped {
		counts[entry.Reason]++
	}

	var parts []string
	for _, reason := range sortedKeys(counts) {
		label := reason
		if counts[reason] != 1 && reason != scanner.ReasonNoAccess {
			label += "s"
		}
		parts = append(parts, fmt.Sprintf("%d %s", counts[reason], label))
	}
	return fmt.Sprintf("%d (%s)", len(skipped), strings.Join(parts, ", "))
}

// Question asks to execute the plan: "Move 214 files (1.8 GB) into 6 folders?"
func Question(p *sorter.Plan) string {
	stats := p.Stats()
	verb := "Move"
	if p.FileAction == string(types.OpCopy) {
		verb = "Copy"
	}
	q := fmt.Sprintf("%s %s (%s) into %s", verb, Count(stats.Files, "file"), Size(stats.Size), Count(len(p.Folders()), "folder"))
	if p.BaseDirDest != p.BaseDirSource {
		q += " in " + Path(p.BaseDirDest)
	}
	return q + "?"
}

// FileList shows what happens to every file.
func FileList(p *sorter.Plan, st Styles) []string {
	var lines []string
	for _, a := range p.Actions {
		src := relTo(p.BaseDirSource, a.SourcePath)
		dst := relTo(p.BaseDirDest, a.TargetPath)

		switch a.Kind {
		case types.OpMove, types.OpCopy:
			line := fmt.Sprintf("  %s → %s", src, dst)
			switch a.Conflict {
			case types.ConflictRenamed:
				line += st.dim(" (renamed)")
			case types.ConflictReplaced:
				line += st.dim(" (replaces the existing file, it goes to trash)")
			}
			lines = append(lines, line)
		case types.OpSkip:
			lines = append(lines, st.dim(fmt.Sprintf("  %s: skipped, %s exists", src, dst)))
		case types.OpNoop:
			lines = append(lines, st.dim(fmt.Sprintf("  %s: already in place", src)))
		case types.OpReject:
			lines = append(lines, st.dim(fmt.Sprintf("  %s: rejected, %s", src, a.Note)))
		}
	}
	for _, path := range p.Unmatched {
		lines = append(lines, st.dim(fmt.Sprintf("  %s: matches no rule, stays", relTo(p.BaseDirSource, path))))
	}
	return lines
}

// relTo shows path relative to root when it lies inside it.
func relTo(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && filepath.IsLocal(rel) {
		return rel
	}
	return Path(path)
}
