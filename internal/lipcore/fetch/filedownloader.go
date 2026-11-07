package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/lipcore/httpclient"
	"github.com/liteldev/LeviLauncher/internal/lipcore/report"
)

// downloadIdleTimeout aborts a download when no response or body data
// arrives for this long: Flurl's default request timeout of 100 seconds.
// A variable for tests.
var downloadIdleTimeout = 100 * time.Second

// errIdle cancels a download that stalled.
var errIdle = errors.New("download stalled")

// downloadBufferSize is FileDownloader's read buffer (81920 bytes).
const downloadBufferSize = 81920

// DownloadFile is lip 0.38's FileDownloader.DownloadFile
// (src/Lip.Core/Infrastructure/FileDownloader.cs): it writes the body of a
// successful (2xx) response to one GET of rawURL, following redirects
// (httpclient.CheckRedirect unless client has its own policy), to dest,
// creating or truncating it. There are no retries.
//
// Once the server answered 2xx, the download is reported to r as one
// measured StageDownload bar "Downloading <rawURL>": bytes received out of
// Content-Length (0 when unknown), throttled, ending with exactly one Done
// update whether the download completed or failed.
//
// Failures of the network or of the server are *DownloadError naming the
// URL (with the HTTP status when the server answered); failures of writing
// dest are *FileError; a URL that cannot be requested is *URLFormatError.
// When ctx is done, ctx.Err() is returned.
func DownloadFile(ctx context.Context, client *http.Client, rawURL, dest string, r report.Reporter) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if u, perr := url.Parse(rawURL); perr != nil || !u.IsAbs() || u.Host == "" {
		return &URLFormatError{URL: rawURL, Err: perr}
	}

	idle := downloadIdleTimeout
	reqCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	timer := time.AfterFunc(idle, func() { cancel(errIdle) })
	defer timer.Stop()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return &URLFormatError{URL: rawURL, Err: err}
	}
	resp, err := httpclient.WithLipRedirects(client).Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(context.Cause(reqCtx), errIdle) {
			return &DownloadError{Message: fmt.Sprintf(
				"Could not download %s: the server did not answer in time.", rawURL), Err: err}
		}
		return &DownloadError{Message: fmt.Sprintf(
			"Could not download %s: the connection to the server failed.", rawURL), Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &DownloadError{Message: describeStatus(rawURL, resp), StatusCode: resp.StatusCode}
	}

	length := resp.ContentLength // -1 when unknown
	r = report.OrNop(r)
	id := newGUIDN()
	message := "Downloading " + rawURL
	tr := &transferReporter{
		fn: func(t transfer) {
			report.Report(r, report.Progress{
				ID: id, Stage: report.StageDownload, Message: message, Item: rawURL,
				Current: t.Current, Total: t.Total, Unit: report.UnitBytes, Done: t.Done,
			})
		},
		t: transfer{Total: max(length, 0)},
	}
	tr.update(0)
	defer func() { tr.finish(err) }()

	file, err := os.Create(dest)
	if err != nil {
		return wrapFileError(fmt.Sprintf(
			"Could not create the file %s to save the download of %s in.", dest, rawURL), err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = wrapFileError(fmt.Sprintf(
				"Could not write the download of %s to %s.", rawURL, dest), cerr)
		}
	}()

	var total int64
	buf := make([]byte, downloadBufferSize)
	for {
		n, rerr := resp.Body.Read(buf)
		timer.Reset(idle)
		if n > 0 {
			if _, werr := file.Write(buf[:n]); werr != nil {
				return wrapFileError(fmt.Sprintf(
					"Could not write the download of %s to %s.", rawURL, dest), werr)
			}
			total += int64(n)
			tr.update(total)
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			of := ""
			if length > 0 {
				of = fmt.Sprintf(" of %d", length)
			}
			return &DownloadError{Message: fmt.Sprintf(
				"The download of %s broke off after %d bytes%s. Check your network connection and try again.",
				rawURL, total, of), Err: rerr}
		}
	}
}

// describeStatus is FileDownloader.DescribeStatus: the message for a
// response of a status other than 2xx.
func describeStatus(rawURL string, resp *http.Response) string {
	status := resp.StatusCode
	reason := ""
	if phrase := strings.TrimSpace(strings.TrimPrefix(resp.Status, strconv.Itoa(status))); phrase != "" {
		reason = " (" + phrase + ")"
	}
	message := fmt.Sprintf("Could not download %s: the server answered HTTP %d%s.", rawURL, status, reason)
	switch {
	case (status == 401 || status == 404) && isGitHub(rawURL):
		return message + " GitHub answers this way when the repository" +
			" or the file does not exist, or when the repository is private."
	case status == 401:
		return message + " The server does not allow lip to download this file."
	case status == 403:
		return message
	case status == 404 || status == 410:
		return message + " The file does not exist there. Check the URL, or the package" +
			" name and version that lead to it."
	case status == 429:
		return message + " Too many requests were sent. Wait a while and try again."
	case status >= 500:
		return message + " The server had a problem. Try again later."
	default:
		return message
	}
}

// isGitHub is FileDownloader.IsGitHub: github.com, codeload.github.com,
// *.githubusercontent.com, or a path containing "github.com/" (a GitHub
// URL through a proxy).
func isGitHub(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "github.com" || host == "codeload.github.com" ||
		strings.HasSuffix(host, ".githubusercontent.com") ||
		strings.Contains(strings.ToLower(u.EscapedPath()), "github.com/")
}

// transfer is measured progress of one download: bytes received out of the
// response's Content-Length.
type transfer struct {
	// Current is the number of bytes received so far.
	Current int64
	// Total is the Content-Length, or 0 when the server sent none.
	Total int64
	// Done marks the last update; Err is the download's error, if it failed.
	Done bool
	Err  error
}

// transferReporter throttles the transfer updates of one download: the
// first update, then at most one per 0.1% of the total (or per 64 KiB
// without a total), and the Done update.
type transferReporter struct {
	fn   func(transfer)
	t    transfer
	last int64 // Current of the last update sent
	sent bool
}

// transferByteStep is the byte step of updates without a total.
const transferByteStep = 64 << 10

func (r *transferReporter) update(current int64) {
	r.t.Current = current
	if r.fn == nil {
		return
	}
	if r.sent {
		if r.t.Total > 0 && current*1000/r.t.Total <= r.last*1000/r.t.Total {
			return
		}
		if r.t.Total <= 0 && current/transferByteStep <= r.last/transferByteStep {
			return
		}
	}
	r.sent = true
	r.last = current
	r.fn(r.t)
}

func (r *transferReporter) finish(err error) {
	if r.fn == nil {
		return
	}
	r.t.Done, r.t.Err = true, err
	r.fn(r.t)
}
