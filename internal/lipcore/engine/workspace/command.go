package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
)

// commandWaitDelay bounds how long a cancelled script may keep its output
// pipes open (through a process that escaped the kill) before Wait gives up
// on them.
const commandWaitDelay = 2 * time.Second

// startCommand starts a prepared script process; tests replace it.
var startCommand = (*exec.Cmd).Start

// DefaultCommandRunner runs commands like lip's CommandRunner, which uses
// CliWrap: Cli.Wrap("cmd.exe").WithArguments(["/c", command]) on Windows
// ("sh", ["-c", command] elsewhere), stdin from the null device, the working
// directory dir and the inherited environment. On Windows the process
// command line is built exactly like .NET's Process does for CliWrap:
// `"<resolved cmd.exe path>" /c <command escaped by CliWrap>`.
//
// Like CliWrap (CreateNoWindow) the process gets no console window. The
// script runs as a process tree that is killed as a whole when ctx is
// cancelled (lip had no cancellation; the launcher used to kill lipd).
//
// A non-zero exit code yields an error carrying CliWrap's
// CommandExecutionException message.
func DefaultCommandRunner(ctx context.Context, dir, command string, stdout, stderr io.Writer) error {
	shell, flag := "sh", "-c"
	if runtime.GOOS == "windows" {
		shell, flag = "cmd.exe", "/c"
	}

	path, err := exec.LookPath(shell)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, path, flag, command)
	cmd.Dir = dir
	cmd.Stdin = nil // os.DevNull
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = commandWaitDelay
	arguments := flag + " " + escapeArgument(command)
	tree, err := newProcessTree(cmd, `"`+path+`" `+arguments)
	if err != nil {
		return err
	}
	defer tree.release()

	if err := startCommand(cmd); err != nil {
		return err
	}
	if err := tree.started(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return err
	}

	if err := cmd.Wait(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ProcessState != nil && exitErr.ExitCode() >= 0 {
			return &CommandError{
				ProcessName: filepath.Base(path),
				PID:         exitErr.Pid(),
				ExitCode:    exitErr.ExitCode(),
				Command:     shell + " " + arguments,
			}
		}
		return err
	}
	return nil
}

// CommandError is a script that exited with a non-zero code. Its message is
// CliWrap's CommandExecutionException message, which lip reports.
type CommandError struct {
	ProcessName string
	PID         int
	ExitCode    int
	// Command is "<target file path> <arguments>" as CliWrap prints it.
	Command string
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("Command execution failed because the underlying process (%s#%d) returned a non-zero exit code (%d).\n\n"+
		"Command:\n%s\n\n"+
		"You can suppress this validation by calling `WithValidation(CommandResultValidation.None)` on the command.",
		e.ProcessName, e.PID, e.ExitCode, e.Command)
}

// escapeArgument ports CliWrap's ArgumentsBuilder.Escape (the
// CommandLineToArgvW convention): arguments that are non-empty and contain
// no whitespace or double quotes are kept as-is; others are wrapped in
// quotes with backslashes before quotes (and at the end) doubled and quotes
// escaped.
func escapeArgument(arg string) string {
	if arg != "" && !strings.ContainsFunc(arg, func(r rune) bool { return unicode.IsSpace(r) || r == '"' }) {
		return arg
	}

	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(arg); {
		c := arg[i]
		i++
		switch c {
		case '\\':
			n := 1
			for i < len(arg) && arg[i] == '\\' {
				n++
				i++
			}
			switch {
			case i == len(arg):
				b.WriteString(strings.Repeat(`\`, n*2))
			case arg[i] == '"':
				b.WriteString(strings.Repeat(`\`, n*2+1))
				b.WriteByte('"')
				i++
			default:
				b.WriteString(strings.Repeat(`\`, n))
			}
		case '"':
			b.WriteString(`\"`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// lineWriter splits script output into lines for the reporter.
type lineWriter struct {
	emit func(string)
	buf  []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		w.line(w.buf[:i])
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

// Flush emits a trailing partial line.
func (w *lineWriter) Flush() {
	if len(w.buf) > 0 {
		w.line(w.buf)
		w.buf = nil
	}
}

func (w *lineWriter) line(b []byte) {
	s := strings.TrimRight(string(b), "\r")
	if strings.TrimSpace(s) == "" {
		return
	}
	w.emit(s)
}
