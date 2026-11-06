package gitproto

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func pkt(lines ...string) []byte {
	w := new(PktLineWriter)
	for _, l := range lines {
		switch l {
		case "0000":
			w.WriteFlush()
		case "0001":
			w.WriteDelimiter()
		case "0002":
			w.WriteResponseEnd()
		default:
			w.WriteLine(l)
		}
	}
	return w.Bytes()
}

func band(ch byte, data []byte) []byte {
	return new(PktLineWriter).WriteData(append([]byte{ch}, data...)).Bytes()
}

const (
	id1 = "1111111111111111111111111111111111111111"
	id2 = "2222222222222222222222222222222222222222"
	id3 = "3333333333333333333333333333333333333333"
	id4 = "4444444444444444444444444444444444444444"
	id5 = "5555555555555555555555555555555555555555"
	id6 = "6666666666666666666666666666666666666666"
	idA = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
)

// fakeRemote is a minimal protocol v2 upload-pack server.
type fakeRemote struct {
	t       *testing.T
	prefix  string // e.g. "/repo.git"
	caps    []string
	refs    []string // ls-refs response lines
	fetch   func(w http.ResponseWriter, body []byte)
	mu      sync.Mutex
	gets    []*http.Request
	posts   []*http.Request
	bodies  [][]byte
	failGet []int // status codes returned for the first GETs
	service bool  // send "# service=" preamble
}

func (f *fakeRemote) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == f.prefix+"/info/refs":
		f.gets = append(f.gets, r)
		if len(f.failGet) > 0 {
			code := f.failGet[0]
			f.failGet = f.failGet[1:]
			w.WriteHeader(code)
			return
		}
		if r.URL.Query().Get("service") != "git-upload-pack" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
		if f.service {
			w.Write(pkt("# service=git-upload-pack", "0000"))
		}
		w.Write(pkt(append(append([]string{"version 2"}, f.caps...), "0000")...))
	case r.Method == http.MethodPost && r.URL.Path == f.prefix+"/git-upload-pack":
		body, _ := io.ReadAll(r.Body)
		f.posts = append(f.posts, r)
		f.bodies = append(f.bodies, body)
		if bytes.HasPrefix(body, pkt("command=ls-refs")) {
			w.Write(pkt(append(append([]string{}, f.refs...), "0000")...))
			return
		}
		if f.fetch != nil {
			f.fetch(w, body)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

var defaultCaps = []string{"agent=git/github-2.0", "ls-refs=unborn", "fetch=shallow wait-for-done filter", "server-option", "object-format=sha1"}

func newFake(t *testing.T) (*fakeRemote, *httptest.Server) {
	f := &fakeRemote{t: t, prefix: "/repo.git", caps: defaultCaps, service: true}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv
}

func recordingSleep(c *Client) *[]time.Duration {
	var sleeps []time.Duration
	c.sleep = func(ctx context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		return nil
	}
	return &sleeps
}

func TestPktLineRoundTrip(t *testing.T) {
	data := pkt("hello", "0001", "world", "0000", "0002")
	if want := "000ahello\n0001000aworld\n00000002"; string(data) != want {
		t.Fatalf("got %q want %q", data, want)
	}
	r := NewPktLineReader(bytes.NewReader(data))
	var got []string
	for {
		typ, err := r.Read()
		if err != nil {
			t.Fatal(err)
		}
		if typ == PktEndOfStream {
			break
		}
		if typ == PktData {
			got = append(got, r.PayloadText())
		} else {
			got = append(got, typ.String())
		}
	}
	want := []string{"hello", "Delimiter", "world", "Flush", "ResponseEnd"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if r.BytesRead != int64(len(data)) {
		t.Fatalf("BytesRead %d want %d", r.BytesRead, len(data))
	}
}

func TestPktLineErrors(t *testing.T) {
	for _, in := range []string{"00", "zzzz", "0003", "fff1", "0009abc", "000A1234"} {
		r := NewPktLineReader(strings.NewReader(in))
		_, err := r.Read()
		var pe *ProtocolError
		if !errors.As(err, &pe) {
			t.Errorf("%q: expected ProtocolError, got %v", in, err)
		}
	}
	r := NewPktLineReader(strings.NewReader("000A12345\n"))
	if typ, err := r.Read(); err != nil || typ != PktData || r.PayloadText() != "12345" {
		t.Fatalf("upper-case length: %v %v %q", typ, err, r.PayloadText())
	}
}

func TestListTags(t *testing.T) {
	f, srv := newFake(t)
	f.refs = []string{
		id1 + " refs/tags/v1.0.0",
		id2 + " refs/tags/v1.1.0 peeled:" + idA,
		id3 + " refs/tags/v1.2",   // -> 1.2.0
		id4 + " refs/tags/v1.2.0", // canonical spelling wins
		id5 + " refs/tags/v2.0.0+build",
		id6 + " refs/tags/v2.0", // ordinally smaller than v2.0.0+build
		id1 + " refs/tags/vfoo",
		id1 + " refs/tags/v01.0.0",
		id1 + " refs/heads/main", // not requested; filtered again
		id1 + " refs/tags/v3.0.0-rc.1",
	}
	c := &Client{}
	tags, err := c.ListTags(context.Background(), srv.URL+"/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	want := []TagRef{
		{Name: "v1.0.0", Version: "1.0.0", Commit: id1},
		{Name: "v1.1.0", Version: "1.1.0", Commit: strings.ToLower(idA)},
		{Name: "v1.2.0", Version: "1.2.0", Commit: id4},
		{Name: "v2.0", Version: "2.0.0", Commit: id6},
		{Name: "v3.0.0-rc.1", Version: "3.0.0-rc.1", Commit: id1},
	}
	if !reflect.DeepEqual(tags, want) {
		t.Fatalf("got %+v\nwant %+v", tags, want)
	}

	if len(f.gets) != 1 || len(f.posts) != 1 {
		t.Fatalf("gets %d posts %d", len(f.gets), len(f.posts))
	}
	g := f.gets[0]
	if g.Header.Get("Git-Protocol") != "version=2" || g.Header.Get("User-Agent") != UserAgent {
		t.Errorf("GET headers: %v", g.Header)
	}
	p := f.posts[0]
	if p.Header.Get("Git-Protocol") != "version=2" || p.Header.Get("User-Agent") != UserAgent ||
		p.Header.Get("Content-Type") != "application/x-git-upload-pack-request" ||
		p.Header.Get("Accept") != "application/x-git-upload-pack-result" {
		t.Errorf("POST headers: %v", p.Header)
	}
	wantBody := "0014command=ls-refs\n00010009peel\n001bref-prefix refs/tags/v\n0000"
	if string(f.bodies[0]) != wantBody {
		t.Errorf("body %q want %q", f.bodies[0], wantBody)
	}

	// Discovery is cached per repository.
	if _, err := c.ListTags(context.Background(), srv.URL+"/repo.git/"); err != nil {
		t.Fatal(err)
	}
	if len(f.gets) != 1 || len(f.posts) != 2 {
		t.Fatalf("after second call: gets %d posts %d", len(f.gets), len(f.posts))
	}
}

func TestDiscoveryWithoutServicePreamble(t *testing.T) {
	f, srv := newFake(t)
	f.service = false
	f.refs = []string{id1 + " refs/tags/v1.0.0"}
	tags, err := (&Client{}).ListTags(context.Background(), srv.URL+"/repo.git")
	if err != nil || len(tags) != 1 {
		t.Fatalf("%v %v", tags, err)
	}
}

func TestDiscoveryRejects(t *testing.T) {
	cases := map[string][]string{
		"no fetch":      {"ls-refs"},
		"no ls-refs":    {"fetch=shallow"},
		"sha256":        {"ls-refs", "fetch", "object-format=sha256"},
		"object-format": {"ls-refs", "fetch", "object-format"},
	}
	for name, caps := range cases {
		t.Run(name, func(t *testing.T) {
			f, srv := newFake(t)
			f.caps = caps
			c := &Client{}
			sleeps := recordingSleep(c)
			_, err := c.ListTags(context.Background(), srv.URL+"/repo.git")
			var pe *ProtocolError
			if !errors.As(err, &pe) {
				t.Fatalf("expected ProtocolError, got %v", err)
			}
			if len(*sleeps) != 0 {
				t.Fatalf("protocol errors must not be retried")
			}
		})
	}

	// Protocol v0 advertisement.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(pkt("# service=git-upload-pack", "0000"))
		w.Write(new(PktLineWriter).WriteData([]byte(id1 + " HEAD\x00multi_ack\n")).WriteFlush().Bytes())
	}))
	defer srv.Close()
	_, err := (&Client{}).ListTags(context.Background(), srv.URL+"/x.git")
	if err == nil || !strings.Contains(err.Error(), "does not support git protocol version 2") {
		t.Fatalf("v0: %v", err)
	}
}

func TestRedirect(t *testing.T) {
	f := &fakeRemote{t: t, prefix: "/new.git", caps: defaultCaps, service: true}
	f.refs = []string{id1 + " refs/tags/v1.0.0"}
	mux := http.NewServeMux()
	var redirectedHeader string
	mux.HandleFunc("/old.git/info/refs", func(w http.ResponseWriter, r *http.Request) {
		redirectedHeader = r.Header.Get("Git-Protocol")
		http.Redirect(w, r, "/new.git/info/refs?"+r.URL.RawQuery, http.StatusMovedPermanently)
	})
	mux.Handle("/", f)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tags, err := (&Client{}).ListTags(context.Background(), srv.URL+"/old.git")
	if err != nil || len(tags) != 1 {
		t.Fatalf("%v %v", tags, err)
	}
	if redirectedHeader != "version=2" {
		t.Errorf("first request header %q", redirectedHeader)
	}
	if len(f.gets) != 1 || f.gets[0].Header.Get("Git-Protocol") != "version=2" {
		t.Fatalf("Git-Protocol not forwarded on redirect")
	}
	if len(f.posts) != 1 || f.posts[0].URL.Path != "/new.git/git-upload-pack" {
		t.Fatalf("POST not sent to redirect target")
	}
}

func TestRetry(t *testing.T) {
	f, srv := newFake(t)
	f.refs = []string{id1 + " refs/tags/v1.0.0"}
	f.failGet = []int{503, 429}
	c := &Client{}
	sleeps := recordingSleep(c)
	if _, err := c.ListTags(context.Background(), srv.URL+"/repo.git"); err != nil {
		t.Fatal(err)
	}
	if want := []time.Duration{500 * time.Millisecond, time.Second}; !reflect.DeepEqual(*sleeps, want) {
		t.Fatalf("sleeps %v want %v", *sleeps, want)
	}

	f2, srv2 := newFake(t)
	f2.failGet = []int{500, 502, 503, 504}
	c2 := &Client{}
	sleeps2 := recordingSleep(c2)
	_, err := c2.ListTags(context.Background(), srv2.URL+"/repo.git")
	var he *HTTPError
	if !errors.As(err, &he) || he.StatusCode != 503 {
		t.Fatalf("got %v", err)
	}
	if len(f2.gets) != 3 || len(*sleeps2) != 2 {
		t.Fatalf("gets %d sleeps %d", len(f2.gets), len(*sleeps2))
	}

	f3, srv3 := newFake(t)
	f3.failGet = []int{404}
	c3 := &Client{}
	sleeps3 := recordingSleep(c3)
	_, err = c3.ListTags(context.Background(), srv3.URL+"/repo.git")
	if !errors.As(err, &he) || he.StatusCode != 404 || len(*sleeps3) != 0 {
		t.Fatalf("404: %v, sleeps %v", err, *sleeps3)
	}
}

func TestRetryOnConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	c := &Client{}
	sleeps := recordingSleep(c)
	_, err := c.ListTags(context.Background(), url+"/repo.git")
	if err == nil || len(*sleeps) != 2 {
		t.Fatalf("err %v sleeps %v", err, *sleeps)
	}
}

func TestRetryOnTruncatedBody(t *testing.T) {
	// A response whose body ends before Content-Length is an IO error
	// (transient), not a protocol error.
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.Header().Set("Content-Length", "100")
		w.Write([]byte("000eversion 2\n"))
	}))
	defer srv.Close()
	c := &Client{}
	sleeps := recordingSleep(c)
	_, err := c.ListTags(context.Background(), srv.URL+"/repo.git")
	if err == nil || len(*sleeps) != 2 || n != 3 {
		t.Fatalf("err %v sleeps %v requests %d", err, *sleeps, n)
	}
}

func TestIsTransient(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{&HTTPError{StatusCode: 500}, true},
		{&HTTPError{StatusCode: 429}, true},
		{&HTTPError{StatusCode: 404}, false},
		{&transportError{err: io.ErrUnexpectedEOF}, true},
		{&timeoutError{}, true},
		{protocolErrorf("x"), false},
		{context.Canceled, false},
		{errors.New("other"), false},
		{fmt.Errorf("wrapped: %w", &HTTPError{StatusCode: 502}), true},
	}
	for _, c := range cases {
		if got := isTransient(c.err); got != c.want {
			t.Errorf("%v: got %v", c.err, got)
		}
	}
}

func fakePack() []byte {
	var b bytes.Buffer
	b.WriteString("PACK")
	binary.Write(&b, binary.BigEndian, uint32(2))
	binary.Write(&b, binary.BigEndian, uint32(1))
	b.WriteString("objectdata")
	return b.Bytes()
}

func TestFetch(t *testing.T) {
	f, srv := newFake(t)
	pack := fakePack()
	f.fetch = func(w http.ResponseWriter, body []byte) {
		w.Write(pkt("acknowledgments", "NAK", "0001", "shallow-info", "shallow "+id1, "0001", "packfile"))
		w.Write(band(2, []byte("Enumerating objects\n")))
		w.Write(band(1, pack[:6]))
		w.Write(band(2, []byte("Counting objects\n")))
		w.Write(band(1, pack[6:]))
		w.Write([]byte("0000"))
	}
	c := &Client{}
	got, err := c.Fetch(context.Background(), srv.URL+"/repo.git", []string{idA}, FetchOptions{Shallow: true, BlobNone: true})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pack) {
		t.Fatalf("pack %q", got)
	}
	wantBody := string(pkt("command=fetch", "0001", "no-progress", "ofs-delta", "deepen 1", "filter blob:none", "want "+strings.ToLower(idA), "done", "0000"))
	if string(f.bodies[0]) != wantBody {
		t.Fatalf("body %q\nwant %q", f.bodies[0], wantBody)
	}

	// Without options nothing extra is sent.
	if _, err := c.Fetch(context.Background(), srv.URL+"/repo.git", []string{id1, id2}, FetchOptions{}); err != nil {
		t.Fatal(err)
	}
	wantBody = string(pkt("command=fetch", "0001", "no-progress", "ofs-delta", "want "+id1, "want "+id2, "done", "0000"))
	if string(f.bodies[1]) != wantBody {
		t.Fatalf("body %q\nwant %q", f.bodies[1], wantBody)
	}
}

func TestFetchUnsupportedOptions(t *testing.T) {
	f, srv := newFake(t)
	f.caps = []string{"ls-refs", "fetch"}
	f.fetch = func(w http.ResponseWriter, body []byte) {
		w.Write(pkt("packfile"))
		w.Write(band(1, fakePack()))
		w.Write([]byte("0002"))
	}
	if _, err := (&Client{}).Fetch(context.Background(), srv.URL+"/repo.git", []string{id1}, FetchOptions{Shallow: true, BlobNone: true}); err != nil {
		t.Fatal(err)
	}
	wantBody := string(pkt("command=fetch", "0001", "no-progress", "ofs-delta", "want "+id1, "done", "0000"))
	if string(f.bodies[0]) != wantBody {
		t.Fatalf("body %q\nwant %q", f.bodies[0], wantBody)
	}
}

func TestFetchErrors(t *testing.T) {
	cases := map[string]struct {
		resp []byte
		msg  string
	}{
		"sideband error": {append(append(pkt("packfile"), band(1, []byte("PA"))...), band(3, []byte("upload-pack: not our ref\n"))...), "Remote error: upload-pack: not our ref"},
		"ERR line":       {pkt("ERR not our ref"), "Remote error: not our ref"},
		"ERR in section": {pkt("acknowledgments", "ERR boom"), "Remote error: boom"},
		"no packfile":    {pkt("acknowledgments", "NAK", "0000"), "Fetch response ended without a packfile (section acknowledgments)."},
		"bad channel":    {append(pkt("packfile"), band(4, []byte("x"))...), "Invalid side-band channel 4."},
		"empty band":     {append(pkt("packfile"), []byte("0004")...), "Empty side-band packet."},
		"eof":            {pkt("packfile"), "Unexpected EndOfStream in packfile section."},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f, srv := newFake(t)
			f.fetch = func(w http.ResponseWriter, body []byte) { w.Write(tc.resp) }
			c := &Client{}
			sleeps := recordingSleep(c)
			_, err := c.Fetch(context.Background(), srv.URL+"/repo.git", []string{id1}, FetchOptions{})
			var pe *ProtocolError
			if !errors.As(err, &pe) || err.Error() != tc.msg {
				t.Fatalf("got %v want %q", err, tc.msg)
			}
			if len(*sleeps) != 0 {
				t.Fatal("protocol errors must not be retried")
			}
		})
	}
}

func TestFetchArgs(t *testing.T) {
	c := &Client{}
	if _, err := c.Fetch(context.Background(), "http://invalid", nil, FetchOptions{}); err == nil {
		t.Error("no wants accepted")
	}
	if _, err := c.Fetch(context.Background(), "http://invalid", []string{"HEAD"}, FetchOptions{}); err == nil {
		t.Error("non object id accepted")
	}
	wants := make([]string, MaxWantsPerRequest+1)
	for i := range wants {
		wants[i] = id1
	}
	if _, err := c.Fetch(context.Background(), "http://invalid", wants, FetchOptions{}); err == nil {
		t.Error("too many wants accepted")
	}
}

func TestLsRefsErrors(t *testing.T) {
	for name, refs := range map[string][]string{
		"ERR":       {"ERR access denied"},
		"malformed": {"nothex refs/tags/v1.0.0"},
		"short":     {id1},
	} {
		t.Run(name, func(t *testing.T) {
			f, srv := newFake(t)
			f.refs = refs
			_, err := (&Client{}).ListTags(context.Background(), srv.URL+"/repo.git")
			var pe *ProtocolError
			if !errors.As(err, &pe) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

// fakeProxy tries the proxied URL first, then the direct one.
type fakeProxy struct {
	proxyBase string
	calls     []string
}

func (p *fakeProxy) Try(ctx context.Context, rawURL string, fn func(url string) error) error {
	proxied := p.proxyBase + rawURL[strings.Index(rawURL[len("http://"):], "/")+len("http://"):]
	p.calls = append(p.calls, proxied)
	if err := fn(proxied); err == nil {
		return nil
	}
	p.calls = append(p.calls, rawURL)
	return fn(rawURL)
}

func TestProxy(t *testing.T) {
	// Proxy server serves the repository under the same path.
	pf, psrv := newFake(t)
	pf.refs = []string{id1 + " refs/tags/v1.0.0"}
	df, dsrv := newFake(t)
	df.refs = []string{id2 + " refs/tags/v1.0.0"}

	p := &fakeProxy{proxyBase: psrv.URL}
	c := &Client{Proxy: p}
	tags, err := c.ListTags(context.Background(), dsrv.URL+"/repo.git")
	if err != nil || len(tags) != 1 || tags[0].Commit != id1 {
		t.Fatalf("%v %v", tags, err)
	}
	if len(pf.gets) != 1 || len(pf.posts) != 1 || len(df.gets) != 0 || len(df.posts) != 0 {
		t.Fatalf("proxy gets %d posts %d; direct gets %d posts %d", len(pf.gets), len(pf.posts), len(df.gets), len(df.posts))
	}

	// Failing proxy (after retries) falls back to direct access.
	pf.failGet = []int{502, 502, 502}
	c2 := &Client{Proxy: &fakeProxy{proxyBase: psrv.URL}}
	sleeps := recordingSleep(c2)
	tags, err = c2.ListTags(context.Background(), dsrv.URL+"/repo.git")
	if err != nil || len(tags) != 1 || tags[0].Commit != id2 {
		t.Fatalf("%v %v", tags, err)
	}
	if len(*sleeps) != 2 || len(df.gets) != 1 || len(df.posts) != 1 {
		t.Fatalf("sleeps %v direct gets %d posts %d", *sleeps, len(df.gets), len(df.posts))
	}
}

func TestRepoKey(t *testing.T) {
	cases := map[string]string{
		"https://github.com/LiteLDev/LeviLamina.git":  "https://github.com/liteldev/levilamina",
		"https://GitHub.com/LiteLDev/LeviLamina/":     "https://github.com/liteldev/levilamina",
		"https://gitee.com/LiteLDev/LeviLamina.git":   "https://gitee.com/LiteLDev/LeviLamina.git",
		"https://github.com.evil/LiteLDev/x.git":      "https://github.com.evil/LiteLDev/x.git",
		"https://user@github.com:443/LiteLDev/x.git/": "https://user@github.com:443/liteldev/x",
	}
	for in, want := range cases {
		if got := RepoKey(in); got != want {
			t.Errorf("RepoKey(%q) = %q want %q", in, got, want)
		}
	}
	if RepoURL("github.com/LiteLDev/LeviLamina") != "https://github.com/LiteLDev/LeviLamina.git" {
		t.Error("RepoURL")
	}
}

func TestContextCanceledNotRetried(t *testing.T) {
	_, srv := newFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &Client{}
	sleeps := recordingSleep(c)
	_, err := c.ListTags(ctx, srv.URL+"/repo.git")
	if !errors.Is(err, context.Canceled) || len(*sleeps) != 0 {
		t.Fatalf("err %v sleeps %v", err, *sleeps)
	}
}

func TestCancelDuringBackoff(t *testing.T) {
	f, srv := newFake(t)
	f.failGet = []int{503}
	c := &Client{}
	c.sleep = func(ctx context.Context, d time.Duration) error { return context.Canceled }
	_, err := c.ListTags(context.Background(), srv.URL+"/repo.git")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

// TestNetworkE2E talks to GitHub; run with LIP_NET_E2E=1.
func TestNetworkE2E(t *testing.T) {
	if os.Getenv("LIP_NET_E2E") != "1" {
		t.Skip("set LIP_NET_E2E=1 to run")
	}
	const repo = "https://github.com/LiteLDev/LeviLamina.git"
	ctx := context.Background()
	c := &Client{}
	tags, err := c.ListTags(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	byVersion := map[string]TagRef{}
	for _, tag := range tags {
		byVersion[tag.Version] = tag
	}
	for _, v := range []string{"1.0.0", "1.1.0", "0.13.0"} {
		if _, ok := byVersion[v]; !ok {
			t.Errorf("version %s missing", v)
		}
	}
	t.Logf("%d version tags", len(tags))

	// Compare with git ls-remote when git is available.
	if out, err := exec.Command("git", "ls-remote", "--tags", repo).Output(); err == nil {
		commits := map[string]string{} // tag name -> peeled or direct id
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			id, ref, ok := strings.Cut(line, "\t")
			if !ok {
				continue
			}
			name := strings.TrimPrefix(ref, "refs/tags/")
			if n, ok := strings.CutSuffix(name, "^{}"); ok {
				commits[n] = id
			} else if _, seen := commits[name]; !seen {
				commits[name] = id
			}
		}
		expected := map[string]bool{}
		for name := range commits {
			if v, ok := parseTagVersion("refs/tags/" + name); ok {
				expected[v] = true
			}
		}
		if len(expected) != len(tags) {
			t.Errorf("git ls-remote yields %d versions, ListTags %d", len(expected), len(tags))
		}
		for _, tag := range tags {
			if commits[tag.Name] != tag.Commit {
				t.Errorf("tag %s: commit %s, git ls-remote %s", tag.Name, tag.Commit, commits[tag.Name])
			}
		}
	} else {
		t.Logf("git ls-remote unavailable: %v", err)
	}

	tag := byVersion["1.0.0"]
	pack, err := c.Fetch(ctx, repo, []string{tag.Commit}, FetchOptions{Shallow: true, BlobNone: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(pack) < 12 || string(pack[:4]) != "PACK" || binary.BigEndian.Uint32(pack[4:8]) != 2 || binary.BigEndian.Uint32(pack[8:12]) == 0 {
		t.Fatalf("invalid pack header %x", pack[:min(len(pack), 12)])
	}
	t.Logf("pack: %d bytes, %d objects", len(pack), binary.BigEndian.Uint32(pack[8:12]))
}
