package cli

import (
	"bufio"
	"strings"
)

// CanPrompt reports whether the user can be asked: stdin and stderr are
// consoles, and neither --no-input, --json nor a non-empty CI variable is
// set.
func (c *Ctx) CanPrompt() bool {
	if c.G.NoInput || c.G.JSON || strings.TrimSpace(c.Getenv("CI")) != "" {
		return false
	}
	return c.streams.StdinConsole && c.streams.StderrConsole
}

// Confirm asks before a destructive or displacing action. question names
// exactly what will happen ("Remove instance 1.21.50 and its 3 worlds?").
//
//   - --yes: returns nil without asking;
//   - a console: asks "[y/N]"; yes returns nil, anything else an
//     ERR_CANCELED error (exit 10);
//   - no console, --no-input or --json: an ERR_CONFIRMATION_REQUIRED error
//     (exit 2) whose hint names --yes.
func (c *Ctx) Confirm(question string) error {
	return c.ConfirmWith(question, "--yes")
}

// ConfirmWith is Confirm for an action that another flag also answers
// (--overwrite, --skip-existing): that flag is named in the hint instead of
// --yes. The caller checks its own flag before calling; --yes still
// answers.
func (c *Ctx) ConfirmWith(question, answerFlag string) error {
	if c.G.Yes {
		return nil
	}
	question = strings.TrimSpace(question)
	if !c.CanPrompt() {
		return UsageError("%s needs confirmation", strings.TrimSuffix(question, "?")).
			WithHint("add %s to confirm without a prompt", answerFlag).
			withCode("ERR_CONFIRMATION_REQUIRED")
	}
	c.errf("%s [y/N] ", question)
	if c.in == nil {
		c.in = bufio.NewReader(c.Stdin)
	}
	line, _ := c.in.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	}
	return Errorf(ExitCancelled, "ERR_CANCELED", "cancelled")
}

func (e *Error) withCode(code string) *Error {
	e.Code = code
	return e
}
