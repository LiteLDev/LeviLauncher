package peeditor

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	EventEnsureStart = "peeditor.ensure.start"
	EventEnsureDone  = "peeditor.ensure.done"
)

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

func PrepareExecutableForLaunch(ctx context.Context, versionDir string, console bool) (string, error) {
	dir := strings.TrimSpace(versionDir)
	if dir == "" {
		return "", fmt.Errorf("version dir is empty")
	}
	c := filepath.Join(dir, "Minecraft.Windows.Console.exe")
	if fileExists(c) {
		_ = os.Remove(c)
	}
	src := filepath.Join(dir, "Minecraft.Windows.exe")
	if !fileExists(src) {
		return "", fmt.Errorf("Minecraft.Windows.exe not found in %s", dir)
	}
	if !SetSubsystem(ctx, src, console) {
		return "", fmt.Errorf("failed to set subsystem for %s", src)
	}
	return src, nil
}

func fileExists(p string) bool {
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		return true
	}
	return false
}
