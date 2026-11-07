package storage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/liteldev/LeviLauncher/internal/fsx"
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
	// Discard marks a Move that only kept a replaced file for an undo: once
	// the migration is committed, Path is removed.
	Discard bool `json:"discard,omitempty"`
	// Deep marks a Move of an entry the migration merges recursively
	// (Backups, Cache): an undo that finds its source recreated merges back
	// recursively too.
	Deep bool `json:"deep,omitempty"`
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
	// The migration is committed once the Commit record is on disk. A journal
	// that cannot be removed now is removed by the next start
	// (recoverJournal); undoing a committed migration here would leave a
	// journal that says it is done.
	_ = os.Remove(j.path)
	return nil
}

// rollback undoes every change and removes the journal. When a change cannot
// be undone, the journal is kept for the next start.
func (j *journal) rollback() error {
	j.close()
	if err := undo(j.path, j.records); err != nil {
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
			// Committed, but the process ended before it removed the files
			// it had kept for an undo.
			for _, d := range records {
				if d.Op == opMove && d.Discard {
					_ = os.Remove(d.Path)
				}
			}
			return os.Remove(path)
		}
	}
	if err := undo(path, records); err != nil {
		return err
	}
	return os.Remove(path)
}

// undo undoes the records from the last one back. After each step it
// shortens the journal to the records still in effect, so that an undo that
// fails or is cut off resumes where it stopped: a step undone twice could
// act on what an earlier undo step put back.
func undo(path string, records []record) error {
	for i := len(records) - 1; i >= 0; i-- {
		if err := undoRecord(records[i]); err != nil {
			return fmt.Errorf("undo migration: %w", err)
		}
		if err := rewriteJournal(path, records[:i]); err != nil {
			return fmt.Errorf("undo migration: %w", err)
		}
	}
	return nil
}

func rewriteJournal(path string, records []record) error {
	var b []byte
	for _, r := range records {
		line, err := json.Marshal(r)
		if err != nil {
			return err
		}
		b = append(append(b, line...), '\n')
	}
	return fsx.WriteFileAtomic(path, b, 0o644)
}

func undoRecord(r record) error {
	switch r.Op {
	case opMove:
		return undoMove(r.From, r.Path, r.Deep)
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

// undoMove moves path back to from. When something was put at from after the
// move (an older launcher that ran between an interrupted migration and this
// start recreates its directories and config.json), nothing is overwritten:
// directories are merged back (recursively for deep entries), and where a
// name is taken the entry moved back is the user's: what appeared since is
// kept next to it under a "-legacy" name. Each step leaves a state from
// which the undo can be run again.
func undoMove(from, path string, deep bool) error {
	if !exists(path) {
		return nil
	}
	if !exists(from) || samePath(from, path) {
		return renameRetry(path, from)
	}
	if isDir(from) && isDir(path) && !isLink(from) && !isLink(path) {
		if !isNotEmpty(from) {
			if err := os.Remove(from); err != nil {
				return err
			}
			return renameRetry(path, from)
		}
		ents, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, e := range ents {
			src, dst := filepath.Join(path, e.Name()), filepath.Join(from, e.Name())
			if exists(dst) {
				if deep && isDir(src) && isDir(dst) && !isLink(src) && !isLink(dst) {
					if err := undoMove(dst, src, true); err != nil {
						return err
					}
					continue
				}
				if err := renameRetry(dst, uniqueName(dst, "-legacy")); err != nil {
					return err
				}
			}
			if err := renameRetry(src, dst); err != nil {
				return err
			}
		}
		return os.Remove(path)
	}
	if err := renameRetry(from, uniqueName(from, "-legacy")); err != nil {
		return fmt.Errorf("cannot move %s back: %w", path, err)
	}
	return renameRetry(path, from)
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func isNotEmpty(dir string) bool {
	ents, err := os.ReadDir(dir)
	return err == nil && len(ents) > 0
}
