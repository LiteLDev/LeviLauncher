package gitpack

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const emptyBlob = "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"

func TestHashObject(t *testing.T) {
	if got := HashObject(ObjBlob, nil); got != emptyBlob {
		t.Fatalf("empty blob = %s", got)
	}
	// echo 'test content' | git hash-object --stdin
	if got := HashObject(ObjBlob, []byte("test content\n")); got != "d670460b4b4aece5915caf5c68d12f560a9fe3e4" {
		t.Fatalf("got %s", got)
	}
	if got := HashObject(ObjTree, nil); got != "4b825dc642cb6eb9a060e54bf8d69288fbee4904" {
		t.Fatalf("empty tree = %s", got)
	}
}

func TestVerifyBlob(t *testing.T) {
	if err := VerifyBlob(emptyBlob, nil); err != nil {
		t.Fatal(err)
	}
	if err := VerifyBlob(strings.ToUpper(emptyBlob), nil); err != nil {
		t.Fatal(err)
	}
	if err := VerifyBlob(emptyBlob, []byte("x")); err == nil {
		t.Fatal("expected mismatch")
	}
	// SHA-256 empty blob.
	if err := VerifyBlob("473a0f4c3be8a93681a267e3b1e9a7dcda1185436fe141f7749120a303721813", nil); err != nil {
		t.Fatal(err)
	}
}

func rawTree(entries ...TreeEntry) []byte {
	var b bytes.Buffer
	for _, e := range entries {
		id, _ := hex.DecodeString(e.OID)
		b.WriteString(e.Mode + " " + e.Name + "\x00")
		b.Write(id)
	}
	return b.Bytes()
}

func TestParseTree(t *testing.T) {
	in := []TreeEntry{
		{"100644", "a.txt", emptyBlob},
		{"040000", "dir", "4b825dc642cb6eb9a060e54bf8d69288fbee4904"},
	}
	got, err := ParseTree(rawTree(in...))
	if err != nil {
		t.Fatal(err)
	}
	want := []TreeEntry{in[0], {"40000", "dir", in[1].OID}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if got, err := ParseTree(nil); err != nil || len(got) != 0 {
		t.Fatalf("empty tree: %v %v", got, err)
	}
	for _, bad := range [][]byte{
		[]byte("100644 a"),
		[]byte("100644 a\x00short"),
		rawTree(TreeEntry{"100648", "a", emptyBlob}),
		rawTree(TreeEntry{"", "a", emptyBlob}),
		rawTree(TreeEntry{"10064400", "a", emptyBlob}),
	} {
		if _, err := ParseTree(bad); err == nil {
			t.Errorf("ParseTree(%q) succeeded", bad)
		}
	}
	got, _ = ParseTree(rawTree(TreeEntry{"100644", "\xffx", emptyBlob}))
	if got[0].Name != "�x" {
		t.Errorf("invalid UTF-8 name = %q", got[0].Name)
	}
}

func TestWalkNames(t *testing.T) {
	for _, name := range []string{"", ".", "..", "a/b", "a\\b"} {
		s := NewStore()
		blob := s.Put(ObjBlob, []byte("x"))
		root := s.Put(ObjTree, rawTree(TreeEntry{"100644", name, blob}))
		var unusable *UnusableRepositoryError
		if _, err := s.WalkFiles(root); !errors.As(err, &unusable) {
			t.Errorf("name %q: %v", name, err)
		} else if want := "it has an entry named '" + name + "' (in tree " + root + "), which is not a valid file name"; err.Error() != want {
			t.Errorf("name %q: %q, want %q", name, err, want)
		}
	}
	// protocol/package.md section 1 puts no other restriction on names.
	for _, name := range []string{".git", ".GIT"} {
		s := NewStore()
		blob := s.Put(ObjBlob, []byte("x"))
		root := s.Put(ObjTree, rawTree(TreeEntry{"100644", name, blob}))
		if files, err := s.WalkFiles(root); err != nil || len(files) != 1 || files[0].Path != name {
			t.Errorf("name %q: %+v, %v", name, files, err)
		}
	}
	// Only 100644 and 100755 are regular files.
	for mode, want := range map[string]bool{"100644": true, "100755": true, "100664": false, "120000": false} {
		if IsRegularFile(mode) != want {
			t.Errorf("IsRegularFile(%s) = %v", mode, !want)
		}
	}
	s := NewStore()
	blob := s.Put(ObjBlob, []byte("x"))
	missing := strings.Repeat("ab", 20)
	root := s.Put(ObjTree, rawTree(TreeEntry{"40000", "sub", missing}))
	if _, err := s.WalkFiles(root); err == nil {
		t.Error("missing subtree accepted")
	}
	root = s.Put(ObjTree, rawTree(TreeEntry{"40000", "sub", blob}))
	if _, err := s.WalkFiles(root); err == nil {
		t.Error("blob as subtree accepted")
	}
	// Gitlinks are skipped without needing the object.
	root = s.Put(ObjTree, rawTree(
		TreeEntry{"100644", ".gitmodules", blob},
		TreeEntry{"120000", "link", blob},
		TreeEntry{"160000", "mod", missing},
	))
	all, err := s.WalkTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[1].Path != "link" || all[1].IsRegularFile() {
		t.Fatalf("WalkTree = %+v", all)
	}
	files, err := s.WalkFiles(root)
	if err != nil || len(files) != 1 || files[0].Path != ".gitmodules" {
		t.Fatalf("WalkFiles = %+v, %v", files, err)
	}
}

func TestCommitTreeAndPeel(t *testing.T) {
	s := NewStore()
	tree := s.Put(ObjTree, nil)
	commit := s.Put(ObjCommit, []byte("tree "+tree+"\r\nauthor a <a> 0 +0000\n\nmsg\ntree "+emptyBlob+"\n"))
	got, err := s.CommitTree(commit)
	if err != nil || got != tree {
		t.Fatalf("CommitTree = %s, %v", got, err)
	}
	tag := s.Put(ObjTag, []byte("object "+commit+"\ntype commit\ntag v1\n\nmsg\n"))
	tag2 := s.Put(ObjTag, []byte("object "+tag+"\ntype tag\ntag v1b\n\nmsg\n"))
	if c, err := s.PeelCommit(tag2); err != nil || c != commit {
		t.Fatalf("PeelCommit = %s, %v", c, err)
	}
	if files, err := s.ListTree(tag2); err != nil || len(files) != 0 {
		t.Fatalf("ListTree = %v, %v", files, err)
	}
	bad := s.Put(ObjCommit, []byte("author a\n\ntree "+tree+"\n"))
	if _, err := s.CommitTree(bad); err == nil {
		t.Error("tree after header block accepted")
	}
	if _, err := s.CommitTree(tree); err == nil {
		t.Error("tree accepted as commit")
	}
	if _, err := s.PeelCommit(tree); err == nil {
		t.Error("tree peeled to commit")
	}
}

func TestApplyDelta(t *testing.T) {
	base := []byte("hello, world")
	// src 12, dst 11: copy off 7 len 5 ("world"), insert ", hi!" (5)... build manually.
	delta := []byte{12, 10, 0x91, 7, 5, 5, ' ', 'h', 'i', '!', '!'}
	got, err := ApplyDelta(base, delta)
	if err != nil || string(got) != "world hi!!" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, bad := range [][]byte{
		{11, 1, 1, 'x'},         // base size mismatch
		{12, 2, 1, 'x'},         // result too short
		{12, 1, 0},              // opcode 0
		{12, 5, 0x91, 10, 5},    // copy out of range
		{12, 5, 5, 'a'},         // insert truncated
		{12, 5, 0x91, 7},        // truncated copy args
		{12, 1, 1, 'x', 1, 'y'}, // output overflow
		{12},                    // missing result size
	} {
		if _, err := ApplyDelta(base, bad); !errors.Is(err, ErrCorrupt) {
			t.Errorf("ApplyDelta(%v) = %v", bad, err)
		}
	}
	// Copy size 0 means 0x10000.
	big := bytes.Repeat([]byte{'a'}, 0x10000)
	d := append(appendVarint(appendVarint(nil, 0x10000), 0x10000), 0x80)
	if got, err := ApplyDelta(big, d); err != nil || !bytes.Equal(got, big) {
		t.Fatalf("0x10000 copy: %v", err)
	}
}

func appendVarint(b []byte, n int) []byte {
	for n >= 0x80 {
		b = append(b, byte(n)|0x80)
		n >>= 7
	}
	return append(b, byte(n))
}

// packWriter builds synthetic packfiles.
type packWriter struct {
	buf     bytes.Buffer
	count   int
	version uint32
}

func (w *packWriter) entryHeader(typ int, size int) int {
	if w.buf.Len() == 0 {
		w.buf.WriteString("PACK")
		w.buf.Write(make([]byte, 8))
	}
	off := w.buf.Len()
	c := byte(typ<<4) | byte(size&0x0f)
	size >>= 4
	for size > 0 {
		w.buf.WriteByte(c | 0x80)
		c = byte(size & 0x7f)
		size >>= 7
	}
	w.buf.WriteByte(c)
	w.count++
	return off
}

func (w *packWriter) zdata(data []byte) {
	zw := zlib.NewWriter(&w.buf)
	zw.Write(data)
	zw.Close()
}

func (w *packWriter) object(t ObjectType, data []byte) int {
	off := w.entryHeader(int(t), len(data))
	w.zdata(data)
	return off
}

func (w *packWriter) refDelta(base string, delta []byte) {
	w.entryHeader(objRefDelta, len(delta))
	id, _ := hex.DecodeString(base)
	w.buf.Write(id)
	w.zdata(delta)
}

func (w *packWriter) ofsDelta(baseOff int, delta []byte) {
	off := w.entryHeader(objOfsDelta, len(delta))
	d := off - baseOff
	var enc []byte
	enc = append(enc, byte(d&0x7f))
	for d >>= 7; d > 0; d >>= 7 {
		d--
		enc = append([]byte{0x80 | byte(d&0x7f)}, enc...)
	}
	w.buf.Write(enc)
	w.zdata(delta)
}

func (w *packWriter) bytes() []byte {
	b := append([]byte(nil), w.buf.Bytes()...)
	v := w.version
	if v == 0 {
		v = 2
	}
	binary.BigEndian.PutUint32(b[4:], v)
	binary.BigEndian.PutUint32(b[8:], uint32(w.count))
	sum := sha1.Sum(b)
	return append(b, sum[:]...)
}

func reseal(p []byte) []byte {
	p = append([]byte(nil), p[:len(p)-20]...)
	sum := sha1.Sum(p)
	return append(p, sum[:]...)
}

// insertDelta returns a delta producing out from a base of size n.
func insertDelta(n int, out string) []byte {
	d := appendVarint(appendVarint(nil, n), len(out))
	return append(append(d, byte(len(out))), out...)
}

func TestDecodeSynthetic(t *testing.T) {
	base := []byte("base content")
	baseID := HashObject(ObjBlob, base)
	w := &packWriter{version: 3}
	// REF_DELTA whose base comes later in the pack, then an OFS_DELTA chain
	// on top of it.
	w.refDelta(baseID, insertDelta(len(base), "one"))
	w.object(ObjBlob, base)
	off := w.object(ObjBlob, []byte("x"))
	w.ofsDelta(off, insertDelta(1, "two"))
	pack := w.bytes()

	s := NewStore()
	if err := s.Decode(pack); err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"base content", "one", "x", "two"} {
		o, ok := s.Get(HashObject(ObjBlob, []byte(c)))
		if !ok || string(o.Data) != c || o.Type != ObjBlob {
			t.Errorf("missing %q", c)
		}
	}
	if s.Len() != 4 {
		t.Errorf("Len = %d", s.Len())
	}

	// REF_DELTA against an object already in the store (thin pack).
	w = &packWriter{}
	w.refDelta(baseID, insertDelta(len(base), "three"))
	thin := w.bytes()
	if err := NewStore().Decode(thin); !errors.Is(err, ErrCorrupt) {
		t.Errorf("thin pack without base: %v", err)
	}
	if err := s.Decode(thin); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get(HashObject(ObjBlob, []byte("three"))); !ok {
		t.Error("thin delta not resolved")
	}

	bad := func(name string, p []byte) {
		t.Helper()
		if err := NewStore().Decode(p); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: %v", name, err)
		}
	}
	bad("short", pack[:31])
	p := append([]byte(nil), pack...)
	p[0] = 'X'
	bad("signature", reseal(p))
	p = append([]byte(nil), pack...)
	p[7] = 4
	bad("version", reseal(p))
	p = append([]byte(nil), pack...)
	p[len(p)-1] ^= 1
	bad("checksum", p)
	p = append([]byte(nil), pack...)
	p[11]++
	bad("count too high", reseal(p))
	p = append([]byte(nil), pack...)
	p[11]--
	bad("trailing data", reseal(p))

	w = &packWriter{}
	w.entryHeader(int(ObjBlob), 5)
	w.zdata([]byte("four"))
	bad("size mismatch short", w.bytes())
	w = &packWriter{}
	w.entryHeader(int(ObjBlob), 3)
	w.zdata([]byte("four"))
	bad("size mismatch long", w.bytes())
	large := bytes.Repeat([]byte("0123456789abcdef"), 1<<17) // 2 MiB
	w = &packWriter{}
	w.object(ObjBlob, large)
	ls := NewStore()
	if err := ls.Decode(w.bytes()); err != nil {
		t.Fatal(err)
	}
	if o, ok := ls.Get(HashObject(ObjBlob, large)); !ok || !bytes.Equal(o.Data, large) {
		t.Error("large blob not decoded")
	}
	w = &packWriter{}
	w.entryHeader(int(ObjBlob), len(large)+1)
	w.zdata(large)
	bad("large size mismatch", w.bytes())

	w = &packWriter{}
	w.entryHeader(5, 1)
	w.zdata([]byte("x"))
	bad("type 5", w.bytes())
	w = &packWriter{}
	w.object(ObjBlob, []byte("abc"))
	w.ofsDelta(1, insertDelta(3, "y"))
	bad("ofs not at entry", w.bytes())
	w = &packWriter{}
	w.object(ObjBlob, []byte("abc"))
	w.refDelta(strings.Repeat("00", 20), insertDelta(3, "y"))
	bad("missing ref base", w.bytes())
}

// git runs git in dir and returns stdout.
func git(t *testing.T, dir string, stdin []byte, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("git", append([]string{
		"-c", "user.name=t", "-c", "user.email=t@example.com",
		"-c", "core.autocrlf=false", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false",
		"-c", "init.defaultBranch=main", "-c", "core.symlinks=false",
	}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, stderr.String())
	}
	return out
}

type gitObj struct {
	typ  string
	data []byte
}

// catAll returns every object of the repository via git cat-file.
func catAll(t *testing.T, dir string) map[string]gitObj {
	out := git(t, dir, nil, "cat-file", "--batch-all-objects", "--batch")
	r := bufio.NewReader(bytes.NewReader(out))
	objs := map[string]gitObj{}
	for {
		line, err := r.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		f := strings.Fields(line)
		n, _ := strconv.Atoi(f[2])
		data := make([]byte, n+1)
		if _, err := io.ReadFull(r, data); err != nil {
			t.Fatal(err)
		}
		objs[f[0]] = gitObj{f[1], data[:n]}
	}
	return objs
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeGitPacks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	git(t, dir, nil, "init", "-q")

	var big strings.Builder
	for i := 0; i < 2000; i++ {
		fmt.Fprintf(&big, "line %d of a reasonably long file used to force deltas\n", i)
	}
	for rev := 0; rev < 4; rev++ {
		content := big.String() + fmt.Sprintf("revision %d\n", rev)
		writeFile(t, filepath.Join(dir, "big.txt"), content)
		writeFile(t, filepath.Join(dir, "src", "deep", "nested", "big.txt"), strings.Replace(content, "line 5 ", "LINE 5 ", 1)+fmt.Sprint(rev))
		writeFile(t, filepath.Join(dir, "a", "b", "c.txt"), fmt.Sprintf("c %d\n", rev))
		writeFile(t, filepath.Join(dir, "run.sh"), "#!/bin/sh\necho hi\n")
		writeFile(t, filepath.Join(dir, "tooth.json"), `{"format_version":3}`)
		writeFile(t, filepath.Join(dir, "empty"), "")
		git(t, dir, nil, "add", "-A")
		git(t, dir, nil, "update-index", "--chmod=+x", "run.sh")
		if rev == 3 {
			// A symlink and a gitlink, added without touching the filesystem.
			link := strings.TrimSpace(string(git(t, dir, []byte("big.txt"), "hash-object", "-w", "--stdin")))
			git(t, dir, nil, "update-index", "--add", "--cacheinfo", "120000,"+link+",link")
			git(t, dir, nil, "update-index", "--add", "--cacheinfo", "160000,"+strings.Repeat("1", 40)+",sub/module")
		}
		git(t, dir, nil, "commit", "-q", "-m", fmt.Sprintf("rev %d", rev))
	}
	git(t, dir, nil, "tag", "-a", "v1.0.0", "-m", "release")
	git(t, dir, nil, "tag", "-a", "v1.0.0-wrapped", "-m", "tag of tag", "v1.0.0")

	want := catAll(t, dir)
	revList := git(t, dir, nil, "rev-list", "--objects", "--all")
	tagID := strings.TrimSpace(string(git(t, dir, nil, "rev-parse", "v1.0.0-wrapped")))

	for _, args := range [][]string{
		{"pack-objects", "--stdout"},
		{"pack-objects", "--stdout", "--delta-base-offset"},
		{"pack-objects", "--stdout", "--delta-base-offset", "--depth=50", "--window=50"},
		{"pack-objects", "--stdout", "--window=0"},
	} {
		t.Run(strings.Join(args[2:], " "), func(t *testing.T) {
			pack := git(t, dir, revList, args...)
			if strings.Contains(strings.Join(args, " "), "window=50") {
				assertHasDeltas(t, dir, pack)
			}
			s := NewStore()
			if err := s.Decode(pack); err != nil {
				t.Fatal(err)
			}
			if s.Len() != len(want) {
				t.Errorf("decoded %d objects, git has %d", s.Len(), len(want))
			}
			for oid, w := range want {
				o, ok := s.Get(oid)
				if !ok {
					t.Errorf("missing %s", oid)
					continue
				}
				if o.Type.String() != w.typ || !bytes.Equal(o.Data, w.data) {
					t.Errorf("object %s differs: %s vs %s", oid, o.Type, w.typ)
				}
			}
			checkTree(t, dir, s, tagID)
		})
	}

	// Thin pack: REF_DELTA bases from an earlier decode.
	t.Run("thin", func(t *testing.T) {
		s := NewStore()
		old := git(t, dir, []byte("HEAD~1\n"), "pack-objects", "--revs", "--stdout")
		if err := s.Decode(old); err != nil {
			t.Fatal(err)
		}
		before := s.Len()
		thin := git(t, dir, []byte("HEAD\n^HEAD~1\n"), "pack-objects", "--revs", "--thin", "--stdout")
		if err := NewStore().Decode(thin); !errors.Is(err, ErrCorrupt) {
			t.Fatal("thin pack had no external bases")
		}
		if err := s.Decode(thin); err != nil {
			t.Fatal(err)
		}
		if s.Len() <= before {
			t.Fatal("nothing added")
		}
		head := strings.TrimSpace(string(git(t, dir, nil, "rev-parse", "HEAD")))
		checkTree(t, dir, s, head)
	})
}

func assertHasDeltas(t *testing.T, dir string, pack []byte) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "x.pack")
	if err := os.WriteFile(p, pack, 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, nil, "index-pack", p)
	out := string(git(t, dir, nil, "verify-pack", "-v", strings.TrimSuffix(p, ".pack")+".idx"))
	if !strings.Contains(out, "chain length") {
		t.Fatalf("pack has no deltas:\n%s", out)
	}
}

// checkTree compares ListTree/WalkFiles of commitish with git ls-tree -r.
func checkTree(t *testing.T, dir string, s *Store, commitish string) {
	t.Helper()
	out := git(t, dir, nil, "ls-tree", "-r", "-z", commitish)
	var want, wantRegular []File
	for _, rec := range strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		meta, path, _ := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if f[1] == "commit" {
			continue
		}
		file := File{Path: path, OID: f[2], Mode: strings.TrimLeft(f[0], "0")}
		want = append(want, file)
		if f[0] != "120000" {
			wantRegular = append(wantRegular, file)
		}
	}
	sortFiles := func(fs []File) {
		sort.Slice(fs, func(i, j int) bool { return fs[i].Path < fs[j].Path })
	}
	got, err := s.ListTree(commitish)
	if err != nil {
		t.Fatal(err)
	}
	sortFiles(got)
	sortFiles(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListTree:\n got %+v\nwant %+v", got, want)
	}
	commit, err := s.PeelCommit(commitish)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := s.CommitTree(commit)
	if err != nil {
		t.Fatal(err)
	}
	regular, err := s.WalkFiles(tree)
	if err != nil {
		t.Fatal(err)
	}
	sortFiles(regular)
	sortFiles(wantRegular)
	if !reflect.DeepEqual(regular, wantRegular) {
		t.Errorf("WalkFiles:\n got %+v\nwant %+v", regular, wantRegular)
	}
	var sawExec bool
	for _, f := range regular {
		o, _ := s.Get(f.OID)
		if err := VerifyBlob(f.OID, o.Data); err != nil {
			t.Error(err)
		}
		sawExec = sawExec || f.Mode == "100755"
	}
	if !sawExec {
		t.Error("no executable file found")
	}
}

func TestIsSafeEntryName(t *testing.T) {
	for name, want := range map[string]bool{"a": true, "a.b": true, "": false, ".": false, "..": false,
		"a/b": false, `a\b`: false, `\`: false} {
		if got := isSafeEntryName(name); got != want {
			t.Errorf("isSafeEntryName(%q) = %v, want %v", name, got, want)
		}
	}
}
