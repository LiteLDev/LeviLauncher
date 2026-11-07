package fetch

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

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

// cleanDir deletes root as HTTPCache.CleanProgress describes.
func cleanDir(ctx context.Context, root string, progress func(done, total int64)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	report := func(done, total int64) {
		if progress != nil {
			progress(done, total)
		}
	}
	fi, err := os.Lstat(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	entries := []string{root}
	if fi.IsDir() {
		entries, err = clearReadOnly(root, nil)
		if err != nil {
			return err
		}
	}
	total := int64(len(entries))
	report(0, total)
	for i, p := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			if err := os.RemoveAll(root); err != nil {
				return err
			}
			report(total, total)
			return nil
		}
		report(int64(i)+1, total)
	}
	return nil
}

// clearReadOnly makes dir and everything below it writable by the owner
// (on Windows this clears FILE_ATTRIBUTE_READONLY) and appends the paths
// of dir and everything below it to list in deletion order (children
// before their directory, dir last). Symbolic links and junctions are
// listed but not followed.
func clearReadOnly(dir string, list []string) ([]string, error) {
	if err := makeWritable(dir, true); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.Type()&fs.ModeSymlink != 0 || e.Type()&fs.ModeIrregular != 0 {
			list = append(list, p)
			continue
		}
		if e.IsDir() {
			if list, err = clearReadOnly(p, list); err != nil {
				return nil, err
			}
			continue
		}
		if err := makeWritable(p, false); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return append(list, dir), nil
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
