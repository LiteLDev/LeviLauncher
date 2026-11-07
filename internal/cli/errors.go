package cli

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Exit codes. Every command exits with one of these (launch --wait-exit
// excepted, which passes the game's own code through).
const (
	ExitOK          = 0   // success, including an idempotent no-op
	ExitFailure     = 1   // the operation failed
	ExitUsage       = 2   // bad command line, ambiguous or missing choice, confirmation needed
	ExitNotFound    = 3   // instance, build, world, pack, mod, package, PID, key or path not found
	ExitConflict    = 4   // already exists, game running, registered, in use, unsupported here
	ExitAuth        = 5   // not signed in, not entitled, interaction required
	ExitNetwork     = 6   // network or remote service unreachable
	ExitPartial     = 7   // some items of a batch failed
	ExitSystem      = 8   // needs Developer Mode, administrator rights or an interactive desktop
	ExitTimeout     = 9   // timed out
	ExitCancelled   = 10  // the user declined at a prompt or the account picker
	ExitInterrupted = 130 // Ctrl-C
)

// Error is a failure reported to the user:
//
//	error: <Message>
//	hint: <Hint>
//	[<Code>]
//
// and, with --json, {"error":{"code","message","hint"}} on stderr. Detail is
// printed only with -v.
type Error struct {
	Code    string // stable ERR_* identifier
	Message string // one plain sentence naming the thing, no trailing period needed
	Hint    string // a runnable command or a short instruction; optional
	Exit    int    // one of the Exit* codes
	Detail  string // raw backend detail, shown with -v
	Err     error  // wrapped cause, for errors.Is/As
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func (e *Error) Unwrap() error { return e.Err }

// WithHint returns e with its hint set.
func (e *Error) WithHint(format string, a ...any) *Error {
	e.Hint = fmt.Sprintf(format, a...)
	return e
}

// WithDetail returns e with its detail set.
func (e *Error) WithDetail(detail string) *Error {
	e.Detail = detail
	return e
}

// Errorf builds an Error.
func Errorf(exit int, code, format string, a ...any) *Error {
	return &Error{Code: code, Exit: exit, Message: fmt.Sprintf(format, a...)}
}

// UsageError is a command-line mistake (exit 2, ERR_USAGE).
func UsageError(format string, a ...any) *Error {
	return Errorf(ExitUsage, "ERR_USAGE", format, a...)
}

// NotFoundError reports a missing object (exit 3, ERR_NOT_FOUND).
func NotFoundError(format string, a ...any) *Error {
	return Errorf(ExitNotFound, "ERR_NOT_FOUND", format, a...)
}

// ConflictError reports a state conflict (exit 4, ERR_CONFLICT).
func ConflictError(format string, a ...any) *Error {
	return Errorf(ExitConflict, "ERR_CONFLICT", format, a...)
}

// ExitCode ends the command with code without printing anything (output was
// already written, e.g. a batch summary or a game's exit code).
type ExitCode int

func (e ExitCode) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

// BackendError describes how a backend ERR_* code is shown and which exit
// code it maps to. Empty Message falls back to the backend's detail text.
type BackendError struct {
	Exit    int
	Message string
	Hint    string
}

var (
	backendMu     sync.RWMutex
	backendErrors = map[string]BackendError{}
)

// RegisterBackendErrors adds or replaces entries of the backend error table.
// Command files call it from init() for the codes their backend calls
// return, so the table needs no central edits.
func RegisterBackendErrors(m map[string]BackendError) {
	backendMu.Lock()
	defer backendMu.Unlock()
	for k, v := range m {
		backendErrors[strings.TrimSpace(k)] = v
	}
}

func lookupBackend(code string) (BackendError, bool) {
	backendMu.RLock()
	defer backendMu.RUnlock()
	v, ok := backendErrors[code]
	return v, ok
}

// exitForCode guesses the exit code of an unregistered ERR_* code from its
// shape.
func exitForCode(code string) int {
	switch {
	case code == "ERR_CANCELED" || code == "ERR_CANCELLED":
		return ExitCancelled
	case strings.HasPrefix(code, "ERR_AUTH_") || strings.HasPrefix(code, "ERR_LICENSE") || strings.Contains(code, "_LICENSE_") || strings.Contains(code, "ENTITLE"):
		return ExitAuth
	case strings.Contains(code, "TIMEOUT"):
		return ExitTimeout
	case strings.Contains(code, "NOT_FOUND"):
		return ExitNotFound
	case strings.HasSuffix(code, "_EXISTS") || strings.Contains(code, "ALREADY") || strings.Contains(code, "_BUSY") || strings.Contains(code, "IN_USE") || strings.Contains(code, "RUNNING"):
		return ExitConflict
	case strings.Contains(code, "NETWORK") || strings.Contains(code, "_HTTP") || strings.Contains(code, "UNREACHABLE") || strings.Contains(code, "_UNAVAILABLE"):
		return ExitNetwork
	case strings.Contains(code, "ADMIN") || strings.Contains(code, "ELEVAT") || strings.Contains(code, "DEVELOPER_MODE"):
		return ExitSystem
	}
	return ExitFailure
}

var codePattern = regexp.MustCompile(`^(ERR_[A-Z0-9_]+)\s*(?:::|:)?\s*(.*)$`)

// SplitBackendCode splits "ERR_X", "ERR_X: detail" or "ERR_X::detail" into
// code and detail. ok is false when s does not start with an ERR_ code.
func SplitBackendCode(s string) (code, detail string, ok bool) {
	m := codePattern.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", "", false
	}
	return m[1], strings.TrimSpace(m[2]), true
}

// Backend converts a backend result string ("" = success, "ERR_X" or
// "ERR_X: detail") into an *Error, using the registered table; nil for "".
// Text without an ERR_ code becomes a generic failure with that text.
func Backend(result string) *Error {
	result = strings.TrimSpace(result)
	if result == "" {
		return nil
	}
	code, detail, ok := SplitBackendCode(result)
	if !ok {
		return &Error{Code: "ERR_FAILED", Exit: ExitFailure, Message: result}
	}
	e := &Error{Code: code, Exit: exitForCode(code), Detail: detail}
	if info, found := lookupBackend(code); found {
		if info.Exit != 0 {
			e.Exit = info.Exit
		}
		e.Message, e.Hint = info.Message, info.Hint
	}
	if e.Message == "" {
		if detail != "" {
			e.Message, e.Detail = detail, ""
		} else {
			e.Message = humanizeCode(code)
		}
	}
	return e
}

// FromError converts any error into an *Error: *Error passes through, text
// starting with an ERR_ code goes through Backend, context cancellation is
// ERR_INTERRUPTED (exit 130), a deadline is ERR_TIMEOUT, anything else is a
// generic failure.
func FromError(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	switch {
	case errors.Is(err, context.Canceled):
		return &Error{Code: "ERR_INTERRUPTED", Exit: ExitInterrupted, Message: "interrupted", Err: err}
	case errors.Is(err, context.DeadlineExceeded):
		return &Error{Code: "ERR_TIMEOUT", Exit: ExitTimeout, Message: "timed out", Err: err}
	}
	if b := Backend(err.Error()); b != nil {
		b.Err = err
		return b
	}
	return &Error{Code: "ERR_FAILED", Exit: ExitFailure, Message: err.Error(), Err: err}
}

// humanizeCode turns ERR_NOT_FOUND_EXE into "not found exe" as a last resort
// for unregistered codes without detail.
func humanizeCode(code string) string {
	s := strings.ToLower(strings.TrimPrefix(code, "ERR_"))
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return "the operation failed"
	}
	return "the operation failed: " + s
}

func init() {
	RegisterBackendErrors(map[string]BackendError{
		"ERR_CANCELED":            {Exit: ExitCancelled, Message: "cancelled"},
		"ERR_INVALID_NAME":        {Exit: ExitUsage, Message: "invalid name"},
		"ERR_PATH_ESCAPE":         {Exit: ExitUsage, Message: "the name must not contain a path"},
		"ERR_ACCESS_VERSIONS_DIR": {Exit: ExitFailure, Message: "cannot access the instances folder"},
		"ERR_CONFIG_CORRUPTED":    {Exit: ExitFailure, Message: "the launcher settings file is corrupted"},
		"ERR_CONFIG_READ_FAILED":  {Exit: ExitFailure, Message: "cannot read the launcher settings file"},
		"ERR_CONFIG_WRITE_FAILED": {Exit: ExitFailure, Message: "cannot write the launcher settings file"},
		"ERR_DOWNLOAD_BUSY":       {Exit: ExitConflict, Message: "another download of this file is running"},
		"ERR_MD5_MISMATCH":        {Exit: ExitFailure, Message: "the download is corrupted (checksum mismatch)", Hint: "run the same command again"},
		"ERR_GAME_ALREADY_RUNNING": {Exit: ExitConflict, Message: "Minecraft is already running for this instance",
			Hint: "LeviLauncher.exe stop <instance>"},
		"ERR_LAUNCH_TIMEOUT": {Exit: ExitTimeout, Message: "the game did not start in time"},
		"ERR_LAUNCH_GAME":    {Exit: ExitFailure, Message: "the game failed to start"},
		"ERR_NOT_FOUND_EXE":  {Exit: ExitNotFound, Message: "the game executable is missing from this instance", Hint: "reinstall the instance"},
		"ERR_UPDATE_ADMIN_REQUIRED": {Exit: ExitSystem, Message: "the launcher folder is not writable",
			Hint: "run the command from an elevated (administrator) terminal"},
		"ERR_UPDATE_NOT_AVAILABLE":       {Exit: ExitFailure, Message: "no newer LeviLauncher release is available"},
		"ERR_UPDATE_RELEASE_UNAVAILABLE": {Exit: ExitNetwork, Message: "cannot reach the release server"},
		"ERR_UPDATE_DOWNLOAD_FAILED":     {Exit: ExitNetwork, Message: "downloading the update failed"},
		"ERR_UPDATE_INSTALL_FAILED":      {Exit: ExitFailure, Message: "installing the update failed"},
		"ERR_UPDATE_WRITE_FAILED":        {Exit: ExitFailure, Message: "cannot write the update files"},
		"ERR_AUTH_INTERACTION_REQUIRED":  {Exit: ExitAuth, Message: "not signed in to a Microsoft account", Hint: "LeviLauncher.exe account login"},
		"ERR_AUTH_TIMEOUT":               {Exit: ExitTimeout, Message: "signing in timed out"},
		"ERR_AUTH_FAILED":                {Exit: ExitAuth, Message: "signing in to the Microsoft account failed"},
		"ERR_AUTH_ACCOUNT_CHANGED":       {Exit: ExitAuth, Message: "the signed-in Microsoft account changed during the operation", Hint: "run the command again"},
		"ERR_AUTH_DEVICE_BUSY":           {Exit: ExitConflict, Message: "another launcher process is using the account"},
		"ERR_NO_INTERACTIVE_DESKTOP":     {Exit: ExitSystem, Message: "signing in needs an interactive Windows desktop"},
	})
}
