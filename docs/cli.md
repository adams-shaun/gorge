# The `cli` package

Package [`cli`](../cli/doc.go) gives a Go command the cobra/viper conventions
(subcommands, GNU flags, environment variables and a config file, with one
precedence order) on the standard library, because gorge's root module has no
third-party requires (`TestRootModuleHasNoRequires`). This page is for someone
building a command on it, in this module or another.

The commands under gorge's `cmd/` still use the standard `flag` package, so the
conventions below don't apply to them yet; each one's `-h` lists its flags. The
examples use a hypothetical command, `mytool`.

## Conventions

| | |
|---|---|
| **Subcommands** | `prog <command> [flags] [args]`. `prog help <command>`, `-h` and `--help` print usage to stdout and exit 0. A group command with no subcommand prints its usage to stderr and exits 2. |
| **Flags** | `--name=value`, `--name value`, `-n value`, `-n=value`. A bool is `--name` or `--name=false` and never takes the next word. `--` ends the flags. Flags may come before, between or after positionals. |
| **Go `flag` spelling** | A single-dash long name (`-name value`, `-name=value`) also works, so a command moved off `flag` keeps every existing invocation. Short flags never cluster: `-abc` is the flag `abc` or an error, never `-a -b -c`. |
| **Inheritance** | A command's flags apply to its subcommands too. |
| **Environment** | `GORGE_<PATH>_<FLAG>`: the path of the command that defines the flag, root first, then the flag name, upper case with `-` as `_`. For example, `mytool play -pairs` is `GORGE_MYTOOL_PLAY_PAIRS`. An empty variable counts as unset. A list flag reads a comma list. |
| **Config file** | `--config <file>` or `GORGE_<ROOT>_CONFIG`. See [Config files](#config-files). |
| **Precedence** | explicit flag > environment > config file > default. A list flag takes its whole value from the highest source that sets it. Sources are never merged. |
| **`--print-config`** | Prints the invoked command's effective configuration (every flag, all sources applied) in the config format, then exits 0. Feeding the output back with `--config` gives the same configuration, which prints identically. |
| **Errors** | An unknown command or flag, a bad value, or a bad config file is a usage error: one line on stderr naming the problem (with "did you mean" when a close name exists), a pointer to `--help`, and exit 2. An error from the command itself exits 1, or the code it chose. |
| **Determinism** | Help, errors and `--print-config` list flags, commands and keys sorted. Nothing reads the clock. |

## Config files

A config file is one JSON object:

```json
{
  "$schema": "mytool-config-1",
  "play": {"pairs": 200, "seed": 78, "a": {"kind": "search", "sims": 200}, "b": "heuristic"},
  "pack": {"shard": 100000}
}
```

- Top-level keys are the root command's own flags. A key that names a
  subcommand holds that subcommand's flags, recursively, so one file can
  configure several subcommands. Inherited flags are set in the section of
  the command that defines them.
- Values are typed per flag: a JSON string, number, bool, or array of strings
  for a list. A flag type that `cli` does not know is a JSON string in the
  flag's text form, for example a duration `"300µs"`. It may also read its
  own JSON (`cli.JSONValue`), as a policy flag such as `play.a` above can.
- An unknown key, a wrong type, a duplicate key, or trailing data is an error
  that names the file and the dotted key (`play.pairz`). Every section is
  checked, not only the invoked command's, so a typo in a section you are not
  running still fails.
- `"$schema"` is optional. When present it must be the program's config tag:
  its contract name and major (`mytool-config-1`), or `gorge-cli-config-1`
  for a program that registers no contract. A file without it is read as the
  current version, so a hand-written file needs no boilerplate.
  `--print-config` always writes the tag.

JSON is the only format, because the standard library has no YAML or TOML
parser.

## Building a command on `cli`

```go
func main() {
	os.Exit(cli.Execute(rootCommand(), os.Args[1:], os.Environ(), os.Stdout, os.Stderr))
}

func rootCommand() *cli.Command {
	return &cli.Command{
		Name: "mytool", Short: "one line about mytool",
		ConfigSchema: "mytool-config-1", // only if the config is a registered contract
		Sub: []*cli.Command{runCommand()},
	}
}

func runCommand() *cli.Command {
	var games int
	var out string
	return &cli.Command{
		Name: "run", Short: "play some games",
		ValidArgs: cli.NoArgs,
		Flags: func(fs *cli.FlagSet) {
			fs.IntVar(&games, "games", 10, "games to play")
			fs.Shorthand("games", 'g')
			fs.StringVar(&out, "out", "", "output file (required)")
		},
		Run: func(c *cli.Context, args []string) error {
			if out == "" {
				return cli.Usagef("--out is required") // exit 2
			}
			return cli.Exit(run(games, out)) // keep an int-returning body
		},
	}
}
```

Rules:

- **`Flags` only defines flags.** It can run more than once per `Execute`,
  because config sections are checked against fresh flag sets. A variable it
  binds is reset to its default on each run, and `Run` sees the last run's
  values. Take defaults from constants or values captured at init, never from
  the variable being bound.
- Flag names are lower case, digits and `-`. A flag may not share a name with
  a subcommand or a global (`config`, `print-config`, `help`).
- Flag types: `String`, `Int`, `Int64`, `Uint64`, `Float64`, `Bool`,
  `StringSlice` (repeatable and comma-separated), and `Var` for any
  `cli.Value`. A `time.Duration` flag is a `Var`, because `cli` does not
  import `time` (`TestTimeIsImportedOnlyByTheHost`).
- Test the parser in process with
  `cli.Execute(root, argv, env, &stdout, &stderr)`. Pass the environment
  explicitly (nil in a test) so the test stays hermetic.
- If anything outside the repo writes config files for the command, register
  its config as a contract: a `ConfigSchema` tag, an example file, and its
  `--print-config` output as goldens.

## Migrating a command

Moving a command from `flag` to `cli` keeps every existing invocation working
when:

1. Every flag keeps its name, type and default. `flag.Duration` becomes a
   `Var` with a small duration `Value`.
2. A test runs the parser over the exact argv shapes the repo's scripts and
   known external callers pass, and checks the effective values through
   `--print-config`.
3. Behaviour changes are named in the commit. Under `flag`, parsing stopped
   at the first positional and ignored the rest. `cli` reads flags after
   positionals, and `cli.NoArgs` refuses a stray positional. This catches
   `-single false`, which under `flag` set `single` and silently dropped every
   flag after it.
