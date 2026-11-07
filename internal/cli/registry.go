package cli

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Unlimited is a MaxArgs value accepting any number of positional arguments.
const Unlimited = -1

// Group places a top-level command or noun in a section of the overview help.
type Group int

const (
	GroupNone     Group = iota // not listed in a section (e.g. a top-level alias target)
	GroupEveryday              // Everyday: play, install, use, stop
	GroupManage                // Manage an instance
	GroupFind                  // Find things to install
	GroupSystem                // This PC and the launcher
)

// Command is one leaf command. Files register commands from init():
//
//	func init() { cli.Register(&cli.Command{Path: "instance list", ...}) }
type Command struct {
	// Path is the command's words: "version", "launch", "instance list".
	// A two-word path adds a verb to a noun (see Noun); the noun must be
	// registered (the standard nouns are pre-registered in nouns.go).
	Path string
	// Aliases are other names for the last word: "play" for "launch".
	// "ls" (list) and "rm" (remove) are added automatically to noun verbs.
	Aliases []string
	// Summary is one line for command lists, lower case, no final period.
	Summary string
	// Args is the positional synopsis shown after the path: "[<filter>]".
	Args string
	// Description is shown below the synopsis in the command's help.
	Description string
	// Examples are full invocations without the program name.
	Examples []string
	// Flags are the command's own flags; global flags are always accepted.
	Flags []*Flag
	// MinArgs and MaxArgs bound the positional arguments (MaxArgs
	// Unlimited for no bound). Both zero means no positional arguments.
	MinArgs, MaxArgs int
	// Group is the overview section of a top-level command.
	Group Group
	// Hidden commands work but are not listed.
	Hidden bool
	// NoBootstrap skips storage/config initialisation (version, help).
	NoBootstrap bool
	// Run does the work. Return nil, an *Error, ExitCode or any error.
	Run func(c *Ctx) error

	noun *Noun
	name string // last word
}

// Name returns the last word of the command's path.
func (c *Command) Name() string { return c.name }

// FullPath returns the command's words joined by spaces.
func (c *Command) FullPath() string { return c.Path }

// Noun groups verbs: "instance" with "list", "show", ...
type Noun struct {
	Name    string
	Aliases []string // "cf" for "curseforge"
	Summary string   // shown in the noun's help, lower case
	Note    string   // shown after the verb list in the overview ("(LeviLamina)")
	Group   Group
	// Default is the verb used when the word after the noun is not a verb:
	// "instance <name>" runs "instance show <name>".
	Default string
	Hidden  bool

	verbs map[string]*Command // by name and alias
	list  []*Command
}

// TopAlias is a top-level word standing for a longer command path:
// "install" for "instance create".
type TopAlias struct {
	Name   string
	Target string
}

var (
	regMu      sync.Mutex
	pending    []*Command
	nounList   []*Noun
	topAliases []TopAlias
	built      *tree
)

type tree struct {
	top     map[string]*Command // top-level commands by name and alias
	nouns   map[string]*Noun    // by name and alias
	aliases map[string]TopAlias
	cmds    []*Command // every command, registration order
	nounSeq []*Noun    // nouns in registration order
}

// Register adds a command. Call it from init(). Duplicates and unknown
// nouns are reported by the first use of the registry (a panic, caught by
// TestRegistry).
func Register(c *Command) {
	regMu.Lock()
	defer regMu.Unlock()
	if built != nil {
		panic("cli.Register called after the registry was built: register from init()")
	}
	pending = append(pending, c)
}

// RegisterNoun adds a noun that is not in the standard set.
func RegisterNoun(n *Noun) {
	regMu.Lock()
	defer regMu.Unlock()
	if built != nil {
		panic("cli.RegisterNoun called after the registry was built: register from init()")
	}
	nounList = append(nounList, n)
}

// RegisterTopAlias makes a top-level word run another command path.
func RegisterTopAlias(name, target string) {
	regMu.Lock()
	defer regMu.Unlock()
	if built != nil {
		panic("cli.RegisterTopAlias called after the registry was built: register from init()")
	}
	topAliases = append(topAliases, TopAlias{Name: name, Target: target})
}

var autoVerbAliases = map[string]string{"list": "ls", "remove": "rm"}

func registry() *tree {
	regMu.Lock()
	defer regMu.Unlock()
	if built != nil {
		return built
	}
	t, err := buildTree(pending, nounList, topAliases)
	if err != nil {
		panic(err)
	}
	built = t
	return built
}

func buildTree(cmds []*Command, nouns []*Noun, aliases []TopAlias) (*tree, error) {
	t := &tree{top: map[string]*Command{}, nouns: map[string]*Noun{}, aliases: map[string]TopAlias{}}
	taken := func(w string) bool {
		_, a := t.top[w]
		_, b := t.nouns[w]
		_, c := t.aliases[w]
		return a || b || c || w == "debug"
	}
	for _, n := range nouns {
		n.verbs, n.list = map[string]*Command{}, nil
		for _, w := range append([]string{n.Name}, n.Aliases...) {
			w = strings.ToLower(w)
			if taken(w) {
				return nil, fmt.Errorf("cli: noun %q: word %q already registered", n.Name, w)
			}
			t.nouns[w] = n
		}
		t.nounSeq = append(t.nounSeq, n)
	}
	for _, c := range cmds {
		words := strings.Fields(strings.ToLower(c.Path))
		if c.Run == nil {
			return nil, fmt.Errorf("cli: command %q has no Run", c.Path)
		}
		if c.MaxArgs != Unlimited && c.MaxArgs < c.MinArgs {
			return nil, fmt.Errorf("cli: command %q: MaxArgs < MinArgs", c.Path)
		}
		if err := checkFlags(c); err != nil {
			return nil, err
		}
		switch len(words) {
		case 1:
			c.name, c.Path = words[0], words[0]
			for _, w := range append([]string{c.name}, c.Aliases...) {
				w = strings.ToLower(w)
				if taken(w) {
					return nil, fmt.Errorf("cli: command %q: word %q already registered", c.Path, w)
				}
				t.top[w] = c
			}
		case 2:
			n, ok := t.nouns[words[0]]
			if !ok || n.Name != words[0] {
				return nil, fmt.Errorf("cli: command %q: unknown noun %q", c.Path, words[0])
			}
			c.noun, c.name, c.Path = n, words[1], n.Name+" "+words[1]
			names := append([]string{c.name}, c.Aliases...)
			if a, ok := autoVerbAliases[c.name]; ok {
				names = append(names, a)
			}
			for i, w := range names {
				w = strings.ToLower(w)
				if _, dup := n.verbs[w]; dup {
					if i > 0 && autoVerbAliases[c.name] == w {
						continue
					}
					return nil, fmt.Errorf("cli: command %q: verb %q already registered", c.Path, w)
				}
				n.verbs[w] = c
			}
			n.list = append(n.list, c)
		default:
			return nil, fmt.Errorf("cli: command path %q must have one or two words", c.Path)
		}
		t.cmds = append(t.cmds, c)
	}
	for _, a := range aliases {
		w := strings.ToLower(a.Name)
		if taken(w) {
			return nil, fmt.Errorf("cli: top-level alias %q already registered", w)
		}
		t.aliases[w] = a
	}
	for _, n := range nouns {
		sort.SliceStable(n.list, func(i, j int) bool { return verbRank(n.list[i].name) < verbRank(n.list[j].name) })
	}
	return t, nil
}

// verbOrder is the order verbs are listed in, the fixed verb set first.
var verbOrder = []string{
	"list", "show", "search", "versions", "categories", "status", "create", "download", "install", "update",
	"import", "export", "copy", "copy-data", "rename", "set", "get", "unset", "remove", "enable", "disable",
	"register", "unregister", "shortcut", "mirrors", "restore", "login", "logout", "stop", "clean", "path",
}

func verbRank(v string) int {
	for i, w := range verbOrder {
		if w == v {
			return i
		}
	}
	return len(verbOrder)
}

// lookupAlias resolves a top-level alias to its command, nil when the
// target is not registered.
func (t *tree) lookupAlias(word string) (*Command, bool) {
	a, ok := t.aliases[word]
	if !ok {
		return nil, false
	}
	c := t.find(a.Target)
	return c, c != nil
}

// find returns the command at path ("instance list"), or nil.
func (t *tree) find(path string) *Command {
	words := strings.Fields(strings.ToLower(path))
	switch len(words) {
	case 1:
		return t.top[words[0]]
	case 2:
		if n, ok := t.nouns[words[0]]; ok {
			return n.verbs[words[1]]
		}
	}
	return nil
}

// visibleVerbs lists a noun's listed verbs.
func (n *Noun) visibleVerbs() []*Command {
	var out []*Command
	for _, c := range n.list {
		if !c.Hidden {
			out = append(out, c)
		}
	}
	return out
}

// usable reports whether the noun has any command and is shown.
func (n *Noun) usable() bool { return !n.Hidden && len(n.visibleVerbs()) > 0 }

// topWords lists every word valid as the first command word, for
// suggestions.
func (t *tree) topWords() []string {
	var out []string
	for w, c := range t.top {
		if !c.Hidden {
			out = append(out, w)
		}
	}
	for w, n := range t.nouns {
		if n.usable() {
			out = append(out, w)
		}
	}
	for w := range t.aliases {
		if _, ok := t.lookupAlias(w); ok {
			out = append(out, w)
		}
	}
	sort.Strings(out)
	return out
}

// Commands returns every registered command, for tests and documentation.
func Commands() []*Command {
	t := registry()
	return append([]*Command(nil), t.cmds...)
}
