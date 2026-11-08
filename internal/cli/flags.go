package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// FlagKind is the value type of a flag.
type FlagKind int

const (
	KindBool     FlagKind = iota // --x, --no-x, --x=true|false
	KindString                   // --x v, --x=v
	KindStrings                  // repeatable: --x a --x b
	KindInt                      // --x 3
	KindDuration                 // --x 90s (Go duration syntax)
)

// Flag declares one flag of a command. Build flags with BoolFlag,
// StringFlag, StringsFlag, IntFlag, DurationFlag and EnumFlag.
type Flag struct {
	Name    string   // long name without dashes: "channel"
	Short   string   // optional single letter: "o"
	Kind    FlagKind //
	Value   string   // placeholder in help: "<dir>"
	Usage   string   // one line, lower case
	Default string   // default value text, shown in help, parsed like input
	Choices []string // allowed values for string kinds (case-insensitive, stored lower case)
	Hidden  bool
}

// BoolFlag declares a boolean flag (--name, --no-name).
func BoolFlag(name, short, usage string) *Flag {
	return &Flag{Name: name, Short: short, Kind: KindBool, Usage: usage}
}

// StringFlag declares a flag taking one value.
func StringFlag(name, short, value, usage string) *Flag {
	return &Flag{Name: name, Short: short, Kind: KindString, Value: value, Usage: usage}
}

// StringsFlag declares a repeatable flag; its values accumulate.
func StringsFlag(name, short, value, usage string) *Flag {
	return &Flag{Name: name, Short: short, Kind: KindStrings, Value: value, Usage: usage}
}

// IntFlag declares an integer flag.
func IntFlag(name, short, value, usage string) *Flag {
	return &Flag{Name: name, Short: short, Kind: KindInt, Value: value, Usage: usage}
}

// DurationFlag declares a duration flag (90s, 3m, 1h30m).
func DurationFlag(name, short, usage string) *Flag {
	return &Flag{Name: name, Short: short, Kind: KindDuration, Value: "<dur>", Usage: usage}
}

// EnumFlag declares a single-value flag restricted to choices.
func EnumFlag(name, short string, choices []string, usage string) *Flag {
	return &Flag{Name: name, Short: short, Kind: KindString, Value: "<" + strings.Join(choices, "|") + ">", Usage: usage, Choices: choices}
}

// EnumsFlag declares a repeatable flag restricted to choices.
func EnumsFlag(name, short string, choices []string, usage string) *Flag {
	return &Flag{Name: name, Short: short, Kind: KindStrings, Value: "<" + strings.Join(choices, "|") + ">", Usage: usage, Choices: choices}
}

// WithDefault sets the flag's default value text.
func (f *Flag) WithDefault(v string) *Flag { f.Default = v; return f }

// takesValue reports whether the flag consumes a value.
func (f *Flag) takesValue() bool { return f.Kind != KindBool }

// globalFlags are accepted before, between and after the command words.
var globalFlags = []*Flag{
	StringFlag("instance", "i", "<name>", "instance to act on (default: $LEVILAUNCHER_INSTANCE, `use`, or the only one)"),
	StringFlag("player", "p", "<id|tag>", "player profile on GDK instances (default: the only one, or yours)"),
	BoolFlag("json", "", "machine-readable output on stdout"),
	BoolFlag("quiet", "q", "only print identifiers; no progress or notices"),
	BoolFlag("verbose", "v", "more detail, and the launcher log on stderr"),
	BoolFlag("yes", "y", "answer yes to confirmations"),
	BoolFlag("no-input", "", "never prompt"),
	BoolFlag("help", "h", "help for a command"),
	BoolFlag("version", "", "print the version"),
}

func findFlag(flags []*Flag, long, short string) *Flag {
	for _, f := range flags {
		if long != "" && f.Name == long {
			return f
		}
		if short != "" && f.Short == short {
			return f
		}
	}
	return nil
}

func checkFlags(c *Command) error {
	seen := map[string]bool{}
	for _, f := range c.Flags {
		if f.Name == "" || strings.HasPrefix(f.Name, "-") || strings.HasPrefix(f.Name, "no-") {
			return fmt.Errorf("cli: command %q: bad flag name %q", c.Path, f.Name)
		}
		if len(f.Short) > 1 {
			return fmt.Errorf("cli: command %q: flag %q: short name must be one letter", c.Path, f.Name)
		}
		if g := findFlag(globalFlags, f.Name, f.Short); g != nil {
			return fmt.Errorf("cli: command %q: flag %q clashes with global flag --%s", c.Path, f.Name, g.Name)
		}
		for _, k := range []string{"--" + f.Name, "-" + f.Short} {
			if k == "-" {
				continue
			}
			if seen[k] {
				return fmt.Errorf("cli: command %q: flag %s declared twice", c.Path, k)
			}
			seen[k] = true
		}
	}
	return nil
}

// flagValues holds parsed flag values by long name.
type flagValues struct {
	vals map[string]any
	set  map[string]bool
}

func newFlagValues() *flagValues {
	return &flagValues{vals: map[string]any{}, set: map[string]bool{}}
}

func (v *flagValues) assign(f *Flag, raw string, explicit bool) error {
	name := "--" + f.Name
	switch f.Kind {
	case KindBool:
		if !explicit {
			v.vals[f.Name] = true
			break
		}
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return UsageError("%s takes no value (or true/false), got %q", name, raw)
		}
		v.vals[f.Name] = b
	case KindString, KindStrings:
		val := raw
		if len(f.Choices) > 0 {
			ok := false
			for _, c := range f.Choices {
				if strings.EqualFold(c, raw) {
					val, ok = c, true
					break
				}
			}
			if !ok {
				return UsageError("invalid value %q for %s", raw, name).WithHint("use one of: %s", strings.Join(f.Choices, ", "))
			}
		}
		if f.Kind == KindStrings {
			prev, _ := v.vals[f.Name].([]string)
			v.vals[f.Name] = append(prev, val)
		} else {
			v.vals[f.Name] = val
		}
	case KindInt:
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return UsageError("%s needs a whole number, got %q", name, raw)
		}
		v.vals[f.Name] = n
	case KindDuration:
		d, err := time.ParseDuration(strings.TrimSpace(raw))
		if err != nil || d < 0 {
			return UsageError("%s needs a duration such as 90s or 3m, got %q", name, raw)
		}
		v.vals[f.Name] = d
	}
	v.set[f.Name] = true
	return nil
}

func (v *flagValues) negate(f *Flag) {
	v.vals[f.Name] = false
	v.set[f.Name] = true
}

// applyDefaults fills unset flags with their declared defaults.
func (v *flagValues) applyDefaults(flags []*Flag) error {
	for _, f := range flags {
		if _, ok := v.vals[f.Name]; ok || f.Default == "" {
			continue
		}
		if err := v.assign(f, f.Default, true); err != nil {
			return err
		}
		v.set[f.Name] = false
	}
	return nil
}

// parser parses GNU-style arguments against a flag set.
type parser struct {
	flags  []*Flag // command flags then globals
	values *flagValues
}

// parse consumes args, assigning flags and returning positionals. With
// stopAtWord it returns at the first positional word (to resolve command
// words), giving its index in rest.
func (p *parser) parse(args []string, stopAtWord bool) (positional []string, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			if stopAtWord {
				return positional, args[i:], nil
			}
			return append(positional, args[i+1:]...), nil, nil
		case strings.HasPrefix(a, "--") && len(a) > 2:
			name, val, hasVal := strings.Cut(a[2:], "=")
			f := findFlag(p.flags, name, "")
			if f == nil && strings.HasPrefix(name, "no-") && !hasVal {
				if nf := findFlag(p.flags, strings.TrimPrefix(name, "no-"), ""); nf != nil && nf.Kind == KindBool {
					p.values.negate(nf)
					continue
				}
			}
			if f == nil {
				return nil, nil, p.unknown("--" + name)
			}
			if f.takesValue() && !hasVal {
				if i+1 >= len(args) {
					return nil, nil, UsageError("--%s needs a value %s", f.Name, f.Value)
				}
				i++
				val, hasVal = args[i], true
			}
			if err := p.values.assign(f, val, hasVal); err != nil {
				return nil, nil, err
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			cluster := a[1:]
			for j := 0; j < len(cluster); j++ {
				f := findFlag(p.flags, "", cluster[j:j+1])
				if f == nil {
					return nil, nil, p.unknown("-" + cluster[j:j+1])
				}
				if !f.takesValue() {
					if err := p.values.assign(f, "", false); err != nil {
						return nil, nil, err
					}
					continue
				}
				val := strings.TrimPrefix(cluster[j+1:], "=")
				if val == "" {
					if i+1 >= len(args) {
						return nil, nil, UsageError("-%s needs a value %s", f.Short, f.Value)
					}
					i++
					val = args[i]
				}
				if err := p.values.assign(f, val, true); err != nil {
					return nil, nil, err
				}
				break
			}
		default:
			if stopAtWord {
				return positional, args[i:], nil
			}
			positional = append(positional, a)
		}
	}
	return positional, nil, nil
}

func (p *parser) unknown(flag string) error {
	e := UsageError("unknown flag %s", flag)
	if strings.EqualFold(strings.TrimLeft(flag, "-/"), "debug") {
		return e.WithHint("--debug opens the launcher window with a log console; on the command line use -v")
	}
	var names []string
	for _, f := range p.flags {
		if !f.Hidden {
			names = append(names, "--"+f.Name)
		}
	}
	if s := suggest(flag, names); s != "" {
		return e.WithHint("did you mean %s?", s)
	}
	return e
}

// suggest returns the candidate closest to word when it is close enough.
func suggest(word string, candidates []string) string {
	best, bestD := "", 1<<30
	w := strings.ToLower(word)
	for _, c := range candidates {
		d := levenshtein(w, strings.ToLower(c))
		if d < bestD {
			best, bestD = c, d
		}
	}
	limit := 2
	if len(w) > 6 {
		limit = 3
	}
	if bestD <= limit {
		return best
	}
	return ""
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
