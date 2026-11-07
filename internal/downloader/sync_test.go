package downloader

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/fsx"
)

func rangeServer(t *testing.T, payload string, block <-chan struct{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := 0
		if v := r.Header.Get("Range"); v != "" {
			n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(v, "bytes="), "-"))
			if err != nil {
				t.Errorf("bad range %q", v)
			}
			start = n
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)-start))
			w.WriteHeader(http.StatusPartialContent)
		} else {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		}
		half := start + (len(payload)-start)/2
		_, _ = w.Write([]byte(payload[start:half]))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		if block != nil {
			select {
			case <-block:
			case <-r.Context().Done():
				return
			}
		}
		_, _ = w.Write([]byte(payload[half:]))
	}))
}

func TestDownloadResumesAndVerifies(t *testing.T) {
	payload := strings.Repeat("minecraft", 1000)
	srv := rangeServer(t, payload, nil)
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "game.msixvc")
	if err := os.WriteFile(fsx.PartialPath(dest), []byte(payload[:100]), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := md5.Sum([]byte(payload))
	var last DownloadProgress
	err := Download(context.Background(), srv.URL, dest, SyncOptions{
		MD5:      hex.EncodeToString(sum[:]),
		Progress: func(p DownloadProgress) { last = p },
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dest)
	if err != nil || string(data) != payload {
		t.Fatalf("published file wrong: %v", err)
	}
	if last.Downloaded != int64(len(payload)) || last.Total != int64(len(payload)) {
		t.Fatalf("last progress = %+v", last)
	}
	if _, err := os.Stat(dest + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("lock file left behind: %v", err)
	}
}

func TestDownloadCancelKeepsPartial(t *testing.T) {
	payload := strings.Repeat("x", 64*1024)
	block := make(chan struct{})
	srv := rangeServer(t, payload, block)
	defer srv.Close()
	defer close(block) // before srv.Close, which waits for the handler
	dest := filepath.Join(t.TempDir(), "game.appx")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := make(chan error, 1)
	go func() { got <- Download(ctx, srv.URL, dest, SyncOptions{}) }()
	// The server sends half the file, then stalls until cancelled.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if fi, err := os.Stat(fsx.PartialPath(dest)); err == nil && fi.Size() > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no data written")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-got:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Download did not return after cancel")
	}
	fi, err := os.Stat(fsx.PartialPath(dest))
	if err != nil || fi.Size() == 0 {
		t.Fatalf("partial file not kept: %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("unfinished file was published")
	}
}

func TestDownloadBusyWhenLocked(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "a.msixvc")
	unlock, err := lockDownload(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if err := Download(context.Background(), "http://127.0.0.1:1/never", dest, SyncOptions{}); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
}

func TestDownloadReportsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	err := Download(context.Background(), srv.URL, filepath.Join(t.TempDir(), "x"), SyncOptions{})
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want HTTP 404", err)
	}
}
