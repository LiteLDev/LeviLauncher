package gitproto

// Network tests, ports of lip 0.38.0-beta.1
// tests/Lip.Core.RegressionTests/Git/GitRemoteClientTests.cs plus a
// differential check against the lip 0.38 CLI. Run with LIP_NET_E2E=1;
// LIP_038_EXE overrides the lip executable compared against.

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	xsemver "golang.org/x/mod/semver"

	"github.com/liteldev/LeviLauncher/internal/lipcore/gitpack"
)

// strictSemver is the SemVer 2.0 regular expression of semver.org.
var strictSemver = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
	`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
	`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

func netE2E(t *testing.T) {
	if os.Getenv("LIP_NET_E2E") != "1" {
		t.Skip("set LIP_NET_E2E=1 to run")
	}
}

func lip038() string {
	if p := os.Getenv("LIP_038_EXE"); p != "" {
		return p
	}
	return filepath.Join(os.TempDir(), "lip-0.38", "lip.exe")
}

func TestNetRemoteLeviLamina(t *testing.T) {
	netE2E(t)
	const module = "github.com/LiteLDev/LeviLamina"
	repo := "https://" + module + ".git"
	ctx := context.Background()
	c := &RemoteClient{TempDir: t.TempDir()}

	tags, err := c.ListVersionTags(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	var versions []string
	var newest TagRef
	for _, tag := range tags {
		if !strictSemver.MatchString(tag.Version) {
			continue
		}
		versions = append(versions, tag.Version)
		if newest.Name == "" || xsemver.Compare("v"+tag.Version, "v"+newest.Version) > 0 {
			newest = tag
		}
	}
	if len(versions) < 90 {
		t.Fatalf("only %d version tags", len(versions))
	}
	t.Logf("%d version tags, newest %s (%s)", len(versions), newest.Name, newest.Commit)

	if exe := lip038(); fileExists(exe) {
		out, err := exec.Command(exe, "versions", module, "--json").Output()
		if err != nil {
			t.Fatalf("lip versions: %v", err)
		}
		var want []string
		if err := json.Unmarshal(out, &want); err != nil {
			t.Fatalf("lip versions output %q: %v", out, err)
		}
		got := slices.Sorted(slices.Values(versions))
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("versions differ from lip 0.38:\n got %v\nwant %v", got, want)
		}
	} else {
		t.Logf("lip 0.38 not found at %s; skipping the comparison", exe)
	}

	commit, err := c.ResolveRef(ctx, repo, RefVersion, newest.Version)
	if err != nil || commit != newest.Commit {
		t.Fatalf("ResolveRef = %s, %v; want %s", commit, err, newest.Commit)
	}

	src, err := c.OpenCommit(ctx, repo, newest.Commit)
	if err != nil {
		t.Fatal(err)
	}
	keys := src.Keys()
	manifest := ""
	for _, k := range []string{"tooth.json", "lip.toml"} {
		if slices.Contains(keys, k) {
			manifest = k
			break
		}
	}
	if manifest == "" {
		t.Fatalf("no tooth.json or lip.toml among %d files", len(keys))
	}
	rc, err := src.Open(manifest)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := c.ListTree(ctx, repo, newest.Commit)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range tree {
		if e.Path == manifest && (e.Mode != 100644 || gitpack.HashObject(gitpack.ObjBlob, data) != e.ObjectID) {
			t.Fatalf("%s: mode %d, id %s, content hashes to %s", manifest, e.Mode, e.ObjectID, gitpack.HashObject(gitpack.ObjBlob, data))
		}
	}
	if manifest == "tooth.json" {
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		if _, ok := m["format_version"]; !ok {
			t.Fatalf("tooth.json without format_version: %s", data)
		}
	} else if !strings.Contains(string(data), "format") {
		t.Fatalf("lip.toml: %s", data)
	}
	t.Logf("%s: %d files, %s of %d bytes", newest.Name, len(keys), manifest, len(data))

	if entries, _ := os.ReadDir(c.TempDir); len(entries) != 0 {
		t.Fatalf("pack files left: %v", entries)
	}
}

func TestNetRemotePeeledAndTagObjectIDsReturnSameEntries(t *testing.T) {
	netE2E(t)
	repo := "https://github.com/LiteLDev/LeviLamina.git"
	c := &RemoteClient{}
	ctx := context.Background()
	refs, err := c.ListRefs(ctx, repo, []string{"refs/tags/v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(refs, func(r RemoteRef) bool { return r.Name == "refs/tags/v1.0.0" })
	if i < 0 {
		t.Fatal("no v1.0.0")
	}
	byTag, err := c.ListTree(ctx, repo, refs[i].ObjectID)
	if err != nil {
		t.Fatal(err)
	}
	peeled := refs[i].Peeled
	if peeled == "" {
		peeled = refs[i].ObjectID
	}
	byCommit, err := c.ListTree(ctx, repo, peeled)
	if err != nil {
		t.Fatal(err)
	}
	if len(byTag) == 0 || !slices.Equal(byTag, byCommit) {
		t.Fatalf("%d vs %d entries", len(byTag), len(byCommit))
	}
}

func TestNetRemoteFetchManyBlobsVerified(t *testing.T) {
	netE2E(t)
	repo := "https://github.com/LiteLDev/LegacyScriptEngine.git"
	c := &RemoteClient{}
	ctx := context.Background()
	heads, err := c.ListRefs(ctx, repo, []string{"refs/heads/"})
	if err != nil || len(heads) == 0 {
		t.Fatalf("%v %v", heads, err)
	}
	tree, err := c.ListTree(ctx, repo, heads[0].ObjectID)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range tree {
		if !slices.Contains(ids, e.ObjectID) && len(ids) < 300 {
			ids = append(ids, e.ObjectID)
		}
	}
	received := map[string]bool{}
	err = c.FetchBlobs(ctx, repo, ids, func(oid string, data []byte) error {
		if gitpack.HashObject(gitpack.ObjBlob, data) != oid || received[oid] {
			t.Errorf("blob %s", oid)
		}
		received[oid] = true
		return nil
	})
	if err != nil || len(received) != len(ids) {
		t.Fatalf("%d of %d, %v", len(received), len(ids), err)
	}
}

func TestNetRemoteBogusProxyFallsThrough(t *testing.T) {
	netE2E(t)
	repo := "https://github.com/LiteLDev/LeviLamina.git"
	ctx := context.Background()
	for _, proxy := range []string{"https://proxy.invalid", "not a url"} {
		c := &RemoteClient{GithubProxy: proxy}
		tags, err := c.ListVersionTags(ctx, repo)
		if err != nil || len(tags) == 0 {
			t.Fatalf("%s: %d tags, %v", proxy, len(tags), err)
		}
		src, err := c.OpenCommit(ctx, repo, tags[0].Commit)
		if err != nil || len(src.Keys()) == 0 {
			t.Fatalf("%s: %v", proxy, err)
		}
		rc, err := src.Open(src.Keys()[0])
		if err != nil {
			t.Fatalf("%s: %v", proxy, err)
		}
		rc.Close()
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
