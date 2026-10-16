// Package cli is gorge's command-line layer: cobra/viper conventions built on
// the standard library alone (the root module has no requires; invariant 7).
//
// A program is a tree of Commands. Execute resolves the subcommand path from
// argv, then sets every flag from four sources, highest first:
//
//	explicit flag > environment > config file > default
//
// Flags are GNU style (--name=value, --name value, -n value, -n=value, bool
// --name and --name=false, -- ends the flags, flags may follow positionals).
// A single-dash long name (-name value, -name=value) is accepted too, the
// spelling Go's flag package uses, so a command moved from package flag keeps
// every invocation working. Short flags never cluster: -abc is the long flag
// "abc" or an error, never -a -b -c.
//
// Environment: a flag defined by the command at path "root sub" is read from
// GORGE_ROOT_SUB_<FLAG>, upper case with '-' as '_'
// (GORGE_DZGORGE_PLAY_PAIRS). An empty variable counts as unset.
//
// Config file: --config <path> (or GORGE_<ROOT>_CONFIG) names a JSON object.
// Top-level keys are the root command's flags; a key naming a subcommand
// holds an object of that subcommand's flags, recursively, so one file can
// configure several subcommands. A "$schema" key, when present, must be the
// root's ConfigSchema (the program's config contract and major, for example
// "dzgorge-config-1"; ConfigSchema, "gorge-cli-config-1", when the root sets
// none); a file without it is read as the current version, so a hand-written
// file need not carry it. An unknown key or a value of the wrong JSON type is
// an error naming the file and the key. Every section is checked, not only
// the invoked command's.
//
// --print-config prints the invoked command's effective configuration in that
// same format (with "$schema") and exits 0; feeding it back with --config
// reproduces the run. -h, --help and the help subcommand print usage to
// stdout. A usage error (unknown command or flag, bad value, bad config)
// exits 2; a Run error exits 1 unless it is an ExitError.
//
// Output is deterministic: flags, commands and config keys are always listed
// sorted, and nothing reads the clock.
package cli
