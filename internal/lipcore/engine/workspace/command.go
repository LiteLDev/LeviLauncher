package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
)

// commandWaitDelay bounds how long a cancelled script may keep its output
// pipes open (through a process that escaped the kill) before the runner
// stops reading them.
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
// Like CliWrap (CreateNoWindow) the process gets no console window, and
// like CliWrap it returns once the script has exited and its output pipes
// are closed, so it also waits for background processes the script left
// holding them. The script runs as a process tree that is killed as a whole
// when ctx is cancelled (lip had no cancellation; the launcher used to kill
// lipd); the runner then waits at most commandWaitDelay for the pipes.
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

	if err := ctx.Err(); err != nil {
		return err
	}

	// The output is copied here rather than by exec.Cmd: its WaitDelay
	// would also cut off a script that exited normally but left a
	// background process holding the pipes, which CliWrap waits for.
	cmd := exec.Command(path, flag, command)
	cmd.Dir = dir
	cmd.Stdin = nil // os.DevNull
	out, err := newOutputPipes(cmd, stdout, stderr)
	if err != nil {
		return err
	}
	defer out.close()
	arguments := flag + " " + escapeArgument(command)
	tree, err := newProcessTree(cmd, `"`+path+`" `+arguments)
	if err != nil {
		return err
	}
	defer tree.release()

	if err := startCommand(cmd); err != nil {
		return err
	}
	out.start()
	if err := tree.started(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		out.wait(closedChan)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return err
	}

	// stop reports false once the tree is being killed for ctx. kill may
	// still be running then, or start late; the tree makes it a no-op once
	// released, so it never reaches a closed job, nor a reaped group except
	// in the window its wait documents.
	stop := context.AfterFunc(ctx, tree.kill)
	waitErr, copyErr := tree.wait(func() error { return out.wait(ctx.Done()) })
	if !stop() {
		return ctx.Err()
	}

	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) && exitErr.ProcessState != nil && exitErr.ExitCode() >= 0 {
			return &CommandError{
				ProcessName: filepath.Base(path),
				PID:         exitErr.Pid(),
				ExitCode:    exitErr.ExitCode(),
				Command:     shell + " " + arguments,
			}
		}
		return waitErr
	}
	return copyErr
}

// outputPipes copies a script's stdout and stderr to the caller's writers.
type outputPipes struct {
	readers []*os.File
	writers []*os.File // the child's ends, closed once it has started
	dsts    []io.Writer
	done    chan struct{}
	err     error // the first copy error, set before done is closed
}

// newOutputPipes points cmd's stdout and stderr at new pipes. Like exec.Cmd
// it uses one pipe for both when they are the same writer, and discards
// the output of a nil writer.
func newOutputPipes(cmd *exec.Cmd, stdout, stderr io.Writer) (*outputPipes, error) {
	p := &outputPipes{done: make(chan struct{})}
	add := func(w io.Writer) (*os.File, error) {
		if w == nil {
			w = io.Discard
		}
		r, pw, err := os.Pipe()
		if err != nil {
			p.close()
			return nil, err
		}
		p.readers = append(p.readers, r)
		p.writers = append(p.writers, pw)
		p.dsts = append(p.dsts, w)
		return pw, nil
	}
	var err error
	if cmd.Stdout, err = add(stdout); err != nil {
		return nil, err
	}
	if stdout != nil && interfaceEqual(stdout, stderr) {
		cmd.Stderr = cmd.Stdout
	} else if cmd.Stderr, err = add(stderr); err != nil {
		return nil, err
	}
	return p, nil
}

var closedChan = func() chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}()

// interfaceEqual reports a == b, false for uncomparable dynamic types.
func interfaceEqual(a, b any) (eq bool) {
	defer func() { _ = recover() }()
	return a == b
}

// start closes this process's copies of the child's pipe ends, so the
// readers see EOF once every process holding them has exited, and starts
// copying.
func (p *outputPipes) start() {
	for _, w := range p.writers {
		w.Close()
	}
	p.writers = nil
	errs := make(chan error, len(p.readers))
	for i, r := range p.readers {
		go func() {
			_, err := io.Copy(p.dsts[i], r)
			errs <- err
		}()
	}
	go func() {
		for range p.readers {
			if err := <-errs; err != nil && p.err == nil {
				p.err = err
			}
		}
		close(p.done)
	}()
}

// wait waits until the pipes are closed by every process holding them and
// returns the first copy error. Once cancel is closed (never for a nil
// cancel) it waits at most commandWaitDelay more, then stops reading and
// returns nil.
func (p *outputPipes) wait(cancel <-chan struct{}) error {
	select {
	case <-p.done:
		return p.err
	case <-cancel:
	}
	timer := time.NewTimer(commandWaitDelay)
	defer timer.Stop()
	select {
	case <-p.done:
		return p.err
	case <-timer.C:
	}
	// Closing the read ends ends the copies (CancelIoEx on Windows).
	for _, r := range p.readers {
		r.Close()
	}
	<-p.done
	return nil
}

// close releases the pipe ends still open.
func (p *outputPipes) close() {
	for _, f := range p.readers {
		f.Close()
	}
	for _, f := range p.writers {
		f.Close()
	}
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
