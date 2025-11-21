package vcruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	EventEnsureStart    = "vcruntime.ensure.start"
	EventEnsureProgress = "vcruntime.ensure.progress"
	EventEnsureDone     = "vcruntime.ensure.done"
)

type EnsureProgress struct {
	Downloaded int64
	Total      int64
}

//go:embed vcruntime140_1.dll
var embeddedVcruntime []byte

func bytesSHA256(b []byte) []byte { h := sha256.Sum256(b); return h[:] }

func fileSHA256(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func EnsureForVersion(ctx context.Context, versionDir string) bool {
	if strings.TrimSpace(versionDir) == "" {
		return false
	}
	dest := filepath.Join(versionDir, "vcruntime140_1.dll")

	if len(embeddedVcruntime) > 0 {
		needWrite := true
		if fi, err := os.Stat(dest); err == nil && fi.Size() > 0 {
			if fh, err := fileSHA256(dest); err == nil {
				if bytes.Equal(fh, bytesSHA256(embeddedVcruntime)) {
					needWrite = false
				}
			}
		}
		if needWrite {
			_ = os.MkdirAll(versionDir, 0755)
			tmp := dest + ".tmp"
			if err := os.WriteFile(tmp, embeddedVcruntime, 0644); err != nil {
				_ = os.Remove(tmp)
				return false
			}
			if err := os.Rename(tmp, dest); err != nil {
				_ = os.Remove(tmp)
				return false
			}
		}
		return true
	}

	if _, err := os.Stat(dest); err == nil {
		return true
	}
	return false
}

func EnsureEmbedded(contentDir string, embedded []byte) {
	if strings.TrimSpace(contentDir) == "" {
		return
	}
	dest := filepath.Join(contentDir, "vcruntime140_1.dll")
	if _, err := os.Stat(dest); err == nil {
		return
	}
	if len(embedded) == 0 {
		return
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, embedded, 0o644); err != nil {
		_ = os.Remove(tmp)
		return
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return
	}
}
