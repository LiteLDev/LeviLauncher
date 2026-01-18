package preloader

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	EventEnsureStart = "preloader.ensure.start"
	EventEnsureDone  = "preloader.ensure.done"
)

//go:embed PreLoader.dll
var embeddedPreLoader []byte

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
	dir := strings.TrimSpace(versionDir)
	if dir == "" {
		application.Get().Event.Emit(EventEnsureDone, false)
		return false
	}
	dest := filepath.Join(dir, "PreLoader.dll")
	if len(embeddedPreLoader) == 0 {
		return false
	}
	needWrite := true
	if fi, err := os.Stat(dest); err == nil && fi.Size() > 0 {
		if fh, err := fileSHA256(dest); err == nil {
			if bytes.Equal(fh, bytesSHA256(embeddedPreLoader)) {
				needWrite = false
			}
		}
	}
	if needWrite {
		_ = os.MkdirAll(dir, 0755)
		tmp := dest + ".tmp"
		if err := os.WriteFile(tmp, embeddedPreLoader, 0644); err != nil {
			_ = os.Remove(tmp)
			return false
		}
		if err := os.Rename(tmp, dest); err != nil {
			_ = os.Remove(tmp)
			application.Get().Event.Emit(EventEnsureDone, false)
			return false
		}
	}
	return true
}

func EnsureEmbedded(contentDir string, embedded []byte) {
	if strings.TrimSpace(contentDir) == "" {
		return
	}
	dest := filepath.Join(contentDir, "PreLoader.dll")
	if _, err := os.Stat(dest); err == nil {
		return
	}
	if len(embedded) == 0 {
		return
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, embedded, 0644); err != nil {
		_ = os.Remove(tmp)
		return
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return
	}
}
