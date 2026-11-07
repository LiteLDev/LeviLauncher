package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Globals are the global flags of the invocation.
type Globals struct {
	Instance string // -i/--instance
	Player   string // -p/--player
	JSON     bool   // --json
	Quiet    bool   // -q/--quiet
	Verbose  bool   // -v/--verbose
	Yes      bool   // -y/--yes
	NoInput  bool   // --no-input
}

// Streams are the standard streams of an invocation and whether each is an
// interactive console. Tests use buffers and set the console flags.
type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// Console reports, for stdin, stdout and stderr, whether the stream is
	// an interactive console (not redirected).
	StdinConsole, StdoutConsole, StderrConsole bool
	// Getenv reads the environment; nil means os.Getenv.
	Getenv func(string) string
}

func (s *Streams) getenv(k string) string {
	if s.Getenv != nil {
		return s.Getenv(k)
	}
	return os.Getenv(k)
}

// Ctx is what a command's Run receives.
type Ctx struct {
	// Cmd is the running command.
	Cmd *Command
	// Args are the positional arguments, already checked against
	// MinArgs/MaxArgs.
	Args []string
	// G holds the global flags.
	G Globals

	// Stdout receives results only. Stderr receives everything else; prefer
	// Notice, Warn and Verbosef, which keep clear of the progress line.
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

	ctx     context.Context
	errw    io.Writer // raw stderr; written only under errMu
	streams *Streams
	values  *flagValues
	errMu   *sync.Mutex // serialises stderr writes (events arrive on many goroutines)
	prog    *Progress
	in      *bufio.Reader
}

// Context is cancelled on Ctrl-C. Pass it to every blocking backend call.
func (c *Ctx) Context() context.Context { return c.ctx }

// Arg returns positional argument i, or "" when there are fewer.
func (c *Ctx) Arg(i int) string {
	if i < len(c.Args) {
		return c.Args[i]
	}
	return ""
}

func (c *Ctx) flagValue(name string) (any, bool) {
	if c.values == nil {
		return nil, false
	}
	if findFlag(c.Cmd.Flags, name, "") == nil && findFlag(globalFlags, name, "") == nil {
		panic(fmt.Sprintf("cli: command %q reads undeclared flag --%s", c.Cmd.Path, name))
	}
	v, ok := c.values.vals[name]
	return v, ok
}

// Bool returns a boolean flag's value.
func (c *Ctx) Bool(name string) bool { v, _ := c.flagValue(name); b, _ := v.(bool); return b }

// String returns a string flag's value ("" when not given and no default).
func (c *Ctx) String(name string) string {
	v, _ := c.flagValue(name)
	s, _ := v.(string)
	return s
}

// Strings returns a repeatable flag's values in order.
func (c *Ctx) Strings(name string) []string {
	v, _ := c.flagValue(name)
	s, _ := v.([]string)
	return s
}

// Int returns an integer flag's value.
func (c *Ctx) Int(name string) int { v, _ := c.flagValue(name); n, _ := v.(int); return n }

// Duration returns a duration flag's value.
func (c *Ctx) Duration(name string) time.Duration {
	v, _ := c.flagValue(name)
	d, _ := v.(time.Duration)
	return d
}

// Changed reports whether the flag was given on the command line (a
// default does not count; --no-x does).
func (c *Ctx) Changed(name string) bool {
	c.flagValue(name)
	return c.values.set[name]
}

// Getenv reads an environment variable of the invocation.
func (c *Ctx) Getenv(key string) string { return c.streams.getenv(key) }

// StdoutIsConsole reports whether stdout is an interactive console.
func (c *Ctx) StdoutIsConsole() bool { return c.streams.StdoutConsole }

// StderrIsConsole reports whether stderr is an interactive console.
func (c *Ctx) StderrIsConsole() bool { return c.streams.StderrConsole }

// Machine reports whether output is for a program (--json).
func (c *Ctx) Machine() bool { return c.G.JSON }

// errf writes to stderr under the stderr lock, clearing a progress line
// first so the two do not mix.
func (c *Ctx) errf(format string, a ...any) {
	c.errMu.Lock()
	defer c.errMu.Unlock()
	if c.prog != nil {
		c.prog.clearLocked()
	}
	fmt.Fprintf(c.errw, format, a...)
}

// Notice prints an informational line on stderr ("Using instance X ...").
// -q and --json suppress it.
func (c *Ctx) Notice(format string, a ...any) {
	if c.G.Quiet || c.G.JSON {
		return
	}
	c.errf("%s\n", strings.TrimRight(fmt.Sprintf(format, a...), "\n"))
}

// Warn prints "warning: ..." on stderr, also with --json (stdout stays
// clean); -q suppresses it.
func (c *Ctx) Warn(format string, a ...any) {
	if c.G.Quiet {
		return
	}
	c.errf("warning: %s\n", strings.TrimRight(fmt.Sprintf(format, a...), "\n"))
}

// Verbosef prints a detail line on stderr with -v only.
func (c *Ctx) Verbosef(format string, a ...any) {
	if !c.G.Verbose {
		return
	}
	c.errf("%s\n", strings.TrimRight(fmt.Sprintf(format, a...), "\n"))
}

// Printf writes to stdout. Use it only for results.
func (c *Ctx) Printf(format string, a ...any) {
	fmt.Fprintf(c.Stdout, format, a...)
}
