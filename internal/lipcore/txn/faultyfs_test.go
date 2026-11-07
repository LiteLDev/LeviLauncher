package txn

import (
	"fmt"
	"io"
)

// faultyFS is FaultyFileSystem.cs over the real file system: Create and
// Rename (File.Create and File.Move) fail on demand, as when a file is
// locked by another process or the disk is full. Remove and WriteFile can be
// failed too, through failOther.
type faultyFS struct {
	FS
	// fail is called with ("create", path) or ("move", source) before the
	// call; returning true makes it fail.
	fail func(op, path string) bool
	// failOther is called with ("remove", path) or ("writefile", path).
	failOther func(op, path string) bool
	calls     int
}

func newFaultyFS() *faultyFS { return &faultyFS{FS: OS} }

// failNthCall fails the n-th (1-based) Create or Rename call from now on.
func (f *faultyFS) failNthCall(n int) {
	target := f.calls + n
	f.fail = func(string, string) bool { return f.calls == target }
}

func (f *faultyFS) check(op, path string) error {
	f.calls++
	if f.fail != nil && f.fail(op, path) {
		return fmt.Errorf("injected failure: %s %s", op, path)
	}
	return nil
}

func (f *faultyFS) checkOther(op, path string) error {
	if f.failOther != nil && f.failOther(op, path) {
		return fmt.Errorf("injected failure: %s %s", op, path)
	}
	return nil
}

func (f *faultyFS) Create(name string) (io.WriteCloser, error) {
	if err := f.check("create", name); err != nil {
		return nil, err
	}
	return f.FS.Create(name)
}

func (f *faultyFS) Rename(oldname, newname string) error {
	if err := f.check("move", oldname); err != nil {
		return err
	}
	return f.FS.Rename(oldname, newname)
}

func (f *faultyFS) Remove(name string) error {
	if err := f.checkOther("remove", name); err != nil {
		return err
	}
	return f.FS.Remove(name)
}

func (f *faultyFS) WriteFile(name string, data []byte) error {
	if err := f.checkOther("writefile", name); err != nil {
		return err
	}
	return f.FS.WriteFile(name, data)
}
