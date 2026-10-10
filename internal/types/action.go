package types

import "time"

type OpKind string

const (
	OpMove   OpKind = "move"
	OpCopy   OpKind = "copy"
	OpSkip   OpKind = "skip"   // conflict under the skip strategy, disk is not changed
	OpNoop   OpKind = "noop"   // file is already in place
	OpReject OpKind = "reject" // unsafe or impossible path
)

const (
	OnConflictReplace = "replace"
	OnConflictSkip    = "skip"
	OnConflictRename  = "rename"
)

const (
	ConflictRenamed  = "renamed"
	ConflictReplaced = "replaced"
)

type Action struct {
	Seq        int
	Kind       OpKind
	SourcePath string // absolute
	DestDir    string // relative to dest, for the plan tree
	TargetPath string // absolute final path of the file
	Size       int64
	ModTime    time.Time
	Conflict   string // "", "renamed", "replaced"
	TrashPath  string // where the displaced file goes on replace
	Note       string // reason for skip and reject
}

// Executable reports whether the action changes the disk.
func (a Action) Executable() bool {
	return a.Kind == OpMove || a.Kind == OpCopy
}

type UndoKind string

const (
	UndoMoveBack   UndoKind = "move-back"   // move the file from target back to source
	UndoRemoveCopy UndoKind = "remove-copy" // delete the copy at target
	UndoNothing    UndoKind = "nothing"     // operation changed nothing or is already reverted
	UndoSkip       UndoKind = "skip"        // verification failed, nothing is touched
)

type UndoAction struct {
	Seq          int
	Kind         UndoKind
	Op           Action // operation being reverted, as recorded in the journal
	RestoreTrash bool   // move the displaced file from trash back to target
	Note         string // reason for skip
}
