package cli

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Command is one node of a command tree.
type Command struct {
	// Name is the word that invokes it (the program name for the root).
	Name string
	// Short is the one-line summary in its parent's command list.
	Short string
	// Long is the help text's body (Short when empty).
	Long string
	// Args is the positional synopsis after "[flags]" in usage ("<file>...").
	Args string
	// Hidden keeps it out of its parent's command list (it still runs).
	Hidden bool
	// Flags defines its flags. It may run more than once per Execute (a
	// config file's sections are checked against fresh flag sets), so it
	// must only define flags: bound variables are reset to their defaults
	// each time, and the last call's values are the ones Run sees.
	// A command's flags are inherited by its subcommands.
	Flags func(fs *FlagSet)
	// Run runs it; nil for a group that only holds subcommands.
	Run func(c *Context, args []string) error
	// ValidArgs checks the positionals before Run (nil accepts any; see
	// NoArgs and ExactArgs).
	ValidArgs func(args []string) error
	// Sub are its subcommands.
	Sub []*Command
	// ConfigSchema (root only) is the "$schema" tag of this program's config
	// file: the name and major of its config contract, for example
	// "dzgorge-config-1". Empty means the generic ConfigSchema.
	ConfigSchema string
}

// schema is the root's config tag.
func schemaOf(root *Command) string {
	if root.ConfigSchema != "" {
		return root.ConfigSchema
	}
	return ConfigSchema
}

// NoArgs refuses any positional argument.
func NoArgs(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument %q (this command takes flags only)", args[0])
	}
	return nil
}

// ExactArgs wants exactly n positionals.
func ExactArgs(n int) func([]string) error {
	return func(args []string) error {
		if len(args) != n {
			return fmt.Errorf("want %d argument(s), got %d", n, len(args))
		}
		return nil
	}
}

// Source is where a flag's value came from.
type Source int

const (
	FromDefault Source = iota
	FromConfig
	FromEnv
	FromFlag
)

func (s Source) String() string {
	return [...]string{"default", "config", "env", "flag"}[s]
}

// Context is what Run receives.
type Context struct {
	// Command is the command running; Path its words from the root.
	Command *Command
	Path    []string
	Stdout  io.Writer
	Stderr  io.Writer
	// ConfigFile is the --config file read ("" for none).
	ConfigFile string
	flags      *FlagSet
	sources    map[string]Source
}

// Source is where the named flag's value came from.
func (c *Context) Source(name string) Source { return c.sources[name] }

// Changed reports whether the named flag was set by any source but its default.
func (c *Context) Changed(name string) bool { return c.sources[name] != FromDefault }

// Flags is the command's flag set (its own, inherited and global flags).
func (c *Context) Flags() *FlagSet { return c.flags }

// ExitError ends Execute with Code, printing Err (when non-nil) to stderr.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit status %d", e.Code)
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error { return e.Err }

// Exit is an error ending Execute with code and no message; nil for 0. It
// lets a command keep an "int exit code" body: return cli.Exit(run()).
func Exit(code int) error {
	if code == 0 {
		return nil
	}
	return &ExitError{Code: code}
}

// UsageError is a misuse of the command line: exit 2 with a pointer to --help.
type UsageError struct{ Msg string }

func (e *UsageError) Error() string { return e.Msg }

// Usagef is a UsageError.
func Usagef(format string, a ...any) error { return &UsageError{Msg: fmt.Sprintf(format, a...)} }

// subNamed is c's subcommand name, or nil.
func (c *Command) subNamed(name string) *Command {
	for _, s := range c.Sub {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// visibleSubs are c's listed subcommands, sorted.
func (c *Command) visibleSubs() []*Command {
	var out []*Command
	for _, s := range c.Sub {
		if !s.Hidden {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func pathWords(path []*Command) []string {
	out := make([]string, len(path))
	for i, c := range path {
		out[i] = c.Name
	}
	return out
}

func pathString(path []*Command) string { return strings.Join(pathWords(path), " ") }

// assignment is one flag occurrence on the command line, resolved later
// against the final flag set by name.
type assignment struct {
	name, value, spelled string
}

// runner is one Execute.
type runner struct {
	root           *Command
	env            map[string]string
	stdout, stderr io.Writer
}

// Execute runs root over argv (without the program name) and returns the
// exit code. env is os.Environ()'s "KEY=VALUE" form.
func Execute(root *Command, argv []string, env []string, stdout, stderr io.Writer) int {
	r := &runner{root: root, env: map[string]string{}, stdout: stdout, stderr: stderr}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			r.env[k] = v
		}
	}
	_, code := r.execute(argv)
	return code
}

// flagSet builds the flag set of the command at path: the globals, then each
// command's flags from the root down.
func (r *runner) flagSet(path []*Command) *FlagSet {
	fs := newFlagSet()
	fs.global = true
	fs.Var((*stringValue)(new(string)), "config", "read flag values from this JSON file (env "+r.configEnv()+")")
	fs.Var((*boolValue)(new(bool)), "print-config", "print the effective configuration as JSON (loadable with --config) and exit")
	fs.Var((*boolValue)(new(bool)), "help", "show this help")
	fs.Shorthand("help", 'h')
	fs.global = false
	for i, c := range path {
		fs.level = i
		fs.envPrefix = envUpper("GORGE_" + strings.Join(pathWords(path[:i+1]), "_"))
		if c.Flags != nil {
			c.Flags(fs)
		}
	}
	for _, c := range path[len(path)-1].Sub {
		if fs.byName[c.Name] != nil && fs.byName[c.Name].level == len(path)-1 {
			panic(fmt.Sprintf("cli: %s: flag --%s has the name of a subcommand (config sections would be ambiguous)", pathString(path), c.Name))
		}
	}
	return fs
}

func (r *runner) configEnv() string { return EnvName([]string{r.root.Name}, "config") }

// execute is Execute's body; it also returns the resolved path (tests).
func (r *runner) execute(argv []string) ([]*Command, int) {
	path := []*Command{r.root}
	fs := r.flagSet(path)
	var assigns []assignment
	var args []string
	helpCmd := false // the help subcommand: the remaining words name the command
	dashdash := false
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if !dashdash && tok == "--" {
			dashdash = true
			continue
		}
		if dashdash || len(tok) < 2 || tok[0] != '-' {
			cur := path[len(path)-1]
			if len(args) == 0 && len(cur.Sub) > 0 && !dashdash {
				if s := cur.subNamed(tok); s != nil {
					path = append(path, s)
					fs = r.flagSet(path)
					continue
				}
				if tok == "help" && cur.subNamed("help") == nil {
					helpCmd = true
					continue
				}
				if cur.Run == nil || helpCmd {
					return path, r.usageFail(path, unknownCommand(cur, tok))
				}
			}
			args = append(args, tok)
			continue
		}
		f, value, hasValue, err := fs.resolve(tok)
		if err != nil {
			return path, r.usageFail(path, err)
		}
		if !hasValue {
			if _, ok := f.Value.(boolFlag); ok {
				value = "true"
			} else {
				if i+1 >= len(argv) {
					return path, r.usageFail(path, fmt.Errorf("flag %s needs a value", tok))
				}
				i++
				value = argv[i]
			}
		}
		assigns = append(assigns, assignment{name: f.Name, value: value, spelled: tok})
	}
	if helpCmd {
		r.help(r.stdout, path)
		return path, 0
	}
	for _, a := range assigns {
		if a.name == "help" {
			if err := fs.byName["help"].Value.Set(a.value); err != nil {
				return path, r.usageFail(path, fmt.Errorf("flag %s: %v", a.spelled, err))
			}
			if fs.byName["help"].Value.String() == "true" {
				r.help(r.stdout, path)
				return path, 0
			}
		}
	}

	// The config file: --config, else the environment.
	cfgPath := r.env[r.configEnv()]
	for _, a := range assigns {
		if a.name == "config" {
			cfgPath = a.value
		}
	}
	var cfg *configFile
	if cfgPath != "" {
		var err error
		if cfg, err = r.loadConfig(cfgPath); err != nil {
			return path, r.usageFail(path, err)
		}
	}

	// The final flag set; then the sources, lowest first.
	fs = r.flagSet(path)
	sources := map[string]Source{}
	stage := func() {
		for _, f := range fs.order {
			if rs, ok := f.Value.(resetter); ok {
				rs.reset()
			}
		}
	}
	if cfg != nil {
		stage()
		if err := cfg.apply(fs, path, sources); err != nil {
			return path, r.usageFail(path, err)
		}
	}
	stage()
	for _, f := range fs.order {
		if f.Env == "" {
			continue
		}
		if v := r.env[f.Env]; v != "" {
			if err := f.Value.Set(v); err != nil {
				return path, r.usageFail(path, fmt.Errorf("environment %s=%q: %v", f.Env, v, err))
			}
			sources[f.Name] = FromEnv
		}
	}
	stage()
	for _, a := range assigns {
		f := fs.byName[a.name]
		if err := f.Value.Set(a.value); err != nil {
			return path, r.usageFail(path, fmt.Errorf("flag %s %q: %v", a.spelled, a.value, err))
		}
		sources[a.name] = FromFlag
	}

	if fs.byName["print-config"].Value.String() == "true" {
		if err := printConfig(r.stdout, fs, path); err != nil {
			fmt.Fprintf(r.stderr, "%s: %v\n", pathString(path), err)
			return path, 1
		}
		return path, 0
	}
	cur := path[len(path)-1]
	if cur.Run == nil {
		if len(path) == 1 {
			fmt.Fprintf(r.stderr, "%s: a command is required\n\n", cur.Name)
		} else {
			fmt.Fprintf(r.stderr, "%s: a subcommand is required\n\n", pathString(path))
		}
		r.help(r.stderr, path)
		return path, 2
	}
	if cur.ValidArgs != nil {
		if err := cur.ValidArgs(args); err != nil {
			return path, r.usageFail(path, err)
		}
	}
	ctx := &Context{Command: cur, Path: pathWords(path), Stdout: r.stdout, Stderr: r.stderr, ConfigFile: cfgPath, flags: fs, sources: sources}
	err := cur.Run(ctx, args)
	if err == nil {
		return path, 0
	}
	var ue *UsageError
	if errors.As(err, &ue) {
		return path, r.usageFail(path, ue)
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		if ee.Err != nil {
			fmt.Fprintf(r.stderr, "%s: %v\n", pathString(path), ee.Err)
		}
		return path, ee.Code
	}
	fmt.Fprintf(r.stderr, "%s: %v\n", pathString(path), err)
	return path, 1
}

// usageFail prints a usage error and returns 2.
func (r *runner) usageFail(path []*Command, err error) int {
	p := pathString(path)
	fmt.Fprintf(r.stderr, "%s: %v\nRun '%s --help' for usage.\n", p, err, p)
	return 2
}

func unknownCommand(c *Command, word string) error {
	var names []string
	for _, s := range c.visibleSubs() {
		names = append(names, s.Name)
	}
	msg := fmt.Sprintf("unknown command %q", word)
	if m := suggest(word, names); m != "" {
		msg += fmt.Sprintf(" (did you mean %q?)", m)
	}
	return errors.New(msg)
}

// resolve finds the flag a token names: --name[=v]; -name[=v] (Go flag's
// single-dash long form); -n[=v] (a shorthand). No clustering.
func (fs *FlagSet) resolve(tok string) (f *Flag, value string, hasValue bool, err error) {
	var name string
	if strings.HasPrefix(tok, "--") {
		name, value, hasValue = strings.Cut(tok[2:], "=")
		f = fs.byName[name]
	} else {
		name, value, hasValue = strings.Cut(tok[1:], "=")
		f = fs.byName[name]
		if f == nil && len(name) == 1 {
			f = fs.byShort[name[0]]
		}
	}
	if f == nil {
		msg := fmt.Sprintf("unknown flag %s", strings.SplitN(tok, "=", 2)[0])
		var names []string
		for _, g := range fs.order {
			names = append(names, g.Name)
		}
		if m := suggest(name, names); m != "" {
			msg += fmt.Sprintf(" (did you mean --%s?)", m)
		}
		return nil, "", false, errors.New(msg)
	}
	return f, value, hasValue, nil
}

// suggest is the candidate closest to word (edit distance at most 2, or a
// prefix of length 3+), or "".
func suggest(word string, candidates []string) string {
	best, bestD := "", 3
	for _, c := range candidates {
		d := editDistance(word, c)
		if len(word) >= 3 && strings.HasPrefix(c, word) && d > 1 {
			d = 1
		}
		if d < bestD || d == bestD && c < best {
			best, bestD = c, d
		}
	}
	if bestD > 2 || best == word {
		return ""
	}
	return best
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
