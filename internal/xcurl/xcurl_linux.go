//go:build linux

package xcurl

import (
    "bytes"
    "context"
    "crypto/sha256"
    _ "embed"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"
)

const (
    EventEnsureStart    = "xcurl.ensure.start"
    EventEnsureProgress = "xcurl.ensure.progress"
    EventEnsureDone     = "xcurl.ensure.done"
)

//go:embed XCurl.dll
var embeddedXCurl []byte

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
    dest := filepath.Join(versionDir, "XCurl.dll")

    if len(embeddedXCurl) > 0 {
        needWrite := true
        if fi, err := os.Stat(dest); err == nil && fi.Size() > 0 {
            if fh, err := fileSHA256(dest); err == nil {
                if bytes.Equal(fh, bytesSHA256(embeddedXCurl)) {
                    needWrite = false
                }
            }
        }
        if needWrite {
            _ = os.MkdirAll(versionDir, 0755)
            tmp := dest + ".tmp"
            if err := os.WriteFile(tmp, embeddedXCurl, 0644); err != nil {
                _ = os.Remove(tmp)
                return false
            }
            if err := os.Rename(tmp, dest); err != nil {
                _ = os.Remove(tmp)
                return false
            }
            ensureCACert(versionDir)
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
    dest := filepath.Join(contentDir, "XCurl.dll")
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

func ensureCACert(dir string) {
    pemPath := filepath.Join(dir, "cacert.pem")
    if fi, err := os.Stat(pemPath); err == nil && fi.Size() > 0 {
        return
    }
    client := &http.Client{Timeout: 15 * time.Second}
    req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://curl.se/ca/cacert.pem", nil)
    if err != nil {
        return
    }
    resp, err := client.Do(req)
    if err != nil {
        return
    }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        return
    }
    tmp := pemPath + ".tmp"
    f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
    if err != nil {
        _ = os.Remove(tmp)
        return
    }
    if _, err := io.Copy(f, resp.Body); err != nil {
        f.Close()
        _ = os.Remove(tmp)
        return
    }
    f.Close()
    if err := os.Rename(tmp, pemPath); err != nil {
        _ = os.Remove(tmp)
        return
    }
}