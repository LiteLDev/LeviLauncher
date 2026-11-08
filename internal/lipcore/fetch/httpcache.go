package fetch

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipcore/lipdirs"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

// HTTPCache is lip 0.38's cache (protocol/userspace.md §3,
// src/Lip.Core/Services/CacheService.cs) together with the download of
// ZIP archives into it (src/Lip.Core/Mcmod/Archives/ArchiveProvider.cs).
//
// Entries live at <Root>/<EscapeKey(key)>. A missing entry is written
// atomically: to <Root>/<N>.tmp, then renamed into place without replacing
// an existing entry. Entries are never modified in place.
type HTTPCache struct {
	// Root is the cache directory (lipdirs.CacheDir() for lip's).
	Root string
	// Reporter receives the cache messages ("Cache hit for key '<key>'.",
	// "Caching file for key '<key>'...") and download progress. nil
	// discards them.
	Reporter report.Reporter
}

// NewHTTPCache returns the cache at root reporting to r (may be nil).
func NewHTTPCache(root string, r report.Reporter) *HTTPCache {
	return &HTTPCache{Root: root, Reporter: r}
}

// DefaultHTTPCache returns lip's cache, lipdirs.CacheDir(), reporting to r.
func DefaultHTTPCache(r report.Reporter) *HTTPCache {
	return NewHTTPCache(lipdirs.CacheDir(), r)
}

// HTTPKey is CacheKeys.Http: "http-" + the lowercase hex SHA-256 of the
// UTF-8 URL. Callers pass the URL without its fragment.
func HTTPKey(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return "http-" + hex.EncodeToString(sum[:])
}

func (c *HTTPCache) rep() report.Reporter { return report.OrNop(c.Reporter) }

// Path returns the entry path for key, whether or not it exists.
func (c *HTTPCache) Path(key string) string {
	return filepath.Join(c.Root, EscapeKey(key))
}

// TryGet is CacheService.TryGetFile: the cached file for key, if it is
// cached. It never creates anything and reports nothing.
func (c *HTTPCache) TryGet(key string) (string, bool) {
	p := c.Path(key)
	if fileExists(p) {
		return p, true
	}
	return "", false
}

// fileExists is .NET FileInfo.Exists: true for an existing file that is
// not a directory.
func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// newGUIDN returns a random version-4 GUID in .NET's "N" format (32
// lowercase hex digits).
func newGUIDN() string {
	b := newGUIDBytes()
	return hex.EncodeToString(b[:])
}

// newGUIDBytes returns the 16 bytes of a random version-4 GUID.
func newGUIDBytes() [16]byte {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return b
}

// GetOrCreate is CacheService.GetOrCreateFile: it returns the cached file
// for key, reporting "Cache hit for key '<key>'.", or reports "Caching file
// for key '<key>'..." and calls factory with the path of a new
// <Root>/<N>.tmp file (not created yet) to write, which is then renamed to
// the entry. If the entry appeared meanwhile (another process), the rename
// fails and the existing entry is returned; so is it when factory fails
// with an IOException-like error while the entry exists. The .tmp file is
// always deleted afterwards. factory's errors are returned as they are;
// failures of the cache itself are *FileError naming the path.
func (c *HTTPCache) GetOrCreate(key string, factory func(tmpPath string) error) (string, error) {
	target := c.Path(key)
	if fileExists(target) {
		c.rep().Info(fmt.Sprintf("Cache hit for key '%s'.", key))
		return target, nil
	}

	c.rep().Info(fmt.Sprintf("Caching file for key '%s'...", key))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", wrapFileError(fmt.Sprintf(
			"Could not create the lip cache directory %s. Check that you may write there.",
			filepath.Dir(target)), err)
	}

	tmp := filepath.Join(c.Root, newGUIDN()+".tmp")
	defer os.Remove(tmp) // a leftover temporary file does no harm

	err := factory(tmp)
	if err == nil {
		if merr := moveNoReplace(tmp, target); merr != nil {
			if !fileExists(target) {
				err = wrapFileError(fmt.Sprintf(
					"Could not move %s into the lip cache as %s. Check that you may write to the"+
						" cache and that the disk is not full.", tmp, target), merr)
			}
			// Otherwise another process has created the entry: use it.
		}
	} else if isIOException(err) && fileExists(target) {
		// Another process has already created the entry.
		err = nil
	}
	if err != nil {
		return "", err
	}
	return target, nil
}

// Forget is ArchiveProvider.Forget: it deletes the cached download of
// rawURL, if any.
func (c *HTTPCache) Forget(rawURL string) error {
	p, ok := c.TryGet(HTTPKey(rawURL))
	if !ok {
		return nil
	}
	return deleteCached(p, rawURL)
}

// deleteCached is ArchiveProvider.DeleteCached.
func deleteCached(path, rawURL string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return wrapFileError(fmt.Sprintf(
			"Could not delete the cached download of %s (%s). Stop other lip processes and"+
				" programs that may use it, or delete the file yourself, then try again.",
			rawURL, path), err)
	}
	return nil
}

// Clean is CacheService.Clean: it clears read-only attributes below Root
// and deletes the cache directory (and nothing else). A missing directory
// is not an error.
func (c *HTTPCache) Clean() error {
	return c.CleanProgress(context.Background(), nil)
}

// CleanProgress is Clean reporting progress: it first walks Root (clearing
// read-only attributes) to count its entries, files and directories
// including Root itself, then deletes them one by one, children before
// their directory, calling progress (if not nil) with the entries deleted
// out of that count: once with 0 before deleting and after every entry.
// ctx is checked between entries; a canceled clean leaves the remaining
// entries in place. If an entry cannot be deleted on its own (e.g. a file
// appeared during the clean), the rest is deleted with os.RemoveAll. A
// missing directory reports nothing.
func (c *HTTPCache) CleanProgress(ctx context.Context, progress func(done, total int64)) error {
	err := cleanDir(ctx, c.Root, progress)
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return wrapFileError(fmt.Sprintf(
		"Could not delete the lip cache %s. Stop other lip processes and programs that use"+
			" files in it, then try again, or delete the directory yourself.", c.Root), err)
}

// CreateTemp creates a new <Root>/<N>.tmp file, open for reading and
// writing, that is deleted when it is closed (GitRemoteClient.
// CreateTempFile; lip keeps nothing in the system temp directory).
func (c *HTTPCache) CreateTemp() (*os.File, error) {
	if err := os.MkdirAll(c.Root, 0o755); err != nil {
		return nil, err
	}
	return createDeleteOnClose(filepath.Join(c.Root, newGUIDN()+".tmp"))
}

// DownloadZip is ArchiveProvider.OpenAsset's download: it returns the
// cached download of the first of urls (an asset's URLs) that can be
// obtained as a valid ZIP archive (see DownloadZipURL). A URL that is not
// https fails as unavailable and the next one is tried. Only
// *UnavailableError failures move on to the next URL; others (failing to
// delete a damaged cached entry, cancellation) are returned at once. As in
// lip, a failure of writing a candidate's download is one of that URL's
// candidate failures. With one failure it is returned; with several, an
// *UnavailableError joining them.
func (c *HTTPCache) DownloadZip(ctx context.Context, urls []string, proxy string, client *http.Client, validate func(path string) error) (string, error) {
	return c.DownloadZipNamed(ctx, urls, proxy, client, ignoreSubject(validate))
}

// ValidateFunc validates a file of the cache: the cached download of a URL
// or a download just made. subject names the file as lip does in the
// messages of ZipValidator, as the subject of a sentence: "The cached
// download of <URL> (<path>)" or "The file downloaded from <download URL>".
type ValidateFunc func(path, subject string) error

func ignoreSubject(validate func(path string) error) ValidateFunc {
	return func(path, _ string) error { return validate(path) }
}

// DownloadZipNamed is DownloadZip with a validation that is told what the
// file is (ValidateFunc).
func (c *HTTPCache) DownloadZipNamed(ctx context.Context, urls []string, proxy string, client *http.Client, validate ValidateFunc) (string, error) {
	if len(urls) == 0 {
		return "", unavailable("The asset lists no URLs to download it from, so the package is"+
			" broken. Report this to the package author.", nil)
	}
	var failures []error
	for _, u := range urls {
		var path string
		var err error
		if !strings.HasPrefix(u, "https://") {
			err = unavailable(fmt.Sprintf(
				"The asset URL %s is not an https URL: lip downloads assets only over https.", u), nil)
		} else {
			path, err = c.DownloadZipURLNamed(ctx, u, proxy, client, validate)
		}
		if err == nil {
			return path, nil
		}
		if _, ok := err.(*UnavailableError); !ok {
			return "", err
		}
		failures = append(failures, err)
	}
	if len(failures) == 1 {
		return "", failures[0]
	}
	return "", unavailable(fmt.Sprintf("Could not download the asset from any of its %d URLs (%s).",
		len(urls), strings.Join(urls, ", ")),
		&AggregateError{Message: "Every URL of the asset failed.", Errs: failures})
}

// DownloadZipURL is ArchiveProvider.DownloadZip: the cached download of
// the ZIP archive at rawURL, keyed by HTTPKey of the URL without its
// fragment. A cached copy is checked with validate; one that fails it
// (damaged) is deleted and downloaded again, while one that cannot be read
// (validate returns a file error) fails. A missing entry is downloaded
// through the GitHub proxy first when RewriteGithub(proxy, url) applies,
// then directly; each candidate's response must pass validate (so a proxy
// error page falls through to the next candidate). validate must return an
// *UnavailableError, or any non file-system error, for an invalid archive.
func (c *HTTPCache) DownloadZipURL(ctx context.Context, rawURL, proxy string, client *http.Client, validate func(path string) error) (string, error) {
	return c.DownloadZipURLNamed(ctx, rawURL, proxy, client, ignoreSubject(validate))
}

// DownloadZipURLNamed is DownloadZipURL with a validation that is told what
// the file is (ValidateFunc).
func (c *HTTPCache) DownloadZipURLNamed(ctx context.Context, rawURL, proxy string, client *http.Client, validate ValidateFunc) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	rawURL, _, _ = strings.Cut(rawURL, "#")
	if p, ok := c.TryGet(HTTPKey(rawURL)); ok {
		err := validate(p, fmt.Sprintf("The cached download of %s (%s)", rawURL, p))
		switch {
		case err == nil:
			return p, nil
		case errors.Is(err, fs.ErrNotExist):
			// Deleted meanwhile: download it again.
			return c.download(ctx, rawURL, proxy, client, validate)
		}
		if isFileError(err) {
			return "", unavailable(fmt.Sprintf(
				"Could not read the cached download of %s (%s). If a program is using the file,"+
					" close it; otherwise delete the file or run 'lip cache clean', then try again.",
				rawURL, p), err)
		}
		// Damaged: download it again.
		if err := deleteCached(p, rawURL); err != nil {
			return "", err
		}
	}
	return c.download(ctx, rawURL, proxy, client, validate)
}

// download is ArchiveProvider.Download.
func (c *HTTPCache) download(ctx context.Context, rawURL, proxy string, client *http.Client, validate ValidateFunc) (string, error) {
	candidates := []string{rawURL}
	if proxied, ok := RewriteGithub(proxy, rawURL); ok {
		candidates = []string{proxied, rawURL}
	}
	path, err := c.GetOrCreate(HTTPKey(rawURL), func(dest string) error {
		var errs []error
		for i, u := range candidates {
			if err := ctx.Err(); err != nil {
				return err
			}
			viaProxy := len(candidates) > 1 && i == 0
			err := c.downloadCandidate(ctx, u, dest, client, validate)
			if err == nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var fe *URLFormatError
			if viaProxy && errors.As(err, &fe) {
				// An invalid github_proxy value is accepted by the configuration and fails
				// only here.
				err = unavailable(fmt.Sprintf(
					"The GitHub proxy set in github_proxy does not give a valid URL (%s). Set a"+
						" valid URL with 'lip config set github_proxy <URL>', or remove it with"+
						" 'lip config delete github_proxy'.", u), err)
			}
			errs = append(errs, err)
		}
		if len(errs) == 1 {
			return unavailable(fmt.Sprintf("Could not download %s.", rawURL), errs[0])
		}
		return unavailable(fmt.Sprintf(
			"Could not download %s, neither through the GitHub proxy (%s) nor directly.",
			rawURL, strings.Join(candidates[:len(candidates)-1], ", ")),
			&AggregateError{Message: "Every download address failed.", Errs: errs})
	})
	if err != nil {
		if _, ok := err.(*UnavailableError); ok || ctx.Err() != nil {
			return "", err
		}
		return "", unavailable(fmt.Sprintf(
			"Could not store the download of %s in the lip cache (%s).", rawURL, c.Root), err)
	}
	return path, nil
}

// downloadCandidate downloads one candidate URL to dest and validates it.
// Leftovers of a failed candidate are deleted first, so they cannot be
// mistaken for this one's content.
func (c *HTTPCache) downloadCandidate(ctx context.Context, u, dest string, client *http.Client, validate ValidateFunc) error {
	if err := os.Remove(dest); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := DownloadFile(ctx, client, u, dest, c.Reporter); err != nil {
		return err
	}
	return validate(dest, "The file downloaded from "+u)
}
