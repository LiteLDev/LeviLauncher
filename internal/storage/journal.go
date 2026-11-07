package storage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The journal makes a migration all or nothing, as lip's .lip/journal does
// for a workspace operation: each change is appended and flushed before it is
// made, so a migration that fails or is interrupted is undone from the
// journal, by this process or by the next start.
//
// The journal is JSON lines. Every change is undone by a step that is safe to
// repeat, so an undo that fails part way is simply run again.

const (
	opBegin  = "Begin"
	opMkdir  = "Mkdir"  // path was created
	opMove   = "Move"   // from was renamed to path
	opCopy   = "Copy"   // path was created as a copy; the source is kept
	opCreate = "Create" // file path was created
	opRmdir  = "Rmdir"  // empty directory path was removed
	opCommit = "Commit"
)

type record struct {
	Op   string `json:"op"`
	Path string `json:"path,omitempty"`
	From string `json:"from,omitempty"`
}

type journal struct {
	path    string
	f       *os.File
	records []record
}

func beginJournal(path string) (*journal, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	j := &journal{path: path, f: f}
	if err := j.append(record{Op: opBegin}); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return j, nil
}

func (j *journal) append(r record) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if _, err := j.f.Write(append(b, '\n')); err != nil {
		return err
	}
	if err := j.f.Sync(); err != nil {
		return err
	}
	j.records = append(j.records, r)
	return nil
}

func (j *journal) close() {
	if j.f != nil {
		_ = j.f.Close()
		j.f = nil
	}
}

// commit marks the migration complete and removes the journal.
func (j *journal) commit() error {
	if err := j.append(record{Op: opCommit}); err != nil {
		return err
	}
	j.close()
	return os.Remove(j.path)
}

// rollback undoes every change and removes the journal. When a change cannot
// be undone, the journal is kept for the next start.
func (j *journal) rollback() error {
	j.close()
	if err := undo(j.records); err != nil {
		return err
	}
	return os.Remove(j.path)
}

func readJournal(path string) ([]record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var r record
		// A line cut off by a crash, or anything else unreadable, is ignored.
		if json.Unmarshal(sc.Bytes(), &r) == nil && r.Op != "" {
			out = append(out, r)
		}
	}
	return out, sc.Err()
}

// recoverJournal finishes a migration that an earlier start left behind: a
// committed one only loses its journal, any other is undone.
func recoverJournal(path string) error {
	records, err := readJournal(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, r := range records {
		if r.Op == opCommit {
			return os.Remove(path)
		}
	}
	if err := undo(records); err != nil {
		return err
	}
	return os.Remove(path)
}

func undo(records []record) error {
	var errs []string
	for i := len(records) - 1; i >= 0; i-- {
		if err := undoRecord(records[i]); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("undo migration: %s", strings.Join(errs, "; "))
	}
	return nil
}

func undoRecord(r record) error {
	switch r.Op {
	case opMove:
		if !exists(r.Path) {
			return nil
		}
		if exists(r.From) && !samePath(r.From, r.Path) {
			return fmt.Errorf("cannot move %s back: %s exists", r.Path, r.From)
		}
		return renameRetry(r.Path, r.From)
	case opCopy, opCreate:
		if err := os.RemoveAll(r.Path); err != nil {
			return err
		}
		return nil
	case opMkdir:
		if err := os.Remove(r.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			// Not empty: something else was put there; keep it.
			if isNotEmpty(r.Path) {
				return nil
			}
			return err
		}
		return nil
	case opRmdir:
		return os.MkdirAll(r.Path, 0o755)
	}
	return nil
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func isNotEmpty(dir string) bool {
	ents, err := os.ReadDir(dir)
	return err == nil && len(ents) > 0
}
