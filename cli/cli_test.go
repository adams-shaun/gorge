package cli

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite cli/testdata goldens")

// upper is a custom Value without JSONValue: config gives it as a string.
type upper string

func (u *upper) Set(s string) error {
	if s == "bad" {
		return errors.New("bad upper")
	}
	*u = upper(strings.ToUpper(s))
	return nil
}
func (u *upper) String() string { return string(*u) }
func (u *upper) Type() string   { return "word" }

// opts are the test tree's bound values.
type opts struct {
	shared string
	games  int
	a      string
	big    int64
	seed   uint64
	tol    float64
	single bool
	tags   []string
	word   upper
	leafX  int
	ran    string
	args   []string
	ctx    *Context
	runErr error
}

func testTree(o *opts) *Command {
	return &Command{
		Name:  "tool",
		Short: "tool is a test command tree.",
		Flags: func(fs *FlagSet) { fs.StringVar(&o.shared, "shared", "s0", "a root flag every subcommand inherits") },
		Sub: []*Command{
			{
				Name:  "run",
				Short: "run some games",
				Long:  "Run plays some games.\n\nIt is the test tree's leaf.",
				Flags: func(fs *FlagSet) {
					fs.IntVar(&o.games, "games", 1, "games to play")
					fs.Shorthand("games", 'g')
					fs.StringVar(&o.a, "a", "bot", "policy A")
					fs.Int64Var(&o.big, "big", 0, "an int64")
					fs.Uint64Var(&o.seed, "seed", 1, "base seed")
					fs.Float64Var(&o.tol, "tol", 1e-3, "tolerance")
					fs.BoolVar(&o.single, "single", false, "one leg")
					fs.StringSliceVar(&o.tags, "tag", nil, "tags (repeatable, comma-separated)")
					o.word = "w"
					fs.Var(&o.word, "word", "an upper-cased word")
				},
				Run: func(c *Context, args []string) error {
					o.ran, o.args, o.ctx = "run", args, c
					return o.runErr
				},
			},
			{
				Name:  "grp",
				Short: "a group",
				Sub: []*Command{{
					Name:      "leaf",
					Short:     "a nested leaf",
					Flags:     func(fs *FlagSet) { fs.IntVar(&o.leafX, "x", 7, "a leaf flag") },
					ValidArgs: NoArgs,
					Run:       func(c *Context, args []string) error { o.ran = "leaf"; return nil },
				}},
			},
			{Name: "secret", Hidden: true, Run: func(c *Context, args []string) error { o.ran = "secret"; return nil }},
		},
	}
}

type result struct {
	code           int
	stdout, stderr string
	o              *opts
}

func run(t *testing.T, env []string, argv ...string) result {
	t.Helper()
	o := &opts{}
	var out, errb bytes.Buffer
	code := Execute(testTree(o), argv, env, &out, &errb)
	return result{code, out.String(), errb.String(), o}
}

func TestParse(t *testing.T) {
	type want struct {
		games  int
		a      string
		single bool
		tags   []string
		args   []string
		big    int64
		seed   uint64
		shared string
	}
	def := want{games: 1, a: "bot", seed: 1, shared: "s0"}
	with := func(f func(*want)) want { w := def; f(&w); return w }
	for _, tc := range []struct {
		argv []string
		want want
	}{
		{[]string{"run"}, def},
		{[]string{"run", "--games=5"}, with(func(w *want) { w.games = 5 })},
		{[]string{"run", "--games", "5"}, with(func(w *want) { w.games = 5 })},
		{[]string{"run", "-g", "5"}, with(func(w *want) { w.games = 5 })},
		{[]string{"run", "-g=5"}, with(func(w *want) { w.games = 5 })},
		{[]string{"run", "-games", "5"}, with(func(w *want) { w.games = 5 })},
		{[]string{"run", "-games=5"}, with(func(w *want) { w.games = 5 })},
		{[]string{"run", "-a", "az:sims=200:autopay"}, with(func(w *want) { w.a = "az:sims=200:autopay" })},
		{[]string{"run", "-a=x:y=z"}, with(func(w *want) { w.a = "x:y=z" })},
		{[]string{"run", "--a", "x"}, with(func(w *want) { w.a = "x" })},
		{[]string{"run", "--single"}, with(func(w *want) { w.single = true })},
		{[]string{"run", "-single"}, with(func(w *want) { w.single = true })},
		{[]string{"run", "--single", "--single=false"}, def},
		{[]string{"run", "--single=1"}, with(func(w *want) { w.single = true })},
		// A bool never takes the next word: "false" is a positional.
		{[]string{"run", "--single", "false"}, with(func(w *want) { w.single = true; w.args = []string{"false"} })},
		{[]string{"run", "p1", "--games", "3", "p2"}, with(func(w *want) { w.games = 3; w.args = []string{"p1", "p2"} })},
		{[]string{"run", "--", "--games", "3"}, with(func(w *want) { w.args = []string{"--games", "3"} })},
		{[]string{"run", "-"}, with(func(w *want) { w.args = []string{"-"} })},
		{[]string{"run", "--tag", "a,b", "--tag", "c"}, with(func(w *want) { w.tags = []string{"a", "b", "c"} })},
		{[]string{"run", "--big", "-5"}, with(func(w *want) { w.big = -5 })},
		{[]string{"run", "--seed", "0x10"}, with(func(w *want) { w.seed = 16 })},
		{[]string{"run", "--games", "2", "--games", "4"}, with(func(w *want) { w.games = 4 })},
		{[]string{"run", "--shared", "z"}, with(func(w *want) { w.shared = "z" })},
		{[]string{"--shared", "z", "run"}, with(func(w *want) { w.shared = "z" })},
		{[]string{"-shared=z", "run", "-g", "2"}, with(func(w *want) { w.shared = "z"; w.games = 2 })},
	} {
		r := run(t, nil, tc.argv...)
		if r.code != 0 || r.o.ran != "run" {
			t.Errorf("%q: exit %d ran %q, stderr %s", tc.argv, r.code, r.o.ran, r.stderr)
			continue
		}
		got := want{r.o.games, r.o.a, r.o.single, r.o.tags, r.o.args, r.o.big, r.o.seed, r.o.shared}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q:\n got %+v\nwant %+v", tc.argv, got, tc.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, tc := range []struct {
		argv []string
		code int
		msg  string
	}{
		{[]string{"run", "--gmaes", "3"}, 2, "unknown flag --gmaes (did you mean --games?)"},
		{[]string{"run", "-gx"}, 2, "unknown flag -gx"}, // no clustering
		{[]string{"run", "--games"}, 2, "flag --games needs a value"},
		{[]string{"run", "--games", "x"}, 2, `flag --games "x": parsing "x": invalid syntax`},
		{[]string{"run", "--single=maybe"}, 2, "want true or false"},
		{[]string{"run", "--seed", "-1"}, 2, `flag --seed "-1"`},
		{[]string{"run", "--word", "bad"}, 2, "bad upper"},
		{[]string{"rnu"}, 2, `unknown command "rnu" (did you mean "run"?)`},
		{[]string{"zzzzzz"}, 2, `unknown command "zzzzzz"` + "\n"},
		{[]string{}, 2, "tool: a command is required"},
		{[]string{"grp"}, 2, "tool grp: a subcommand is required"},
		{[]string{"grp", "leaf", "extra"}, 2, `tool grp leaf: unexpected argument "extra"`},
		{[]string{"--games", "1", "run"}, 2, "unknown flag --games"},
		{[]string{"help", "nope"}, 2, `unknown command "nope"`},
	} {
		r := run(t, nil, tc.argv...)
		if r.code != tc.code || !strings.Contains(r.stderr, tc.msg) {
			t.Errorf("%q: exit %d stderr %q; want exit %d containing %q", tc.argv, r.code, r.stderr, tc.code, tc.msg)
		}
		if tc.code == 2 && !strings.Contains(r.stderr, "--help") && !strings.Contains(r.stderr, "Usage:") {
			t.Errorf("%q: a usage error does not point at --help: %q", tc.argv, r.stderr)
		}
	}
}

func TestRunErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code int
		msg  string
	}{
		{nil, 0, ""},
		{Exit(0), 0, ""},
		{Exit(3), 3, ""},
		{&ExitError{Code: 4, Err: errors.New("boom")}, 4, "tool run: boom\n"},
		{errors.New("plain"), 1, "tool run: plain\n"},
		{Usagef("-out is required"), 2, "tool run: -out is required\nRun 'tool run --help' for usage.\n"},
	} {
		o := &opts{runErr: tc.err}
		var out, errb bytes.Buffer
		code := Execute(testTree(o), []string{"run"}, nil, &out, &errb)
		if code != tc.code || errb.String() != tc.msg {
			t.Errorf("%v: exit %d stderr %q, want %d %q", tc.err, code, errb.String(), tc.code, tc.msg)
		}
	}
}

func TestHiddenRunsAndIsNotListed(t *testing.T) {
	if r := run(t, nil, "secret"); r.code != 0 || r.o.ran != "secret" {
		t.Fatalf("hidden command: exit %d ran %q", r.code, r.o.ran)
	}
	if r := run(t, nil, "--help"); strings.Contains(r.stdout, "secret") {
		t.Fatalf("hidden command listed:\n%s", r.stdout)
	}
	if r := run(t, nil, "grp", "leaf"); r.code != 0 || r.o.ran != "leaf" || r.o.leafX != 7 {
		t.Fatalf("nested leaf: %+v", r)
	}
}

func TestEnvNames(t *testing.T) {
	for _, tc := range []struct {
		path []string
		flag string
		want string
	}{
		{[]string{"dzgorge", "play"}, "games", "GORGE_DZGORGE_PLAY_GAMES"},
		{[]string{"dzgorge", "play"}, "max-turns", "GORGE_DZGORGE_PLAY_MAX_TURNS"},
		{[]string{"dzgorge"}, "config", "GORGE_DZGORGE_CONFIG"},
		{[]string{"tool", "gonet-check"}, "tol", "GORGE_TOOL_GONET_CHECK_TOL"},
	} {
		if got := EnvName(tc.path, tc.flag); got != tc.want {
			t.Errorf("EnvName(%q, %q) = %q, want %q", tc.path, tc.flag, got, tc.want)
		}
	}
	// Each flag's Env is its defining command's: the root's shared flag is
	// GORGE_TOOL_SHARED even under run.
	r := run(t, []string{"GORGE_TOOL_SHARED=e", "GORGE_TOOL_RUN_GAMES=9", "GORGE_TOOL_GRP_LEAF_X=3", "GORGE_TOOL_RUN_SHARED=no"}, "run")
	if r.o.shared != "e" || r.o.games != 9 {
		t.Fatalf("env: shared %q games %d", r.o.shared, r.o.games)
	}
	if r := run(t, []string{"GORGE_TOOL_GRP_LEAF_X=3"}, "grp", "leaf"); r.o.leafX != 3 {
		t.Fatalf("nested env: x %d", r.o.leafX)
	}
	// Empty is unset; a bad value is a usage error naming the variable.
	if r := run(t, []string{"GORGE_TOOL_RUN_GAMES="}, "run"); r.code != 0 || r.o.games != 1 {
		t.Fatalf("empty env: exit %d games %d", r.code, r.o.games)
	}
	if r := run(t, []string{"GORGE_TOOL_RUN_GAMES=lots"}, "run"); r.code != 2 || !strings.Contains(r.stderr, "GORGE_TOOL_RUN_GAMES") {
		t.Fatalf("bad env: exit %d %q", r.code, r.stderr)
	}
	// A slice reads its comma list from the environment.
	if r := run(t, []string{"GORGE_TOOL_RUN_TAG=x,y"}, "run"); !reflect.DeepEqual(r.o.tags, []string{"x", "y"}) {
		t.Fatalf("env slice: %q", r.o.tags)
	}
}

func writeFile(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestPrecedence is the full matrix: for each subset of {config, env, flag}
// that sets --games, the highest source wins and Source names it.
func TestPrecedence(t *testing.T) {
	cfg := writeFile(t, "c.json", `{"run": {"games": 20}}`)
	for mask := 0; mask < 8; mask++ {
		var env, argv []string
		want, src := 1, FromDefault
		argv = append(argv, "run")
		if mask&1 != 0 {
			argv = append(argv, "--config", cfg)
			want, src = 20, FromConfig
		}
		if mask&2 != 0 {
			env = append(env, "GORGE_TOOL_RUN_GAMES=30")
			want, src = 30, FromEnv
		}
		if mask&4 != 0 {
			argv = append(argv, "-g", "40")
			want, src = 40, FromFlag
		}
		r := run(t, env, argv...)
		if r.code != 0 || r.o.games != want || r.o.ctx.Source("games") != src || r.o.ctx.Changed("games") != (src != FromDefault) {
			t.Errorf("mask %03b: exit %d games %d source %v; want %d from %v (%s)", mask, r.code, r.o.games, r.o.ctx.Source("games"), want, src, r.stderr)
		}
	}
	// The config file named by the environment, and --config over it.
	other := writeFile(t, "d.json", `{"run": {"games": 21}}`)
	if r := run(t, []string{"GORGE_TOOL_CONFIG=" + cfg}, "run"); r.o.games != 20 || r.o.ctx.ConfigFile != cfg {
		t.Fatalf("GORGE_TOOL_CONFIG: games %d", r.o.games)
	}
	if r := run(t, []string{"GORGE_TOOL_CONFIG=" + cfg}, "run", "--config", other); r.o.games != 21 {
		t.Fatalf("--config over GORGE_TOOL_CONFIG: games %d", r.o.games)
	}
	// A slice: a higher source replaces the list, it never appends to it.
	tags := writeFile(t, "t.json", `{"run": {"tag": ["a,b", "c"]}}`)
	if r := run(t, nil, "run", "--config", tags); !reflect.DeepEqual(r.o.tags, []string{"a,b", "c"}) {
		t.Fatalf("config slice (elements are not split): %q", r.o.tags)
	}
	if r := run(t, nil, "run", "--config", tags, "--tag", "d", "--tag", "e"); !reflect.DeepEqual(r.o.tags, []string{"d", "e"}) {
		t.Fatalf("flag over config slice: %q", r.o.tags)
	}
	if r := run(t, []string{"GORGE_TOOL_RUN_TAG=x"}, "run", "--config", tags); !reflect.DeepEqual(r.o.tags, []string{"x"}) {
		t.Fatalf("env over config slice: %q", r.o.tags)
	}
}

func TestConfigValues(t *testing.T) {
	cfg := writeFile(t, "c.json", `{
  "$schema": "gorge-cli-config-1",
  "shared": "fromfile",
  "run": {"games": 3, "a": "az:sims=5", "big": -9, "seed": 18446744073709551615, "tol": 0.5,
          "single": true, "tag": ["x"], "word": "hello"},
  "grp": {"leaf": {"x": 11}}
}`)
	r := run(t, nil, "run", "--config", cfg)
	o := r.o
	if r.code != 0 || o.shared != "fromfile" || o.games != 3 || o.a != "az:sims=5" || o.big != -9 || o.seed != 1<<64-1 ||
		o.tol != 0.5 || !o.single || !reflect.DeepEqual(o.tags, []string{"x"}) || o.word != "HELLO" {
		t.Fatalf("exit %d %+v %s", r.code, *o, r.stderr)
	}
	if r := run(t, nil, "grp", "leaf", "--config", cfg); r.o.leafX != 11 || r.o.shared != "fromfile" {
		t.Fatalf("nested section: x %d shared %q", r.o.leafX, r.o.shared)
	}
	// No $schema: a hand-written file is read as the current version.
	if r := run(t, nil, "run", "--config", writeFile(t, "h.json", `{"run": {"games": 2}}`)); r.code != 0 || r.o.games != 2 {
		t.Fatalf("no $schema: exit %d %s", r.code, r.stderr)
	}
}

func TestConfigErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body, msg string
	}{
		{"unknown", `{"run": {"gmaes": 1}}`, `unknown key "run.gmaes" (did you mean "run.games"?)`},
		{"unknown-top", `{"sharde": "x"}`, `unknown key "sharde" (did you mean "shared"?)`},
		{"other-section", `{"grp": {"leaf": {"y": 1}}}`, `unknown key "grp.leaf.y"`},
		{"global", `{"run": {"config": "x"}}`, `unknown key "run.config"`},
		{"parent-flag", `{"run": {"shared": "x"}}`, `key "run.shared": --shared is a flag of "tool"`},
		{"string-for-int", `{"run": {"games": "10"}}`, `key "run.games": want an integer, got a string`},
		{"float-for-int", `{"run": {"games": 1.5}}`, `key "run.games": want an integer, got 1.5`},
		{"negative-uint", `{"run": {"seed": -1}}`, `key "run.seed": want a non-negative integer`},
		{"num-for-bool", `{"run": {"single": 1}}`, `key "run.single": want a bool, got a number`},
		{"array-for-string", `{"run": {"a": ["x"]}}`, `key "run.a": want a string, got an array`},
		{"string-for-slice", `{"run": {"tag": "x"}}`, `key "run.tag": want an array of strings, got a string`},
		{"slice-element", `{"run": {"tag": ["x", 2]}}`, `element 1: want a string, got a number`},
		{"custom-not-string", `{"run": {"word": 3}}`, `key "run.word": want a string (the flag's word text form), got a number`},
		{"custom-set-fails", `{"run": {"word": "bad"}}`, `key "run.word": bad upper`},
		{"section-not-object", `{"run": 3}`, `key "run" (the run command's section): want a JSON object`},
		{"schema-version", `{"$schema": "gorge-cli-config-2"}`, `key "$schema": "gorge-cli-config-2", this build reads "gorge-cli-config-1"`},
		{"schema-type", `{"$schema": 1}`, `key "$schema": 1, this build reads`},
		{"schema-nested", `{"run": {"$schema": "gorge-cli-config-1"}}`, `key "run.$schema": $schema belongs at the top level only`},
		{"duplicate", `{"run": {"games": 1, "games": 2}}`, `duplicate key "games"`},
		{"not-object", `[1]`, `want a JSON object`},
		{"trailing", `{} {}`, `data after the top-level object`},
		{"syntax", `{"run": `, `config `},
	} {
		p := writeFile(t, tc.name+".json", tc.body)
		r := run(t, nil, "run", "--config", p)
		if r.code != 2 || !strings.Contains(r.stderr, tc.msg) || !strings.Contains(r.stderr, p) {
			t.Errorf("%s: exit %d stderr %q; want exit 2 naming the file and %q", tc.name, r.code, r.stderr, tc.msg)
		}
	}
	if r := run(t, nil, "run", "--config", filepath.Join(t.TempDir(), "missing.json")); r.code != 2 || !strings.Contains(r.stderr, "missing.json") {
		t.Fatalf("missing file: exit %d %q", r.code, r.stderr)
	}
}

// TestPrintConfigRoundTrip: --print-config's output, fed back with --config,
// reproduces every value and prints byte-identically.
func TestPrintConfigRoundTrip(t *testing.T) {
	env := []string{"GORGE_TOOL_RUN_TOL=0.25", "GORGE_TOOL_SHARED=env"}
	argv := []string{"run", "-g", "6", "-a", "x<y&z", "--big", "-3", "--seed", "99", "--single", "--tag", "p,q", "--word", "mixed", "--print-config"}
	r1 := run(t, env, argv...)
	if r1.code != 0 || r1.o.ran != "" {
		t.Fatalf("print-config: exit %d ran %q %s", r1.code, r1.o.ran, r1.stderr)
	}
	checkGolden(t, "print_config.json", r1.stdout)
	p := writeFile(t, "rt.json", r1.stdout)
	r2 := run(t, nil, "run", "--config", p, "--print-config")
	if r2.code != 0 || r2.stdout != r1.stdout {
		t.Fatalf("round trip differs:\n%s\nvs\n%s", r2.stdout, r1.stdout)
	}
	r3 := run(t, nil, "run", "--config", p)
	if o := r3.o; o.games != 6 || o.a != "x<y&z" || o.big != -3 || o.seed != 99 || !o.single || o.tol != 0.25 ||
		!reflect.DeepEqual(o.tags, []string{"p", "q"}) || o.word != "MIXED" || o.shared != "env" {
		t.Fatalf("round trip values: %+v", *o)
	}
	// The root's own configuration.
	r4 := run(t, nil, "--print-config")
	if r4.code != 0 || r4.stdout != "{\n  \"$schema\": \"gorge-cli-config-1\",\n  \"shared\": \"s0\"\n}\n" {
		t.Fatalf("root print-config: %d %q", r4.code, r4.stdout)
	}
}

// TestCustomSchema: a root's ConfigSchema is the tag printed and required.
func TestCustomSchema(t *testing.T) {
	o := &opts{}
	root := testTree(o)
	root.ConfigSchema = "tool-config-3"
	var out, errb bytes.Buffer
	if code := Execute(root, []string{"--print-config"}, nil, &out, &errb); code != 0 || !strings.Contains(out.String(), `"$schema": "tool-config-3"`) {
		t.Fatalf("exit %d %q", code, out.String())
	}
	p := writeFile(t, "s.json", `{"$schema": "gorge-cli-config-1"}`)
	errb.Reset()
	if code := Execute(testTree(o), []string{"run", "--config", p}, nil, &out, &errb); code != 0 {
		t.Fatalf("default schema refused: %s", errb.String())
	}
	root = testTree(o)
	root.ConfigSchema = "tool-config-3"
	if code := Execute(root, []string{"run", "--config", p}, nil, &out, &errb); code != 2 || !strings.Contains(errb.String(), `this build reads "tool-config-3"`) {
		t.Fatalf("another program's tag accepted: exit %d %q", code, errb.String())
	}
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (go test ./cli -update writes it)", err)
	}
	if got != string(want) {
		t.Fatalf("%s changed:\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}

func TestHelpGolden(t *testing.T) {
	for _, tc := range []struct {
		name string
		argv []string
	}{
		{"help_root.txt", []string{"--help"}},
		{"help_run.txt", []string{"run", "-h"}},
		{"help_grp.txt", []string{"help", "grp"}},
	} {
		r := run(t, nil, tc.argv...)
		if r.code != 0 || r.stderr != "" {
			t.Fatalf("%q: exit %d stderr %q", tc.argv, r.code, r.stderr)
		}
		checkGolden(t, tc.name, r.stdout)
	}
	// Every spelling of help prints the same text.
	want := run(t, nil, "run", "--help").stdout
	for _, argv := range [][]string{{"run", "-help"}, {"help", "run"}, {"run", "--games", "3", "-h"}} {
		if got := run(t, nil, argv...).stdout; got != want {
			t.Errorf("%q: help differs", argv)
		}
	}
	if r := run(t, nil, "run", "--help=false"); r.code != 0 || r.o.ran != "run" {
		t.Errorf("--help=false: exit %d ran %q", r.code, r.o.ran)
	}
}

func TestDefinitionPanics(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags func(fs *FlagSet)
	}{
		{"dup", func(fs *FlagSet) { fs.Int("n", 0, ""); fs.Int("n", 0, "") }},
		{"global", func(fs *FlagSet) { fs.String("config", "", "") }},
		{"upper", func(fs *FlagSet) { fs.Int("Games", 0, "") }},
		{"short-taken", func(fs *FlagSet) { fs.Int("games", 0, ""); fs.Shorthand("games", 'h') }},
		{"short-undefined", func(fs *FlagSet) { fs.Shorthand("nope", 'n') }},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: no panic", tc.name)
				}
			}()
			Execute(&Command{Name: "x", Flags: tc.flags, Run: func(*Context, []string) error { return nil }}, nil, nil, &bytes.Buffer{}, &bytes.Buffer{})
		}()
	}
}

func TestSuggest(t *testing.T) {
	names := []string{"games", "max-turns", "max-intents", "pairs", "out"}
	for word, want := range map[string]string{
		"gmaes": "games", "game": "games", "max-turn": "max-turns", "pair": "pairs", "zzz": "", "out": "", "max": "max-intents",
	} {
		if got := suggest(word, names); got != want {
			t.Errorf("suggest(%q) = %q, want %q", word, got, want)
		}
	}
}
