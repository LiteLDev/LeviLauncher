package gitproto

// Ports of lip 0.38.0-beta.1 tests/Lip.Core.Tests/Git/GitRemoteClientTests.cs.

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/lipcore/gitpack"
)

const testRepoURL = "https://example.com/owner/repo.git"

var (
	sReadme      = []byte("# readme\n")
	sScript      = []byte("#!/bin/sh\necho hi\n")
	sPackageFile = []byte("format: 4\n")
)

// --- pack building (tests/Lip.Core.Tests/Git/PackBuilder.cs) ---

func objectID(t gitpack.ObjectType, data []byte) string { return gitpack.HashObject(t, data) }

type packBuilder struct {
	body  bytes.Buffer
	count uint32
}

func (b *packBuilder) header(typ byte, size int) int64 {
	if b.body.Len() == 0 {
		b.body.WriteString("PACK")
		binary.Write(&b.body, binary.BigEndian, uint32(2))
		binary.Write(&b.body, binary.BigEndian, uint32(0))
	}
	off := int64(b.body.Len())
	c := typ<<4 | byte(size&0x0f)
	for size >>= 4; size > 0; size >>= 7 {
		b.body.WriteByte(c | 0x80)
		c = byte(size & 0x7f)
	}
	b.body.WriteByte(c)
	b.count++
	return off
}

func (b *packBuilder) deflate(data []byte) {
	zw := zlib.NewWriter(&b.body)
	zw.Write(data)
	zw.Close()
}

func (b *packBuilder) add(t gitpack.ObjectType, data []byte) int64 {
	off := b.header(byte(t), len(data))
	b.deflate(data)
	return off
}

func (b *packBuilder) addOfsDelta(base int64, delta []byte) {
	off := b.header(6, len(delta))
	d := off - base
	var enc []byte
	enc = append(enc, byte(d&0x7f))
	for d >>= 7; d > 0; d >>= 7 {
		d--
		enc = append([]byte{byte(0x80 | d&0x7f)}, enc...)
	}
	b.body.Write(enc)
	b.deflate(delta)
}

func (b *packBuilder) build() []byte {
	if b.body.Len() == 0 {
		b.body.WriteString("PACK")
		binary.Write(&b.body, binary.BigEndian, uint32(2))
		binary.Write(&b.body, binary.BigEndian, uint32(0))
	}
	out := bytes.Clone(b.body.Bytes())
	binary.BigEndian.PutUint32(out[8:], b.count)
	sum := sha1.Sum(out)
	return append(out, sum[:]...)
}

func deltaVarint(n int) []byte {
	var out []byte
	for {
		c := byte(n & 0x7f)
		n >>= 7
		if n > 0 {
			out = append(out, c|0x80)
			continue
		}
		return append(out, c)
	}
}

// copyInsertDelta copies base[0:copyLen] and inserts insert.
func copyInsertDelta(baseLen, resultLen, copyLen int, insert []byte) []byte {
	d := append(deltaVarint(baseLen), deltaVarint(resultLen)...)
	d = append(d, 0x80|0x10, byte(copyLen)) // copy, offset 0, 1 size byte
	d = append(d, byte(len(insert)))
	return append(d, insert...)
}

type treeItem struct{ mode, name, oid string }

func treeData(items ...treeItem) []byte {
	var b bytes.Buffer
	for _, it := range items {
		b.WriteString(it.mode + " " + it.name + "\x00")
		raw, _ := hex.DecodeString(it.oid)
		b.Write(raw)
	}
	return b.Bytes()
}

func commitData(tree string) []byte {
	return []byte("tree " + tree + "\nauthor a <a@b> 0 +0000\ncommitter a <a@b> 0 +0000\n\nmsg\n")
}

func tagData(target string) []byte {
	return []byte("object " + target + "\ntype commit\ntag v1.0.0\ntagger a <a@b> 0 +0000\n\nmsg\n")
}

// fetchResponse wraps a pack in a v2 fetch response.
func fetchResponse(pack []byte, withShallowInfo bool) []byte {
	var b bytes.Buffer
	if withShallowInfo {
		b.Write(pkt("shallow-info", "shallow "+strings.Repeat("c", 40), "0001"))
	}
	b.Write(pkt("packfile"))
	for i := 0; i < len(pack); i += 1000 {
		b.Write(band(1, pack[i:min(i+1000, len(pack))]))
	}
	b.WriteString("0000")
	return b.Bytes()
}

func advertisement(fetchFeatures string) []byte {
	return pkt("# service=git-upload-pack", "0000", "version 2", "agent=git/2.0", "ls-refs=unborn",
		"fetch="+fetchFeatures, "server-option", "object-format=sha1", "0000")
}

// createRepositoryPack is CreateRepositoryPack of the C# tests.
func createRepositoryPack(includeBlobs bool) (pack []byte, tagID string, blobIDs map[string]string) {
	readmeID := objectID(gitpack.ObjBlob, sReadme)
	scriptID := objectID(gitpack.ObjBlob, sScript)
	packageFileID := objectID(gitpack.ObjBlob, sPackageFile)
	binTree := treeData(treeItem{"120000", "link", readmeID}, treeItem{"100755", "run.sh", scriptID})
	binTreeID := objectID(gitpack.ObjTree, binTree)
	rootTree := treeData(
		treeItem{"100644", "README.md", readmeID},
		treeItem{"40000", "bin", binTreeID},
		treeItem{"160000", "submodule", strings.Repeat("d", 40)},
		treeItem{"100644", "lip.yaml", packageFileID})
	commit := commitData(objectID(gitpack.ObjTree, rootTree))
	tag := tagData(objectID(gitpack.ObjCommit, commit))

	var b packBuilder
	b.add(gitpack.ObjTag, tag)
	b.add(gitpack.ObjCommit, commit)
	b.add(gitpack.ObjTree, rootTree)
	b.add(gitpack.ObjTree, binTree)
	if includeBlobs {
		b.add(gitpack.ObjBlob, sReadme)
		b.add(gitpack.ObjBlob, sScript)
		b.add(gitpack.ObjBlob, sPackageFile)
	}
	return b.build(), objectID(gitpack.ObjTag, tag), map[string]string{
		"README.md":  readmeID,
		"bin/run.sh": scriptID,
		"lip.yaml":   packageFileID,
	}
}

// --- fake hosts ---

type call struct {
	method, url string
	header      http.Header
	body        []byte
}

// hosts routes requests by host to handlers, like Flurl's HttpTest.
type hosts struct {
	mu       sync.Mutex
	handlers map[string]http.HandlerFunc
	calls    []call
}

func (h *hosts) RoundTrip(r *http.Request) (*http.Response, error) {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
	}
	h.mu.Lock()
	h.calls = append(h.calls, call{r.Method, r.URL.String(), r.Header.Clone(), body})
	handler := h.handlers[r.URL.Host]
	h.mu.Unlock()
	rec := httptest.NewRecorder()
	if handler == nil {
		rec.WriteHeader(http.StatusNotFound)
	} else {
		handler(rec, r)
	}
	resp := rec.Result()
	resp.Request = r
	return resp, nil
}

func (h *hosts) client(proxy string) *RemoteClient {
	return &RemoteClient{HTTP: &http.Client{Transport: h}, GithubProxy: proxy}
}

func (h *hosts) count(match func(c call) bool) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.calls {
		if match(c) {
			n++
		}
	}
	return n
}

func (h *hosts) posts() []call {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []call
	for _, c := range h.calls {
		if c.method == http.MethodPost {
			out = append(out, c)
		}
	}
	return out
}

func newHosts(handlers map[string]http.HandlerFunc) *hosts {
	return &hosts{handlers: handlers}
}

// gitHandler serves discovery with fetchFeatures and answers POSTs with
// respond.
func gitHandler(fetchFeatures string, respond func(body []byte) []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/info/refs"):
			w.Write(advertisement(fetchFeatures))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/git-upload-pack"):
			body, _ := io.ReadAll(r.Body)
			w.Write(respond(body))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func fixed(b []byte) func([]byte) []byte { return func([]byte) []byte { return b } }

func statusHandler(code int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
		io.WriteString(w, body)
	}
}

const allFeatures = "shallow wait-for-done filter"

// --- tests ---

func TestRemoteListRefsParsesRefsAndPeeledTags(t *testing.T) {
	a, b, c := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	h := newHosts(map[string]http.HandlerFunc{"example.com": gitHandler(allFeatures, fixed(pkt(
		a+" refs/tags/v1.0.0 peeled:"+b, c+" refs/tags/v1.1.0", c+" refs/heads/main", "0000")))})

	refs, err := h.client("").ListRefs(context.Background(), testRepoURL, []string{"refs/tags/v"})
	if err != nil {
		t.Fatal(err)
	}
	want := []RemoteRef{{"refs/tags/v1.0.0", a, b}, {"refs/tags/v1.1.0", c, ""}}
	if !reflect.DeepEqual(refs, want) {
		t.Fatalf("refs %+v", refs)
	}
	get := h.calls[0]
	if get.method != "GET" || get.url != testRepoURL+"/info/refs?service=git-upload-pack" ||
		get.header.Get("Git-Protocol") != "version=2" || get.header.Get("User-Agent") != "git/2.0 (lip)" {
		t.Fatalf("discovery %+v", get)
	}
	post := h.calls[1]
	body := string(post.body)
	for _, s := range []string{"command=ls-refs\n", "ref-prefix refs/tags/v\n", "peel\n"} {
		if !strings.Contains(body, s) {
			t.Errorf("body %q lacks %q", body, s)
		}
	}
	if post.url != testRepoURL+"/git-upload-pack" || post.header.Get("Content-Type") != "application/x-git-upload-pack-request" ||
		post.header.Get("Accept") != "application/x-git-upload-pack-result" || post.header.Get("Git-Protocol") != "version=2" {
		t.Fatalf("post %+v", post)
	}
}

func TestRemoteListRefsServerWithoutV2(t *testing.T) {
	h := newHosts(map[string]http.HandlerFunc{"example.com": func(w http.ResponseWriter, r *http.Request) {
		w.Write(pkt("# service=git-upload-pack", "0000", strings.Repeat("a", 40)+" HEAD\x00multi_ack side-band-64k", "0000"))
	}})
	_, err := h.client("").ListRefs(context.Background(), testRepoURL, nil)
	var pe *ProtocolError
	if !errors.As(err, &pe) || !strings.Contains(err.Error(), "version 2") {
		t.Fatalf("got %v", err)
	}
	want := "The git server example.com cannot be used to list the tags and branches of " + testRepoURL +
		": lip needs a git server that supports version 2 of the git protocol with shallow and partial fetches, as GitHub does."
	if err.Error() != want {
		t.Fatalf("got %q", err)
	}
	var us *UnsupportedServerError
	if !errors.As(err, &us) || us.Message != "The server at "+testRepoURL+" does not support version 2 of the git protocol." {
		t.Fatalf("inner %v", us)
	}
}

func TestRemoteListRefsHTTPErrorDoesNotRetry(t *testing.T) {
	for _, status := range []int{404, 429, 502} {
		h := newHosts(map[string]http.HandlerFunc{"example.com": statusHandler(status, "error")})
		_, err := h.client("").ListRefs(context.Background(), testRepoURL, nil)
		var re *RequestError
		if !errors.As(err, &re) || re.StatusCode != status {
			t.Fatalf("%d: got %v", status, err)
		}
		if n := len(h.calls); n != 1 {
			t.Fatalf("%d: %d calls", status, n)
		}
	}
}

func TestRemoteListRefsGithubUnauthorized(t *testing.T) {
	h := newHosts(map[string]http.HandlerFunc{"github.com": statusHandler(401, "Repository not found.")})
	_, err := h.client("").ListRefs(context.Background(), "https://github.com/owner/missing.git", nil)
	var re *RequestError
	if !errors.As(err, &re) || re.StatusCode != 401 || !strings.Contains(err.Error(), "does not exist or is private") {
		t.Fatalf("got %v", err)
	}
	want := "GitHub refused to let lip list the tags and branches of https://github.com/owner/missing.git (HTTP 401)." +
		" GitHub answers this way when a repository does not exist or is private: check the spelling of the package," +
		" and that its repository is public."
	if err.Error() != want {
		t.Fatalf("got %q", err)
	}
	se, ok := re.Err.(*StatusError)
	if !ok || se.Body != "Repository not found." ||
		se.Error() != "The server answered HTTP 401 (Unauthorized) to GET https://github.com/owner/missing.git/info/refs?service=git-upload-pack." {
		t.Fatalf("inner %#v", re.Err)
	}
}

func TestRemoteErrorBodyIsTruncated(t *testing.T) {
	long := strings.Repeat("é", 2500)
	h := newHosts(map[string]http.HandlerFunc{"example.com": statusHandler(500, long)})
	_, err := h.client("").ListRefs(context.Background(), testRepoURL, nil)
	var se *StatusError
	if !errors.As(err, &se) || se.Body != strings.Repeat("é", 2000)+"..." {
		t.Fatalf("got %v", err)
	}
	want := "The git server example.com had a problem when lip tried to list the tags and branches of " + testRepoURL + " (HTTP 500). Try again later."
	if err.Error() != want {
		t.Fatalf("got %q", err)
	}
	h = newHosts(map[string]http.HandlerFunc{"example.com": statusHandler(418, strings.Repeat("x", 2000))})
	_, err = h.client("").ListRefs(context.Background(), testRepoURL, nil)
	if !errors.As(err, &se) || se.Body != strings.Repeat("x", 2000) {
		t.Fatalf("got %v", err)
	}
	want = "The git server example.com answered HTTP 418 when lip tried to list the tags and branches of " + testRepoURL +
		". Try again later; if this keeps happening, check that the URL points to a git repository."
	if err.Error() != want {
		t.Fatalf("got %q", err)
	}
}

func TestRemoteDescribeStatus(t *testing.T) {
	for status, want := range map[int]string{
		403: "GitHub denied access when lip tried to P (HTTP 403). The repository may be private, or GitHub may be limiting how many requests lip can send; wait a while and try again.",
		404: "GitHub found no such repository when lip tried to P (HTTP 404). Check the spelling of the package, and that its repository is public.",
		429: "GitHub refused to let lip P because lip sent too many requests (HTTP 429). Wait a while and try again.",
		503: "GitHub had a problem when lip tried to P (HTTP 503). Try again later.",
		400: "GitHub answered HTTP 400 (BadRequest) when lip tried to P. Try again later; if this keeps happening, check that the URL points to a git repository.",
		401: "GitHub refused to let lip P (HTTP 401). The repository may not exist or may be private: check the spelling of the package, and that its repository is public.",
	} {
		if got := describeStatus("GitHub", "GitHub", "P", status, false); got != want {
			t.Errorf("%d: %q", status, got)
		}
	}
}

func TestRemoteGithubProxyRewritesURL(t *testing.T) {
	h := newHosts(map[string]http.HandlerFunc{"proxy.example.com": gitHandler(allFeatures, fixed(pkt("0000")))})
	if _, err := h.client("https://proxy.example.com/gh").ListRefs(context.Background(), "https://github.com/LiteLDev/LeviLamina.git", nil); err != nil {
		t.Fatal(err)
	}
	if h.calls[0].url != "https://proxy.example.com/gh/LiteLDev/LeviLamina.git/info/refs?service=git-upload-pack" ||
		h.calls[1].url != "https://proxy.example.com/gh/LiteLDev/LeviLamina.git/git-upload-pack" {
		t.Fatalf("calls %v %v", h.calls[0].url, h.calls[1].url)
	}
}

func TestRewriteGithub(t *testing.T) {
	for _, tc := range []struct{ proxy, in, want string }{
		{"https://p.example/gh", "https://github.com/A/B.git", "https://p.example/gh/A/B.git"},
		{"https://p.example/gh/", "https://GitHub.com/A/B.git", "https://p.example/gh/A/B.git"},
		{"https://p.example", "https://github.com/A/B/releases/download/v1/x.zip?a=1", "https://p.example/A/B/releases/download/v1/x.zip?a=1"},
		{"https://p.example/?k=v", "https://github.com/A", "https://p.example/A?k=v"},
		{"", "https://github.com/A/B.git", ""},
		{"https://p.example", "https://example.com/A/B.git", ""},
		{"https://github.com", "https://github.com/A/B.git", ""},
	} {
		got, ok := rewriteGithub(tc.proxy, tc.in)
		if got != tc.want || ok != (tc.want != "") {
			t.Errorf("rewriteGithub(%q, %q) = %q, %v", tc.proxy, tc.in, got, ok)
		}
	}
}

func TestRemoteGithubProxyFailsFallsBackForEachRequest(t *testing.T) {
	h := newHosts(map[string]http.HandlerFunc{
		"proxy.example.com": statusHandler(404, "not found"),
		"github.com": gitHandler(allFeatures, fixed(pkt(
			"0123456789abcdef0123456789abcdef01234567 refs/tags/v1.0.0", "0000"))),
	})
	c := h.client("https://proxy.example.com/gh")
	repo := "https://github.com/LiteLDev/LeviLamina.git"
	first, err := c.ListRefs(context.Background(), repo, []string{"refs/tags/v"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.ListRefs(context.Background(), repo, []string{"refs/tags/v"})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Name != "refs/tags/v1.0.0" || !reflect.DeepEqual(first, second) {
		t.Fatalf("%v %v", first, second)
	}
	// A request that fails through the proxy goes to GitHub directly; the
	// next one tries the proxy again (protocol/userspace.md §2).
	if n := h.count(func(c call) bool { return strings.HasPrefix(c.url, "https://proxy.example.com/") }); n != 2 {
		t.Fatalf("proxy calls %d", n)
	}
	if n := h.count(func(c call) bool { return c.url == repo+"/git-upload-pack" }); n != 2 {
		t.Fatalf("github posts %d", n)
	}
}

func TestRemoteGithubProxyAndGithubFail(t *testing.T) {
	h := newHosts(map[string]http.HandlerFunc{
		"proxy.example.com": statusHandler(404, "not found"),
		"github.com":        statusHandler(404, "not found"),
	})
	_, err := h.client("https://proxy.example.com/gh").ListRefs(context.Background(), "https://github.com/LiteLDev/LeviLamina.git", nil)
	var agg *AggregateError
	if !errors.As(err, &agg) || len(agg.Errs) != 2 ||
		!strings.Contains(err.Error(), "GitHub proxy proxy.example.com and then GitHub directly") {
		t.Fatalf("got %v", err)
	}
	for _, e := range agg.Errs {
		if re, ok := e.(*RequestError); !ok || re.StatusCode != 404 {
			t.Fatalf("inner %v", e)
		}
	}
	if !strings.HasSuffix(agg.Errs[0].Error(), " If the GitHub proxy does not support git repositories, remove it with 'lip config delete github_proxy'.") ||
		!strings.HasPrefix(agg.Errs[0].Error(), "The GitHub proxy proxy.example.com found no such repository") {
		t.Fatalf("proxy error %q", agg.Errs[0])
	}
}

func TestRemoteInvalidProxyFallsBack(t *testing.T) {
	h := newHosts(map[string]http.HandlerFunc{"github.com": gitHandler(allFeatures, fixed(pkt("0000")))})
	if _, err := h.client("not a url").ListRefs(context.Background(), "https://github.com/a/b.git", nil); err != nil {
		t.Fatal(err)
	}
	h = newHosts(nil)
	_, err := h.client("not a url").ListRefs(context.Background(), "https://github.com/a/b.git", nil)
	var agg *AggregateError
	var ip *InvalidProxyError
	if !errors.As(err, &agg) || !errors.As(agg.Errs[0], &ip) ||
		ip.Message != "The GitHub proxy set in github_proxy does not give a valid URL (not a url/a/b.git). Set a valid URL with"+
			" 'lip config set github_proxy <URL>', or remove it with 'lip config delete github_proxy'." {
		t.Fatalf("got %v", err)
	}
}

func TestRemoteFetchBlobsProxyIncompletePack(t *testing.T) {
	withoutBlobs, _, blobIDs := createRepositoryPack(false)
	withBlobs, _, _ := createRepositoryPack(true)
	h := newHosts(map[string]http.HandlerFunc{
		"proxy.example.com": gitHandler(allFeatures, fixed(fetchResponse(withoutBlobs, true))),
		"github.com":        gitHandler(allFeatures, fixed(fetchResponse(withBlobs, true))),
	})
	var received []string
	err := h.client("https://proxy.example.com/gh").FetchBlobs(context.Background(), "https://github.com/owner/repo.git",
		[]string{blobIDs["lip.yaml"], blobIDs["README.md"]}, func(oid string, _ []byte) error {
			received = append(received, oid)
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{blobIDs["README.md"], blobIDs["lip.yaml"]}
	sort.Strings(want)
	sort.Strings(received)
	if !reflect.DeepEqual(received, want) {
		t.Fatalf("received %v", received)
	}
	if n := h.count(func(c call) bool { return c.url == "https://github.com/owner/repo.git/git-upload-pack" }); n != 1 {
		t.Fatalf("direct posts %d", n)
	}
}

func TestRemoteRedirectedRepositoryPostsToTarget(t *testing.T) {
	inner := gitHandler(allFeatures, fixed(pkt("0000")))
	h := newHosts(map[string]http.HandlerFunc{"github.com": func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/Old/Name.git/info/refs") {
			w.Header().Set("Location", "https://github.com/New/Name.git/info/refs?service=git-upload-pack")
			w.WriteHeader(301)
			return
		}
		inner(w, r)
	}})
	if _, err := h.client("").ListRefs(context.Background(), "https://github.com/Old/Name.git", nil); err != nil {
		t.Fatal(err)
	}
	// Without Git-Protocol the redirect target would answer with protocol
	// version 0.
	target := h.calls[1]
	if target.url != "https://github.com/New/Name.git/info/refs?service=git-upload-pack" ||
		target.header.Get("Git-Protocol") != "version=2" || target.header.Get("User-Agent") != "git/2.0 (lip)" {
		t.Fatalf("redirected discovery %+v", target)
	}
	if posts := h.posts(); len(posts) != 1 || posts[0].url != "https://github.com/New/Name.git/git-upload-pack" {
		t.Fatalf("posts %+v", posts)
	}
}

func TestRemoteListTreeSameCommitTwiceAcrossGithubCaseVariants(t *testing.T) {
	pack, tagID, _ := createRepositoryPack(false)
	h := newHosts(map[string]http.HandlerFunc{"github.com": gitHandler(allFeatures, fixed(fetchResponse(pack, true)))})
	c := h.client("")
	first, err := c.ListTree(context.Background(), "https://github.com/Owner/Repo.git", tagID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.ListTree(context.Background(), "https://github.com/owner/repo", tagID)
	if err != nil {
		t.Fatal(err)
	}
	if &first[0] != &second[0] || len(h.posts()) != 1 {
		t.Fatalf("not reused: %d posts", len(h.posts()))
	}
}

func TestRemoteListTreeSharedListingCanceledByAnotherCaller(t *testing.T) {
	pack, tagID, _ := createRepositoryPack(false)
	posted := make(chan struct{}, 2)
	var mu sync.Mutex
	first := true
	inner := gitHandler(allFeatures, fixed(fetchResponse(pack, true)))
	h := newHosts(map[string]http.HandlerFunc{"example.com": func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			mu.Lock()
			block := first
			first = false
			mu.Unlock()
			posted <- struct{}{}
			if block {
				<-r.Context().Done()
				w.WriteHeader(500)
				return
			}
		}
		inner(w, r)
	}})
	c := h.client("")
	ctx1, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		_, err := c.ListTree(ctx1, testRepoURL, tagID)
		errc <- err
	}()
	<-posted
	res := make(chan error, 1)
	go func() {
		entries, err := c.ListTree(context.Background(), testRepoURL, tagID)
		if err == nil && len(entries) != 4 {
			err = fmt.Errorf("%d entries", len(entries))
		}
		res <- err
	}()
	cancel()
	if err := <-errc; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled caller: %v", err)
	}
	if err := <-res; err != nil {
		t.Fatalf("other caller: %v", err)
	}
}

func TestRemoteListTreeThroughAnnotatedTag(t *testing.T) {
	pack, tagID, blobIDs := createRepositoryPack(false)
	h := newHosts(map[string]http.HandlerFunc{"example.com": gitHandler(allFeatures, fixed(fetchResponse(pack, true)))})
	entries, err := h.client("").ListTree(context.Background(), testRepoURL, strings.ToUpper(tagID))
	if err != nil {
		t.Fatal(err)
	}
	want := []TreeEntry{
		{"README.md", 100644, blobIDs["README.md"]},
		{"bin/link", 120000, blobIDs["README.md"]},
		{"bin/run.sh", 100755, blobIDs["bin/run.sh"]},
		{"lip.yaml", 100644, blobIDs["lip.yaml"]},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries %+v", entries)
	}
	body := string(h.posts()[0].body)
	for _, s := range []string{"want " + tagID + "\n", "deepen 1\n", "filter blob:none\n", "no-progress\n", "ofs-delta\n", "done\n"} {
		if !strings.Contains(body, s) {
			t.Errorf("body %q lacks %q", body, s)
		}
	}
}

func singleTreePack(name string, blobID string) (pack []byte, commitID string, treeID string) {
	tree := treeData(treeItem{"100644", name, blobID})
	commit := commitData(objectID(gitpack.ObjTree, tree))
	var b packBuilder
	b.add(gitpack.ObjCommit, commit)
	b.add(gitpack.ObjTree, tree)
	return b.build(), objectID(gitpack.ObjCommit, commit), objectID(gitpack.ObjTree, tree)
}

func TestRemoteListTreeUnsafeEntryName(t *testing.T) {
	for _, name := range []string{"..", ".", "", "a/b"} {
		pack, commit, tree := singleTreePack(name, strings.Repeat("a", 40))
		h := newHosts(map[string]http.HandlerFunc{"example.com": gitHandler(allFeatures, fixed(fetchResponse(pack, true)))})
		_, err := h.client("").ListTree(context.Background(), testRepoURL, commit)
		var pe *ProtocolError
		if !errors.As(err, &pe) {
			t.Fatalf("%q: %v", name, err)
		}
		want := "The git repository " + testRepoURL + " contains data lip cannot handle: it has an entry named '" + name +
			"' (in tree " + tree + "), which is not a valid file name. Pick another version of the package, or report the problem to the package author."
		if err.Error() != want {
			t.Fatalf("%q: %q", name, err)
		}
	}
}

func TestRemoteListTreeNameGitDoesNotCheckOutIsListed(t *testing.T) {
	// protocol/package.md §1 puts no restriction on the names in a git
	// archive.
	for _, name := range []string{".git", `a\b`} {
		blob := strings.Repeat("a", 40)
		pack, commit, _ := singleTreePack(name, blob)
		h := newHosts(map[string]http.HandlerFunc{"example.com": gitHandler(allFeatures, fixed(fetchResponse(pack, true)))})
		entries, err := h.client("").ListTree(context.Background(), testRepoURL, commit)
		if err != nil || !reflect.DeepEqual(entries, []TreeEntry{{name, 100644, blob}}) {
			t.Fatalf("%q: %v %v", name, entries, err)
		}
	}
}

func TestTreeEntryIsRegularFile(t *testing.T) {
	for mode, want := range map[int]bool{100644: true, 100755: true, 100664: false, 120000: false} {
		if (TreeEntry{"a", mode, strings.Repeat("a", 40)}).IsRegularFile() != want {
			t.Errorf("%d", mode)
		}
	}
}

func TestRemoteListTreeErrorBandNotOurRef(t *testing.T) {
	resp := append(pkt("packfile"), band(3, []byte("upload-pack: not our ref"))...)
	resp = append(resp, "0000"...)
	h := newHosts(map[string]http.HandlerFunc{"example.com": gitHandler(allFeatures, fixed(resp))})
	id := strings.Repeat("a", 40)
	_, err := h.client("").ListTree(context.Background(), testRepoURL, id)
	var pe *ProtocolError
	if !errors.As(err, &pe) || !strings.Contains(err.Error(), "has no commit "+id) ||
		!strings.Contains(pe.Err.Error(), "not our ref") {
		t.Fatalf("got %v", err)
	}
	want := "The git repository " + testRepoURL + " has no commit " + id + ". The commit may have been removed from the" +
		" repository (for example by a force push), or the commit id is wrong; check it, or pick another version of the package."
	if err.Error() != want {
		t.Fatalf("got %q", err)
	}
}

func TestRemoteListTreeErrLine(t *testing.T) {
	h := newHosts(map[string]http.HandlerFunc{"example.com": gitHandler(allFeatures, fixed(pkt("ERR something broke", "0000")))})
	_, err := h.client("").ListTree(context.Background(), testRepoURL, strings.Repeat("a", 40))
	var pe *ProtocolError
	if !errors.As(err, &pe) || !strings.Contains(err.Error(), "example.com reported an error") ||
		!strings.Contains(pe.Err.Error(), "something broke") {
		t.Fatalf("got %v", err)
	}
}

func TestRemoteListTreeNotAnObjectID(t *testing.T) {
	_, err := (&RemoteClient{}).ListTree(context.Background(), testRepoURL, "main")
	var ae *ArgumentError
	if !errors.As(err, &ae) || err.Error() != "'main' is not a git object id; an object id is 40 hexadecimal digits." {
		t.Fatalf("got %v", err)
	}
}

func TestRemoteListTreeWithoutShallowPartialFetch(t *testing.T) {
	for _, features := range []string{"shallow", "filter", ""} {
		h := newHosts(map[string]http.HandlerFunc{"example.com": gitHandler(features, fixed(nil))})
		_, err := h.client("").ListTree(context.Background(), testRepoURL, strings.Repeat("a", 40))
		var pe *ProtocolError
		if !errors.As(err, &pe) {
			t.Fatalf("%q: %v", features, err)
		}
		// The repository is never cloned in full.
		if len(h.posts()) != 0 {
			t.Fatalf("%q: fetched", features)
		}
	}
}

func TestRemoteListTreeIncompletePackFromProxyRetriedDirectly(t *testing.T) {
	pack, tagID, _ := createRepositoryPack(false)
	var b packBuilder // the tag only
	b.add(gitpack.ObjTag, tagData(strings.Repeat("e", 40)))
	h := newHosts(map[string]http.HandlerFunc{
		"proxy.example.com": gitHandler(allFeatures, fixed(fetchResponse(b.build(), true))),
		"github.com":        gitHandler(allFeatures, fixed(fetchResponse(pack, true))),
	})
	entries, err := h.client("https://proxy.example.com").ListTree(context.Background(), "https://github.com/o/r.git", tagID)
	if err != nil || len(entries) != 4 {
		t.Fatalf("%v %v", entries, err)
	}
}

func TestRemoteUnsupportedObjectFormat(t *testing.T) {
	h := newHosts(map[string]http.HandlerFunc{"example.com": func(w http.ResponseWriter, r *http.Request) {
		w.Write(pkt("version 2", "ls-refs", "fetch=shallow filter", "object-format=sha256", "0000"))
	}})
	_, err := h.client("").ListRefs(context.Background(), testRepoURL, nil)
	want := "The git repository " + testRepoURL + " uses sha256 object ids, which lip does not support yet." +
		" Pick another version of the package, or ask the package author to publish it from a repository that uses SHA-1 object ids."
	if err == nil || err.Error() != want {
		t.Fatalf("got %v", err)
	}
}

func TestRemoteFetchBlobsReturnsEachBlobOnce(t *testing.T) {
	var b packBuilder
	base := b.add(gitpack.ObjBlob, sReadme)
	modified := []byte("# readme\nmore\n")
	b.addOfsDelta(base, copyInsertDelta(len(sReadme), len(modified), len(sReadme), []byte("more\n")))
	readmeID := objectID(gitpack.ObjBlob, sReadme)
	modifiedID := objectID(gitpack.ObjBlob, modified)
	h := newHosts(map[string]http.HandlerFunc{"example.com": gitHandler(allFeatures, fixed(fetchResponse(b.build(), false)))})

	blobs := map[string][]byte{}
	err := h.client("").FetchBlobs(context.Background(), testRepoURL, []string{readmeID, modifiedID, strings.ToUpper(readmeID)},
		func(oid string, data []byte) error {
			if _, dup := blobs[oid]; dup {
				t.Errorf("%s twice", oid)
			}
			blobs[oid] = bytes.Clone(data)
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(blobs[readmeID], sReadme) || !bytes.Equal(blobs[modifiedID], modified) {
		t.Fatalf("blobs %q", blobs)
	}
	body := string(h.posts()[0].body)
	if !strings.Contains(body, "want "+readmeID+"\n") || !strings.Contains(body, "want "+modifiedID+"\n") ||
		strings.Contains(body, "deepen") || strings.Contains(body, "filter") || strings.Count(body, "want ") != 2 {
		t.Fatalf("body %q", body)
	}
}

func TestRemoteFetchBlobsManyIDsBatchesRequests(t *testing.T) {
	var empty packBuilder
	h := newHosts(map[string]http.HandlerFunc{"example.com": gitHandler(allFeatures, fixed(fetchResponse(empty.build(), false)))})
	ids := make([]string, 300)
	for i := range ids {
		ids[i] = fmt.Sprintf("%040x", i)
	}
	err := h.client("").FetchBlobs(context.Background(), testRepoURL, ids, func(string, []byte) error { return nil })
	var pe *ProtocolError
	if !errors.As(err, &pe) {
		t.Fatalf("got %v", err)
	}
	want := "Could not download 300 of the 300 requested files from " + testRepoURL + ": 2 of the download requests failed."
	if err.Error() != want {
		t.Fatalf("got %q", err)
	}
	if len(h.calls) != 3 {
		t.Fatalf("%d calls", len(h.calls))
	}
	posts := h.posts()
	if strings.Count(string(posts[0].body), "want ") != 256 || strings.Count(string(posts[1].body), "want ") != 44 {
		t.Fatal("batches")
	}
	var agg *AggregateError
	if !errors.As(pe.Err, &agg) || len(agg.Errs) != 2 ||
		agg.Errs[1].Error() != "The git server example.com sent an unexpected response when lip tried to download 44 files from "+
			testRepoURL+". Try again later; if this keeps happening, it may not be a git server." {
		t.Fatalf("inner %v", pe.Err)
	}
}

func TestRemoteFetchBlobsEmptyMakesNoRequests(t *testing.T) {
	h := newHosts(nil)
	if err := h.client("").FetchBlobs(context.Background(), testRepoURL, nil, nil); err != nil || len(h.calls) != 0 {
		t.Fatalf("%v %d", err, len(h.calls))
	}
}

func TestRemoteFetchBlobsCallbackErrorReturnedAsIs(t *testing.T) {
	var b packBuilder
	b.add(gitpack.ObjBlob, sReadme)
	h := newHosts(map[string]http.HandlerFunc{"example.com": gitHandler(allFeatures, fixed(fetchResponse(b.build(), false)))})
	boom := errors.New("boom")
	err := h.client("").FetchBlobs(context.Background(), testRepoURL, []string{objectID(gitpack.ObjBlob, sReadme)},
		func(string, []byte) error { return boom })
	if err != boom {
		t.Fatalf("got %v", err)
	}
}

func TestRemoteFetchBlobsNotABlob(t *testing.T) {
	var b packBuilder
	tree := treeData()
	b.add(gitpack.ObjTree, tree)
	treeID := objectID(gitpack.ObjTree, tree)
	h := newHosts(map[string]http.HandlerFunc{"example.com": gitHandler(allFeatures, fixed(fetchResponse(b.build(), false)))})
	err := h.client("").FetchBlobs(context.Background(), testRepoURL, []string{treeID}, func(string, []byte) error { return nil })
	want := "The git repository " + testRepoURL + " contains data lip cannot handle: the object " + treeID +
		" was requested as a file (blob) but is a tree. Pick another version of the package, or report the problem to the package author."
	if err == nil || err.Error() != want {
		t.Fatalf("got %v", err)
	}
}

func TestRemoteExplainNetworkFailures(t *testing.T) {
	const purpose = "do it"
	for _, tc := range []struct {
		err  error
		want string
	}{
		{&timeoutError{url: "x"}, "The git server example.com did not answer in time when lip tried to do it. Check your network connection and try again."},
		{&transportError{err: io.ErrUnexpectedEOF}, "Could not connect to the git server example.com to do it. Check your network connection and try again."},
		{&transportError{err: io.ErrUnexpectedEOF, body: true}, "The connection to the git server example.com broke off while lip tried to do it. Check your network connection and try again."},
		{protocolErrorf("bad"), "The git server example.com sent an unexpected response when lip tried to do it. Try again later; if this keeps happening, it may not be a git server."},
	} {
		got := explain(tc.err, testRepoURL, testRepoURL, purpose, "")
		if got.Error() != tc.want || !errors.Is(got, tc.err) {
			t.Errorf("%v: %q", tc.err, got)
		}
	}
	plain := errors.New("disk full")
	if got := explain(plain, testRepoURL, testRepoURL, purpose, ""); got != plain {
		t.Errorf("plain: %v", got)
	}
}

func TestRemoteProgressAndTempFiles(t *testing.T) {
	dir := t.TempDir()
	var b packBuilder
	b.add(gitpack.ObjBlob, sReadme)
	h := newHosts(map[string]http.HandlerFunc{"example.com": gitHandler(allFeatures, fixed(fetchResponse(b.build(), false)))})
	c := h.client("")
	c.TempDir = dir
	var got []PackProgress
	ctx := WithPackProgress(context.Background(), func(p PackProgress) { got = append(got, p) })
	err := c.FetchBlobs(ctx, testRepoURL, []string{objectID(gitpack.ObjBlob, sReadme)}, func(string, []byte) error {
		entries, _ := os.ReadDir(dir)
		if len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), ".tmp") {
			t.Errorf("pack file while reading: %v", entries)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	by := checkAttempts(t, got)
	last := by[1][len(by[1])-1]
	if len(by) != 1 || !last.Done || last.Err != nil || last.Objects != 1 || last.Total != 1 || last.URL != testRepoURL {
		t.Fatalf("progress %+v", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("pack file left: %v", entries)
	}
}

func TestRemoteContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := newHosts(map[string]http.HandlerFunc{"example.com": gitHandler(allFeatures, fixed(pkt("0000")))})
	h2 := &RemoteClient{HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.Context().Err(); err != nil {
			return nil, err
		}
		return h.RoundTrip(r)
	})}, GithubProxy: "https://proxy.example.com"}
	if _, err := h2.ListRefs(ctx, "https://github.com/a/b.git", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRemoteListVersionTagsAndResolveRef(t *testing.T) {
	a, b, c := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	h := newHosts(map[string]http.HandlerFunc{"example.com": gitHandler(allFeatures, func(body []byte) []byte {
		all := []string{a + " refs/tags/v1.0.0 peeled:" + b, c + " refs/tags/v1.1.0-rc.1", c + " refs/tags/vnext",
			c + " refs/heads/main", a + " refs/heads/main-old", a + " refs/tags/tag peeled:" + strings.ToUpper(b)}
		var out []string
		for _, r := range all {
			for _, line := range strings.Split(string(body), "\n") {
				if p, ok := strings.CutPrefix(line[min(4, len(line)):], "ref-prefix "); ok && strings.HasPrefix(strings.Fields(r)[1], p) {
					out = append(out, r)
					break
				}
			}
		}
		return pkt(append(out, "0000")...)
	})})
	cl := h.client("")
	ctx := context.Background()
	tags, err := cl.ListVersionTags(ctx, testRepoURL)
	want := []TagRef{{"v1.0.0", "1.0.0", b}, {"v1.1.0-rc.1", "1.1.0-rc.1", c}, {"vnext", "next", c}}
	if err != nil || !reflect.DeepEqual(tags, want) {
		t.Fatalf("%+v %v", tags, err)
	}
	for _, tc := range []struct {
		kind RefKind
		name string
		want string
	}{
		{RefVersion, "1.0.0", b}, {RefBranch, "main", c}, {RefTag, "tag", b}, {RefRev, strings.ToUpper(a), a},
	} {
		got, err := cl.ResolveRef(ctx, testRepoURL, tc.kind, tc.name)
		if err != nil || got != tc.want {
			t.Errorf("%v %s: %s %v", tc.kind, tc.name, got, err)
		}
	}
	var nf *RefNotFoundError
	if _, err := cl.ResolveRef(ctx, testRepoURL, RefBranch, "mai"); !errors.As(err, &nf) || nf.RefName != "refs/heads/mai" {
		t.Fatalf("got %v", err)
	}
	var ae *ArgumentError
	if _, err := cl.ResolveRef(ctx, testRepoURL, RefRev, "abc"); !errors.As(err, &ae) {
		t.Fatalf("got %v", err)
	}
	if slices.ContainsFunc(h.posts(), func(c call) bool { return strings.Contains(string(c.body), "abc") }) {
		t.Fatal("rev looked up")
	}
}

func TestRemoteOpenCommit(t *testing.T) {
	withBlobs, tagID, _ := createRepositoryPack(true)
	h := newHosts(map[string]http.HandlerFunc{"example.com": gitHandler(allFeatures, fixed(fetchResponse(withBlobs, true)))})
	src, err := h.client("").OpenCommit(context.Background(), testRepoURL, tagID)
	if err != nil {
		t.Fatal(err)
	}
	if keys := src.Keys(); !reflect.DeepEqual(keys, []string{"README.md", "bin/run.sh", "lip.yaml"}) {
		t.Fatalf("keys %v", keys)
	}
	rc, err := src.Open("lip.yaml")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(data, sPackageFile) {
		t.Fatalf("content %q", data)
	}
	if _, ok := src.Size("lip.yaml"); ok {
		t.Fatal("size known before prefetch")
	}
	if err := src.Prefetch(context.Background(), []string{"README.md", "bin/run.sh"}); err != nil {
		t.Fatal(err)
	}
	if n, ok := src.Size("bin/run.sh"); !ok || n != int64(len(sScript)) {
		t.Fatalf("size %d %v", n, ok)
	}
	if _, err := src.Open("bin/link"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("symlink: %v", err)
	}
	if _, err := h.client("").OpenCommit(context.Background(), testRepoURL, "main"); err == nil {
		t.Fatal("non-id accepted")
	}
}
