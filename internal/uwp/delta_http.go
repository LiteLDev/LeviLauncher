package uwp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// errNoRange reports a server that cannot serve byte ranges; a delta upgrade
// is then impossible and the caller falls back to a full download.
var errNoRange = errors.New("server does not support byte ranges")

// httpRangeReader reads a remote file with HTTP Range requests. It never
// downloads more than asked and counts what it transferred.
type httpRangeReader struct {
	ctx        context.Context
	client     *http.Client
	url        string
	size       int64
	downloaded atomic.Int64
}

func newHTTPRangeReader(ctx context.Context, client *http.Client, url string) (*httpRangeReader, error) {
	r := &httpRangeReader{ctx: ctx, client: client, url: url}
	resp, err := r.get(0, 0)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	_, _, total, err := parseContentRange(resp.Header.Get("Content-Range"))
	if err != nil || total <= 0 {
		return nil, errNoRange
	}
	r.size = total
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 16))
	return r, nil
}

func (r *httpRangeReader) Size() int64 { return r.size }

// get requests [start, end] and returns a 206 response.
func (r *httpRangeReader) get(start, end int64) (*http.Response, error) {
	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	// A transparently decompressed body would not match the byte range.
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil, errNoRange
		}
		return nil, fmt.Errorf("range request HTTP %d", resp.StatusCode)
	}
	first, last, _, err := parseContentRange(resp.Header.Get("Content-Range"))
	if err != nil || first != start || last != end {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected Content-Range %q for %d-%d", resp.Header.Get("Content-Range"), start, end)
	}
	return resp, nil
}

// ReadAt fills p from offset off, retrying transient failures.
func (r *httpRangeReader) ReadAt(p []byte, off int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if off < 0 || off >= r.size {
		return 0, io.EOF
	}
	want := p
	if remain := r.size - off; int64(len(want)) > remain {
		want = want[:remain]
	}
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			select {
			case <-r.ctx.Done():
				return 0, r.ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		resp, err := r.get(off, off+int64(len(want))-1)
		if err != nil {
			if errors.Is(err, errNoRange) || r.ctx.Err() != nil {
				return 0, err
			}
			lastErr = err
			continue
		}
		n, err := io.ReadFull(resp.Body, want)
		resp.Body.Close()
		r.downloaded.Add(int64(n))
		if err == nil {
			if len(want) < len(p) {
				return n, io.EOF
			}
			return n, nil
		}
		if r.ctx.Err() != nil {
			return 0, r.ctx.Err()
		}
		lastErr = err
	}
	return 0, lastErr
}

func parseContentRange(value string) (first, last, total int64, err error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "bytes ") {
		return 0, 0, 0, fmt.Errorf("bad Content-Range %q", value)
	}
	spec, size, ok := strings.Cut(strings.TrimPrefix(value, "bytes "), "/")
	if !ok {
		return 0, 0, 0, fmt.Errorf("bad Content-Range %q", value)
	}
	a, b, ok := strings.Cut(spec, "-")
	if !ok {
		return 0, 0, 0, fmt.Errorf("bad Content-Range %q", value)
	}
	if first, err = strconv.ParseInt(a, 10, 64); err != nil {
		return
	}
	if last, err = strconv.ParseInt(b, 10, 64); err != nil {
		return
	}
	if size == "*" {
		return first, last, -1, nil
	}
	total, err = strconv.ParseInt(size, 10, 64)
	return
}

// chunkCache serves the many small reads of zip metadata from a few large
// range requests. Bulk block data bypasses it.
type chunkCache struct {
	src       io.ReaderAt
	size      int64
	chunkSize int64
	mu        sync.Mutex
	chunks    map[int64][]byte
	order     []int64
	limit     int
}

func newChunkCache(src io.ReaderAt, size int64) *chunkCache {
	return &chunkCache{src: src, size: size, chunkSize: 512 << 10, chunks: map[int64][]byte{}, limit: 16}
}

func (c *chunkCache) chunk(index int64) ([]byte, error) {
	c.mu.Lock()
	if data, ok := c.chunks[index]; ok {
		c.mu.Unlock()
		return data, nil
	}
	c.mu.Unlock()
	start := index * c.chunkSize
	n := c.chunkSize
	if start+n > c.size {
		n = c.size - start
	}
	data := make([]byte, n)
	if _, err := c.src.ReadAt(data, start); err != nil && err != io.EOF {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.chunks[index]; !ok {
		c.chunks[index] = data
		c.order = append(c.order, index)
		if len(c.order) > c.limit {
			delete(c.chunks, c.order[0])
			c.order = c.order[1:]
		}
	}
	return data, nil
}

func (c *chunkCache) ReadAt(p []byte, off int64) (int, error) {
	if off >= c.size {
		return 0, io.EOF
	}
	read := 0
	for read < len(p) && off+int64(read) < c.size {
		pos := off + int64(read)
		data, err := c.chunk(pos / c.chunkSize)
		if err != nil {
			return read, err
		}
		read += copy(p[read:], data[pos%c.chunkSize:])
	}
	if read < len(p) {
		return read, io.EOF
	}
	return read, nil
}
