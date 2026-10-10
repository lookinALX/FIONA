package journal

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const (
	Version = 2

	// StateDir is the folder inside dest where FIONA keeps journals and trash.
	StateDir = ".fiona"
	RunsDir  = "runs"
	TrashDir = "trash"

	fileExt     = ".jsonl"
	runIDLayout = "20060102-150405"
)

const (
	typeHeader = "header"
	typeMkdir  = "mkdir"
	typeOp     = "op"
	typeDone   = "done"
	typeFooter = "footer"
	typeUndone = "undone"
)

type Header struct {
	RunID        string
	StartedAt    time.Time
	Source       string
	Dest         string
	FileAction   string
	OnConflict   string
	FionaVersion string
}

// Summary is what the footer of a finished run records.
type Summary struct {
	OK          int
	Failed      int
	Skipped     int
	Interrupted bool // an interrupted run gets no footer
}

type Footer struct {
	FinishedAt time.Time
	OK         int
	Failed     int
	Skipped    int
}

type headerRecord struct {
	Type         string    `json:"type"`
	Version      int       `json:"version"`
	RunID        string    `json:"run_id"`
	StartedAt    time.Time `json:"started_at"`
	Source       string    `json:"source"`
	Dest         string    `json:"dest"`
	FileAction   string    `json:"file_action"`
	OnConflict   string    `json:"on_conflict"`
	FionaVersion string    `json:"fiona_version"`
}

type mkdirRecord struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

type opRecord struct {
	Type     string     `json:"type"`
	Seq      int        `json:"seq"`
	Kind     string     `json:"kind"`
	Src      string     `json:"src"`
	Dst      string     `json:"dst,omitempty"`
	Size     *int64     `json:"size,omitempty"`
	Mtime    *time.Time `json:"mtime,omitempty"`
	Conflict string     `json:"conflict,omitempty"`
	Trash    string     `json:"trash,omitempty"`
	Note     string     `json:"note,omitempty"`
}

type doneRecord struct {
	Type   string     `json:"type"`
	Seq    int        `json:"seq"`
	Status string     `json:"status"`
	Size   *int64     `json:"size,omitempty"`
	Mtime  *time.Time `json:"mtime,omitempty"`
	Error  string     `json:"error,omitempty"`
}

type footerRecord struct {
	Type       string    `json:"type"`
	FinishedAt time.Time `json:"finished_at"`
	OK         int       `json:"ok"`
	Failed     int       `json:"failed"`
	Skipped    int       `json:"skipped"`
}

type undoneRecord struct {
	Type string    `json:"type"`
	At   time.Time `json:"at"`
}

// anyRecord is the union of all record types, used for reading.
type anyRecord struct {
	Type string `json:"type"`

	Version      *int       `json:"version"`
	RunID        string     `json:"run_id"`
	StartedAt    *time.Time `json:"started_at"`
	Source       string     `json:"source"`
	Dest         string     `json:"dest"`
	FileAction   string     `json:"file_action"`
	OnConflict   string     `json:"on_conflict"`
	FionaVersion string     `json:"fiona_version"`

	Path string `json:"path"`

	Seq      *int       `json:"seq"`
	Kind     string     `json:"kind"`
	Src      string     `json:"src"`
	Dst      string     `json:"dst"`
	Size     *int64     `json:"size"`
	Mtime    *time.Time `json:"mtime"`
	Conflict string     `json:"conflict"`
	Trash    string     `json:"trash"`
	Note     string     `json:"note"`

	Status string `json:"status"`
	Error  string `json:"error"`

	FinishedAt *time.Time `json:"finished_at"`
	OK         int        `json:"ok"`
	Failed     int        `json:"failed"`
	Skipped    int        `json:"skipped"`

	At *time.Time `json:"at"`
}

// NewRunID returns an identifier like 20261010-153012-a1b2: UTC time and 4 random hex characters.
func NewRunID(now time.Time) string {
	suffix := make([]byte, 2)
	rand.Read(suffix)
	return now.UTC().Format(runIDLayout) + "-" + hex.EncodeToString(suffix)
}

// RunTime extracts the start time from a run identifier.
func RunTime(runID string) (time.Time, bool) {
	if len(runID) != len(runIDLayout)+5 || runID[len(runIDLayout)] != '-' {
		return time.Time{}, false
	}
	if _, err := hex.DecodeString(runID[len(runIDLayout)+1:]); err != nil {
		return time.Time{}, false
	}
	t, err := time.Parse(runIDLayout, runID[:len(runIDLayout)])
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
