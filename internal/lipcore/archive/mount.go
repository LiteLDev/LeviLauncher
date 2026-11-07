package archive

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// Mount makes several sources one (MountSource.cs): the files of source i
// are under the directory "i", so its key k is "i/k". Used for the assets of
// an entry (protocol/workspace.md §4 step 2). Keys are in mount order, each
// source's keys in its own order.
func Mount(sources ...Source) Source {
	m := &mountSource{}
	for i, s := range sources {
		m.mounts = append(m.mounts, mount{name: strconv.Itoa(i), source: s})
	}
	return m
}

type mount struct {
	name   string
	source Source
}

type mountSource struct{ mounts []mount }

func (m *mountSource) Keys() []string {
	var keys []string
	for _, mt := range m.mounts {
		for _, k := range mt.source.Keys() {
			keys = append(keys, mt.name+"/"+k)
		}
	}
	return keys
}

// Load opens the mounted sources that open on first use, failing with the
// first failure.
func (m *mountSource) Load() (Source, error) {
	for _, mt := range m.mounts {
		if l, ok := mt.source.(loader); ok {
			if _, err := l.Load(); err != nil {
				return nil, err
			}
		}
	}
	return m, nil
}

func (m *mountSource) Open(key string) (io.ReadCloser, error) {
	s, inner, err := m.resolve(key)
	if err != nil {
		return nil, err
	}
	return s.Open(inner)
}

func (m *mountSource) Size(key string) (int64, bool) {
	s, inner, err := m.resolve(key)
	if err != nil {
		return 0, false
	}
	return s.Size(inner)
}

func (m *mountSource) resolve(key string) (Source, string, error) {
	if name, inner, ok := strings.Cut(key, "/"); ok {
		for _, mt := range m.mounts {
			if mt.name == name {
				return mt.source, inner, nil
			}
		}
	}
	return nil, "", &noFileError{"No asset of the package has the file '" + key + "'."}
}

// Defer returns a source that is opened on first use (DeferredSource.cs):
// installed packages are rebuilt from the lock without touching their
// archives; the archive is only opened when something reads it. open runs
// at most once, even when used concurrently, and its failure is kept, as
// with lip's Lazy<Task>.
func Defer(open func() (Source, error)) Source {
	return &DeferredSource{open: open}
}

// DeferredSource is the source Defer returns.
type DeferredSource struct {
	once   sync.Once
	open   func() (Source, error)
	source Source
	err    error
}

// Load opens the underlying source, once, and returns it or the failure to
// open it. Keys returns no keys when it fails, where lip's Keys throws: list
// keys with ListKeys to get the failure.
func (d *DeferredSource) Load() (Source, error) {
	d.once.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				d.err = fmt.Errorf("archive: opening the source panicked: %v", r)
				panic(r)
			}
		}()
		d.source, d.err = d.open()
		d.open = nil
	})
	return d.source, d.err
}

func (d *DeferredSource) Keys() []string {
	s, err := d.Load()
	if err != nil {
		return nil
	}
	return s.Keys()
}

func (d *DeferredSource) Open(key string) (io.ReadCloser, error) {
	s, err := d.Load()
	if err != nil {
		return nil, err
	}
	return s.Open(key)
}

func (d *DeferredSource) Size(key string) (int64, bool) {
	s, err := d.Load()
	if err != nil {
		return 0, false
	}
	return s.Size(key)
}

// loader is a source that may fail to open (DeferredSource, or a mount of
// one).
type loader interface {
	Load() (Source, error)
}

// ListKeys returns the keys of a source, or the failure to open it when it
// is opened on first use (Defer, or a Mount of such sources), which Keys
// cannot report.
func ListKeys(src Source) ([]string, error) {
	if l, ok := src.(loader); ok {
		if _, err := l.Load(); err != nil {
			return nil, err
		}
	}
	return src.Keys(), nil
}
