package fetch

import (
	"errors"
	"io/fs"
	"os"
)

// The error types of lip 0.38's download plumbing. Like the .NET exceptions
// they port, Error returns the error's own message only; its causes are
// reached with errors.Unwrap / errors.As (lip's ErrorReporter prints each
// cause that adds something as "caused by: <message>").

// UnavailableError is lip's ArchiveUnavailableException
// (src/Lip.Core/Mcmod/Archives.cs): an archive or asset cannot be obtained.
type UnavailableError struct {
	Message string
	Err     error
}

func (e *UnavailableError) Error() string { return e.Message }
func (e *UnavailableError) Unwrap() error { return e.Err }

func unavailable(message string, cause error) error {
	return &UnavailableError{Message: message, Err: cause}
}

// FileError is a file system failure with a message saying what lip was
// doing (FileSystemExtensions.WrapFileError): an IOException, or an
// UnauthorizedAccessException when errors.Is(e, fs.ErrPermission).
type FileError struct {
	Message string
	Err     error
}

func (e *FileError) Error() string { return e.Message }
func (e *FileError) Unwrap() error { return e.Err }

func wrapFileError(message string, cause error) error {
	return &FileError{Message: message, Err: cause}
}

// AggregateError is .NET's AggregateException: Message and every failure
// that led to it.
type AggregateError struct {
	Message string
	Errs    []error
}

func (e *AggregateError) Error() string   { return e.Message }
func (e *AggregateError) Unwrap() []error { return e.Errs }

// DownloadError is the HttpRequestException of FileDownloader: a failure of
// the network or of the server, naming the URL. StatusCode is the HTTP
// status when the server answered, else 0.
type DownloadError struct {
	Message    string
	StatusCode int
	Err        error
}

func (e *DownloadError) Error() string { return e.Message }
func (e *DownloadError) Unwrap() error { return e.Err }

// URLFormatError is .NET's UriFormatException: a URL that cannot be
// requested. A GitHub proxy that does not give a valid URL fails with it.
type URLFormatError struct {
	URL string
	Err error
}

func (e *URLFormatError) Error() string {
	return "Invalid URI: The format of the URI " + e.URL + " could not be determined."
}
func (e *URLFormatError) Unwrap() error { return e.Err }

// isFileError is FileSystemExtensions.IsFileError: a failure of the file
// system (IOException or UnauthorizedAccessException). Like a .NET type
// test it looks at the error itself, seeing only through plain wrappers
// (fmt.Errorf("%w")): an UnavailableError or DownloadError caused by a file
// error is not one.
func isFileError(err error) bool {
	for err != nil {
		switch err.(type) {
		case *FileError, *fs.PathError, *os.LinkError, *os.SyscallError:
			return true
		case *UnavailableError, *DownloadError, *AggregateError, *URLFormatError:
			return false
		}
		err = errors.Unwrap(err)
	}
	return false
}

// isIOException is isFileError without access failures, which .NET raises
// as UnauthorizedAccessException rather than IOException.
func isIOException(err error) bool {
	return isFileError(err) && !errors.Is(err, fs.ErrPermission)
}
