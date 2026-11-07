package cli

import (
	"strings"
)

// Wants reports whether args (os.Args[1:]) are a command-line invocation.
// Otherwise the launcher window handles them, exactly as before the CLI:
//
//   - no arguments open the window;
//   - the legacy window forms (--launch=<name>, debug, -debug, --debug,
//     /debug, --self-update=<v>, --post-update-restart,
//     --webview2-runtime-dir, --uninstall-cleanup...) go to the window;
//   - any other dash or slash argument that is not a CLI global flag goes to
//     the window, so old scripts keep working;
//   - otherwise, after skipping the CLI global flags and their values, a
//     bare word (known command or not) or globals alone run the CLI. An
//     unknown word then fails with exit 2 and a suggestion; a bare word never
//     opens the window.
func Wants(args []string) bool {
	sawGlobal := false
	for i := 0; i < len(args); i++ {
		a := strings.TrimSpace(args[i])
		if a == "" {
			continue
		}
		if IsLegacyGUIArg(a) {
			return false
		}
		if a == "/?" || a == "-?" {
			return true
		}
		if strings.HasPrefix(a, "-") || strings.HasPrefix(a, "/") {
			values, ok := globalArity(a)
			if !ok {
				return false
			}
			i += values
			sawGlobal = true
			continue
		}
		if isDebugArg(a) {
			return false
		}
		return true
	}
	return sawGlobal
}

// isDebugArg matches the window's debug switches (debug, -debug, --debug,
// /debug), case-insensitively.
func isDebugArg(arg string) bool {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "debug", "-debug", "--debug", "/debug":
		return true
	}
	return false
}

// IsLegacyGUIArg reports whether arg is one of the launcher window's own
// switches, which always open (or are handled by) the window.
func IsLegacyGUIArg(arg string) bool {
	a := strings.ToLower(strings.TrimSpace(arg))
	if strings.HasPrefix(a, "-") || strings.HasPrefix(a, "/") {
		if isDebugArg(a) {
			return true
		}
	}
	for _, p := range []string{"--launch=", "--self-update=", "--uninstall-cleanup", "--webview2-runtime-dir"} {
		if strings.HasPrefix(a, p) {
			return true
		}
	}
	return a == "--post-update-restart"
}

// globalArity reports whether a is a CLI global flag (long, short, or a
// cluster of short ones) and how many following arguments it consumes as
// values.
func globalArity(a string) (values int, ok bool) {
	if strings.HasPrefix(a, "/") {
		return 0, false
	}
	if strings.HasPrefix(a, "--") {
		name, _, hasVal := strings.Cut(a[2:], "=")
		f := findFlag(globalFlags, name, "")
		if f == nil && strings.HasPrefix(name, "no-") && !hasVal {
			if nf := findFlag(globalFlags, strings.TrimPrefix(name, "no-"), ""); nf != nil && nf.Kind == KindBool {
				return 0, true
			}
		}
		if f == nil {
			return 0, false
		}
		if f.takesValue() && !hasVal {
			return 1, true
		}
		return 0, true
	}
	cluster := a[1:]
	for j := 0; j < len(cluster); j++ {
		f := findFlag(globalFlags, "", cluster[j:j+1])
		if f == nil {
			return 0, false
		}
		if f.takesValue() {
			if j == len(cluster)-1 {
				return 1, true
			}
			return 0, true // -iName
		}
	}
	return 0, true
}

// resolved is the outcome of matching the command words.
type resolved struct {
	cmd   *Command
	noun  *Noun    // set when only a noun was given (noun help)
	words []string // the words that named the command, as typed
	rest  []string // arguments after the command words
	// viaDefault: cmd is the noun's default verb, reached without naming it
	// ("instance Beta" for "instance show Beta"); rest starts with the word
	// that was not a verb.
	viaDefault bool
}

// resolveCommand reads the command words from args (globals may precede
// and separate them; they are parsed into vals). It returns a usage error
// for unknown words.
func resolveCommand(t *tree, args []string, vals *flagValues) (resolved, error) {
	gp := &parser{flags: globalFlags, values: vals}
	_, rest, err := gp.parse(args, true)
	if err != nil {
		return resolved{}, err
	}
	if len(rest) == 0 || rest[0] == "--" {
		return resolved{rest: tail(rest)}, nil
	}
	word := strings.ToLower(rest[0])
	if word == "/?" || word == "-?" {
		vals.vals["help"], vals.set["help"] = true, true
		return resolved{rest: rest[1:]}, nil
	}
	if c, ok := t.top[word]; ok {
		return resolved{cmd: c, words: rest[:1], rest: rest[1:]}, nil
	}
	if c, ok := t.lookupAlias(word); ok {
		return resolved{cmd: c, words: rest[:1], rest: rest[1:]}, nil
	}
	n, ok := t.nouns[word]
	if !ok || !n.usable() {
		return resolved{}, unknownCommand(t, rest[0], t.topWords())
	}
	_, rest2, err := gp.parse(rest[1:], true)
	if err != nil {
		return resolved{}, err
	}
	if len(rest2) == 0 || rest2[0] == "--" {
		return resolved{noun: n, words: rest[:1], rest: tail(rest2)}, nil
	}
	verb := strings.ToLower(rest2[0])
	if c, ok := n.verbs[verb]; ok && !(c.Hidden && verb != c.name) {
		return resolved{cmd: c, words: []string{rest[0], rest2[0]}, rest: rest2[1:]}, nil
	}
	if n.Default != "" {
		if c, ok := n.verbs[n.Default]; ok {
			return resolved{cmd: c, words: rest[:1], rest: rest2, viaDefault: true}, nil
		}
	}
	var verbs []string
	for _, c := range n.visibleVerbs() {
		verbs = append(verbs, c.name)
	}
	return resolved{}, unknownCommand(t, n.Name+" "+rest2[0], prefixAll(n.Name+" ", verbs))
}

// mistypedVerb returns an unknown-command error for word under noun n when
// word is close to one of its verbs, or always when always is set; nil
// otherwise (word is an argument of the default verb).
func mistypedVerb(n *Noun, word string, always bool) error {
	var verbs []string
	for _, c := range n.visibleVerbs() {
		verbs = append(verbs, c.name)
	}
	e := UsageError("unknown command %q", n.Name+" "+word)
	if s := suggest(word, verbs); s != "" {
		return e.WithHint("did you mean %q?", n.Name+" "+s)
	}
	if always {
		return e.WithHint("run '%s help %s' for its commands", ProgramName, n.Name)
	}
	return nil
}

func tail(a []string) []string {
	if len(a) > 0 && a[0] == "--" {
		return a[1:]
	}
	return a
}

func prefixAll(p string, words []string) []string {
	out := make([]string, len(words))
	for i, w := range words {
		out[i] = p + w
	}
	return out
}

func unknownCommand(t *tree, typed string, candidates []string) error {
	e := UsageError("unknown command %q", typed)
	if s := suggest(typed, candidates); s != "" {
		return e.WithHint("did you mean %q?", s)
	}
	return e.WithHint("run '%s help' for the list of commands", ProgramName)
}
