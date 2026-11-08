package install

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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

// CommandRunner runs a hook command (ICommandRunner.Run): command with the
// shell of the OS in the working directory dir, its standard output and
// standard error both written to output. A command that cannot be started
// or exits with a non-zero code is a *CommandFailedError; when ctx is
// cancelled the runner returns ctx.Err().
type CommandRunner func(ctx context.Context, dir, command string, output io.Writer) error

// DefaultCommandRunner runs commands like lip's CommandRunner
// (src/Lip.Core/Infrastructure/CommandRunner.cs), which uses CliWrap:
// Cli.Wrap("cmd.exe").WithArguments("/s /c \"" + command + "\"") on Windows
// (Cli.Wrap("sh").WithArguments(["-c", command]) elsewhere), stdin from the
// null device, the working directory dir, the inherited environment with
// nothing added, and stdout and stderr merged into one target (lip writes it
// to its own stderr). cmd.exe does not parse its command line with the
// MSVCRT rules CliWrap escapes with, so on Windows the command goes verbatim
// (lip 67ba662); /s makes cmd strip only the outer quotes added here. The
// process command line is built exactly like .NET's Process does:
// `"<resolved cmd.exe path>" /s /c "<command>"`.
//
// Like CliWrap (CreateNoWindow) the process gets no console window, and
// like CliWrap it returns once the script has exited and its output pipes
// are closed, so it also waits for background processes the script left
// holding them. The script runs as a process tree that is killed as a whole
// when ctx is cancelled (lip has no cancellation of hook commands; the
// launcher must be able to stop them); the runner then waits at most
// commandWaitDelay for the pipes.
func DefaultCommandRunner(ctx context.Context, dir, command string, output io.Writer) error {
	shell, flag := "sh", "-c"
	if runtime.GOOS == "windows" {
		shell, flag = "cmd.exe", "/c"
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if output == nil {
		output = io.Discard
	}
	startFailed := func(err error) error {
		return &CommandFailedError{
			Msg:     fmt.Sprintf("Could not start %s in '%s' to run the command.", shell, dir),
			Command: command,
			Dir:     dir,
			Err:     err,
		}
	}

	path, err := exec.LookPath(shell)
	if err != nil {
		return startFailed(err)
	}

	// The output is copied here rather than by exec.Cmd: its WaitDelay
	// would also cut off a script that exited normally but left a
	// background process holding the pipes, which CliWrap waits for.
	cmd := exec.Command(path, flag, command)
	cmd.Dir = dir
	cmd.Stdin = nil // os.DevNull
	out, err := newOutputPipes(cmd, output, output)
	if err != nil {
		return startFailed(err)
	}
	defer out.close()
	arguments := flag + " " + escapeArgument(command)
	if runtime.GOOS == "windows" {
		arguments = `/s /c "` + command + `"`
	}
	tree, err := newProcessTree(cmd, `"`+path+`" `+arguments)
	if err != nil {
		return startFailed(err)
	}
	defer tree.release()

	if err := startCommand(cmd); err != nil {
		return startFailed(err)
	}
	out.start()
	if err := tree.started(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		out.wait(closedChan)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return startFailed(err)
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
		if errors.As(waitErr, &exitErr) && exitErr.ProcessState != nil {
			code, ok := exitCode(exitErr)
			if !ok {
				return startFailed(waitErr)
			}
			return &CommandFailedError{
				Msg: fmt.Sprintf("The command exited with code %d. Its output is shown above"+
					" and in the trace of the full report.", code),
				Command:  command,
				Dir:      dir,
				ExitCode: &code,
			}
		}
		return startFailed(waitErr)
	}
	if copyErr != nil {
		return startFailed(copyErr)
	}
	return nil
}

// CommandFailedError is CommandFailedException: a hook command that could
// not be started or exited with a non-zero code. Error() is lip's message;
// Command, Dir and ExitCode are the details lip adds to the full report.
type CommandFailedError struct {
	Msg      string
	Command  string
	Dir      string
	ExitCode *int // nil when the command could not be started
	Err      error
}

func (e *CommandFailedError) Error() string { return e.Msg }
func (e *CommandFailedError) Unwrap() error { return e.Err }

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
