package fetch

import (
	"math"
	"os"
)

// The Size methods below report an entry's content size as recorded by the
// archive (or the file system) before the entry is read, so that callers can
// measure copy progress in bytes. ok is false when the key is unknown or the
// size is not known up front.

func (s *zipSource) Size(key string) (int64, bool) {
	zf, ok := s.files[key]
	if !ok || zf.UncompressedSize64 > math.MaxInt64 {
		return 0, false
	}
	return int64(zf.UncompressedSize64), true
}

func (s *sevenZipSource) Size(key string) (int64, bool) {
	f, ok := s.files[key]
	if !ok || f.UncompressedSize > math.MaxInt64 {
		return 0, false
	}
	return int64(f.UncompressedSize), true
}

// Size is unknown for sparse entries: their expanded length is only bounded
// by the header.
func (s *tarSource) Size(key string) (int64, bool) {
	e, ok := s.entries[key]
	if !ok || e.sparse {
		return 0, false
	}
	return e.size, true
}

func (s *SingleFileSource) Size(key string) (int64, bool) {
	if key != "" {
		return 0, false
	}
	fi, err := os.Stat(s.path)
	if err != nil || !fi.Mode().IsRegular() {
		return 0, false
	}
	return fi.Size(), true
}
