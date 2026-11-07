package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// ProgramName is how help and hints name the program.
const ProgramName = "LeviLauncher.exe"

// everydayEntry is one line of the Everyday section, shown when its target
// command is registered.
type everydayEntry struct {
	usage, summary, target, note string
}

var everyday = []everydayEntry{
	{"play [instance]", "Start the game and wait until its window is up", "launch", "(alias: launch)"},
	{"install <version|file>", "Download and install Minecraft as a new instance", "instance create", "(= instance create)"},
	{"use [instance]", "Set or show the default instance", "use", ""},
	{"stop [instance|pid]", "Close a running Minecraft", "process stop", "(= process stop)"},
}

var groupTitles = []struct {
	g     Group
	title string
}{
	{GroupManage, "Manage an instance:"},
	{GroupFind, "Find things to install:"},
	{GroupSystem, "This PC and the launcher:"},
}

const overviewHead = `LeviLauncher - install, run and manage Minecraft Bedrock from the command line.

Usage:
  LeviLauncher.exe <command> [arguments] [flags]
  LeviLauncher.exe                  (no arguments: open the launcher window)
`

const overviewTail = `Examples:
  LeviLauncher.exe install latest --loader --use
  LeviLauncher.exe play
  LeviLauncher.exe mod install LiteLDev/LegacyMoney
  LeviLauncher.exe instance list --json

Exit codes: 0 ok, 1 failed, 2 usage, 3 not found, 4 conflict, 5 sign-in/licence,
6 network, 7 partial, 8 needs system change or admin, 9 timeout, 10 cancelled,
130 interrupted.

The program does not wait in cmd.exe and some shells; use
'start /wait LeviLauncher.exe ...' or pipe its output to see results and
%ERRORLEVEL%.

Run 'LeviLauncher.exe help <command>' for details and examples.
`

// writeOverview prints the tiered top-level help.
func writeOverview(w io.Writer, t *tree) {
	io.WriteString(w, overviewHead)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	var lines []string
	for _, e := range everyday {
		if t.find(e.target) == nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s\t%s\t%s", e.usage, e.summary, e.note))
	}
	if len(lines) > 0 {
		fmt.Fprintln(w, "\nEveryday:")
		for _, l := range lines {
			fmt.Fprintln(tw, strings.TrimRight(l, "\t"))
		}
		tw.Flush()
	}
	for _, gt := range groupTitles {
		var rows [][2]string
		for _, n := range t.nounSeq {
			if n.Group != gt.g || !n.usable() {
				continue
			}
			var verbs []string
			for _, c := range n.visibleVerbs() {
				verbs = append(verbs, c.name)
			}
			text := strings.Join(verbs, ", ")
			if n.Note != "" {
				text += "   " + n.Note
			}
			name := n.Name
			if len(n.Aliases) > 0 {
				text += "   (alias: " + strings.Join(n.Aliases, ", ") + ")"
			}
			rows = append(rows, [2]string{name, text})
		}
		for _, c := range t.cmds {
			if c.noun == nil && c.Group == gt.g && !c.Hidden {
				rows = append(rows, [2]string{c.name, capitalize(c.Summary)})
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%s\n", gt.title)
		for _, r := range rows {
			fmt.Fprintf(tw, "  %s\t%s\n", r[0], r[1])
		}
		tw.Flush()
	}
	fmt.Fprintln(w, "\nGlobal flags:")
	writeFlags(w, globalFlags)
	fmt.Fprintln(w)
	io.WriteString(w, overviewTail)
}

// writeNounHelp prints a noun's verbs.
func writeNounHelp(w io.Writer, n *Noun) {
	if n.Summary != "" {
		fmt.Fprintf(w, "%s\n\n", capitalize(n.Summary))
	}
	fmt.Fprintf(w, "Usage:\n  %s %s <command> [arguments] [flags]\n", ProgramName, n.Name)
	if n.Default != "" {
		if c, ok := n.verbs[n.Default]; ok {
			fmt.Fprintf(w, "  %s %s %s   (= %s %s)\n", ProgramName, n.Name, strings.TrimSpace(c.Args), n.Name, c.name)
		}
	}
	fmt.Fprintln(w, "\nCommands:")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, c := range n.visibleVerbs() {
		line := fmt.Sprintf("  %s\t%s", c.name, c.Summary)
		if a := aliasesOf(c); len(a) > 0 {
			line += "  (alias: " + strings.Join(a, ", ") + ")"
		}
		fmt.Fprintln(tw, line)
	}
	tw.Flush()
	if len(n.Aliases) > 0 {
		fmt.Fprintf(w, "\nAlias: %s\n", strings.Join(n.Aliases, ", "))
	}
	fmt.Fprintf(w, "\nRun '%s help %s <command>' for details and examples.\n", ProgramName, n.Name)
}

func aliasesOf(c *Command) []string {
	out := append([]string{}, c.Aliases...)
	if a, ok := autoVerbAliases[c.name]; ok && c.noun != nil {
		out = append(out, a)
	}
	return out
}

// writeCommandHelp prints a command's synopsis and examples first, then the
// description and flags.
func writeCommandHelp(w io.Writer, c *Command) {
	usage := ProgramName + " " + c.Path
	if c.Args != "" {
		usage += " " + c.Args
	}
	if len(c.Flags) > 0 {
		usage += " [flags]"
	}
	fmt.Fprintf(w, "Usage:\n  %s\n", usage)
	if a := aliasesOf(c); len(a) > 0 {
		fmt.Fprintf(w, "  (alias: %s)\n", strings.Join(a, ", "))
	}
	fmt.Fprintf(w, "\n%s\n", capitalize(c.Summary)+".")
	if len(c.Examples) > 0 {
		fmt.Fprintln(w, "\nExamples:")
		for _, e := range c.Examples {
			fmt.Fprintf(w, "  %s %s\n", ProgramName, e)
		}
	}
	if d := strings.TrimSpace(c.Description); d != "" {
		fmt.Fprintf(w, "\n%s\n", d)
	}
	var visible []*Flag
	for _, f := range c.Flags {
		if !f.Hidden {
			visible = append(visible, f)
		}
	}
	if len(visible) > 0 {
		fmt.Fprintln(w, "\nFlags:")
		writeFlags(w, visible)
	}
	fmt.Fprintf(w, "\nGlobal flags: -i <instance>, -p <player>, --json, -q, -v, -y, --no-input (see '%s help')\n", ProgramName)
}

func writeFlags(w io.Writer, flags []*Flag) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, f := range flags {
		short := "    "
		if f.Short != "" {
			short = "-" + f.Short + ", "
		}
		name := "--" + f.Name
		if f.Value != "" {
			name += " " + f.Value
		}
		usage := f.Usage
		if f.Kind == KindStrings {
			usage += " (repeatable)"
		}
		if f.Default != "" {
			usage += " (default " + f.Default + ")"
		}
		fmt.Fprintf(tw, "  %s%s\t%s\n", short, name, usage)
	}
	tw.Flush()
}

func capitalize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// helpFor resolves "help <words...>" and prints the matching help.
func helpFor(w io.Writer, t *tree, words []string) error {
	if len(words) == 0 {
		writeOverview(w, t)
		return nil
	}
	vals := newFlagValues()
	r, err := resolveCommand(t, words, vals)
	if err != nil {
		return err
	}
	// help takes command words only: "help instance lst" names no command,
	// even though "instance lst" alone would reach the default verb.
	if r.viaDefault && len(r.rest) > 0 {
		return mistypedVerb(r.cmd.noun, r.rest[0], true)
	}
	switch {
	case r.cmd != nil:
		writeCommandHelp(w, r.cmd)
	case r.noun != nil:
		writeNounHelp(w, r.noun)
	default:
		writeOverview(w, t)
	}
	return nil
}

func init() {
	Register(&Command{
		Path:        "help",
		Summary:     "help for any command",
		Args:        "[<command>...]",
		Examples:    []string{"help", "help instance", "help instance list"},
		MaxArgs:     Unlimited,
		Group:       GroupSystem,
		NoBootstrap: true,
		Run: func(c *Ctx) error {
			return helpFor(c.Stdout, registry(), c.Args)
		},
	})
}
