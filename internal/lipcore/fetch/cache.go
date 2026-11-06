package fetch

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

// Cache is lip's download cache. Entries live at
// <Root>/<EscapeKey(key)>; writes go to <TempRoot>/<random> then are moved
// into place.
type Cache struct {
	Root     string
	TempRoot string
	// Info, if set, receives lip's cache log lines ("Cache hit for key
	// '<key>'." and "Caching file for key '<key>'...") from GetOrCreate.
	Info func(message string)
}

// DefaultCache returns the cache shared with lip:
// %LOCALAPPDATA%\lip\cache and %LOCALAPPDATA%\lip\temp. On other systems
// the base directory mirrors .NET's Environment.SpecialFolder.
// LocalApplicationData ($XDG_DATA_HOME or ~/.local/share; macOS:
// ~/Library/Application Support).
func DefaultCache() *Cache {
	base := localAppData()
	return &Cache{
		Root:     filepath.Join(base, "lip", "cache"),
		TempRoot: filepath.Join(base, "lip", "temp"),
	}
}

// EscapeKey escapes key like .NET Uri.EscapeDataString: every byte of the
// UTF-8 encoding except RFC 3986 unreserved characters (A-Z a-z 0-9 - _ . ~)
// becomes %XX with uppercase hex. Invalid UTF-8 bytes are treated as U+FFFD,
// as a .NET string decoded from them would be.
func EscapeKey(key string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	b.Grow(len(key) * 3)
	var buf [utf8.UTFMax]byte
	for i := 0; i < len(key); {
		r, size := utf8.DecodeRuneInString(key[i:])
		i += size
		if r < utf8.RuneSelf && isUnreserved(byte(r)) {
			b.WriteByte(byte(r))
			continue
		}
		n := utf8.EncodeRune(buf[:], r) // RuneError encodes as EF BF BD
		for _, c := range buf[:n] {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0xF])
		}
	}
	return b.String()
}

func isUnreserved(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' ||
		c == '-' || c == '_' || c == '.' || c == '~'
}

// BlobKey is the cache key for a git blob: "git-blob:<oid>".
func BlobKey(oid string) string { return "git-blob:" + oid }

// Path returns the file path for key (whether or not it exists).
func (c *Cache) Path(key string) string {
	return filepath.Join(c.Root, EscapeKey(key))
}

// Get returns the cached file path for key if present (lip TryGetFile:
// only regular files count; never creates anything).
func (c *Cache) Get(key string) (path string, ok bool) {
	p := c.Path(key)
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		return "", false
	}
	return p, true
}

// Put writes an entry via write (to a temp file, then moved into place)
// and returns its path. A failed write leaves no entry. As in lip, an
// existing entry is never replaced: if the key is already cached, write is
// not called, and if another writer wins the race the first entry is kept.
func (c *Cache) Put(key string, write func(w io.Writer) error) (string, error) {
	return c.put(key, func(f *os.File) error { return write(f) })
}

// GetOrCreate is lip's CacheService.GetOrCreateFile: it returns the cached
// file for key, creating it with write (as Put) when missing, and logs to
// Info.
func (c *Cache) GetOrCreate(key string, write func(w io.Writer) error) (string, error) {
	return c.getOrCreate(key, func(f *os.File) error { return write(f) })
}

func (c *Cache) getOrCreate(key string, write func(f *os.File) error) (string, error) {
	// Concurrent creations of one key in this process wait for each other
	// instead of downloading twice.
	unlock := lockKey(c.Path(key))
	defer unlock()
	if p, ok := c.Get(key); ok {
		c.info(fmt.Sprintf("Cache hit for key '%s'.", key))
		return p, nil
	}
	c.info(fmt.Sprintf("Caching file for key '%s'...", key))
	return c.put(key, write)
}

// keyLocks holds a refcounted mutex per cache path being created.
var (
	keyLocksMu sync.Mutex
	keyLocks   = map[string]*keyLock{}
)

type keyLock struct {
	mu   sync.Mutex
	refs int // guarded by keyLocksMu
}

func lockKey(path string) (unlock func()) {
	keyLocksMu.Lock()
	l := keyLocks[path]
	if l == nil {
		l = &keyLock{}
		keyLocks[path] = l
	}
	l.refs++
	keyLocksMu.Unlock()
	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		keyLocksMu.Lock()
		if l.refs--; l.refs == 0 {
			delete(keyLocks, path)
		}
		keyLocksMu.Unlock()
	}
}

func (c *Cache) info(msg string) {
	if c.Info != nil {
		c.Info(msg)
	}
}

func (c *Cache) put(key string, write func(f *os.File) error) (string, error) {
	target := c.Path(key)
	if p, ok := c.Get(key); ok {
		return p, nil
	}
	if err := os.MkdirAll(c.Root, 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(c.TempRoot, 0o755); err != nil {
		return "", err
	}

	tmp := filepath.Join(c.TempRoot, newGUID())
	f, err := os.OpenFile(tmp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp) // no-op once moved

	werr := write(f)
	cerr := f.Close()
	if werr != nil {
		return "", werr
	}
	if cerr != nil {
		return "", cerr
	}

	if err := moveNoReplace(tmp, target); err != nil {
		// Another process has already created the entry.
		if p, ok := c.Get(key); ok {
			return p, nil
		}
		return "", err
	}
	return target, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// newGUID returns a random version-4 GUID in .NET's "D" format.
func newGUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Clean deletes the cache directory, clearing read-only attributes first.
// A missing directory is not an error. The temp directory is left alone,
// as in lip.
func (c *Cache) Clean() error {
	fi, err := os.Lstat(c.Root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.IsDir() {
		if err := clearReadOnly(c.Root); err != nil {
			return err
		}
	}
	return os.RemoveAll(c.Root)
}

// clearReadOnly makes dir and everything below it writable by the owner
// (on Windows this clears FILE_ATTRIBUTE_READONLY). Symbolic links and
// junctions are not followed.
func clearReadOnly(dir string) error {
	if err := makeWritable(dir, true); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.Type()&fs.ModeSymlink != 0 || e.Type()&fs.ModeIrregular != 0 {
			continue
		}
		if e.IsDir() {
			if err := clearReadOnly(p); err != nil {
				return err
			}
			continue
		}
		if err := makeWritable(p, false); err != nil {
			return err
		}
	}
	return nil
}

func makeWritable(p string, isDir bool) error {
	fi, err := os.Lstat(p)
	if err != nil {
		return err
	}
	mode := fi.Mode().Perm()
	want := mode | 0o200
	if isDir {
		want |= 0o700
	}
	if want == mode {
		return nil
	}
	return os.Chmod(p, want)
}
