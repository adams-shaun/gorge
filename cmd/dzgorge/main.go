// Command dzgorge plays DraftZero's FDN Limited decks on the gorge rules engine.
//
// It lives in DraftZero (gorge/cmd/dzgorge) and is copied into a pinned gorge
// checkout to build (gorge/build.sh), because it uses gorge's internal packages:
// the AlphaZero-style search (internal/azmcts), the pure-Go network
// (internal/policynet) and the game runner (internal/bench).
//
//	dzgorge play     -a <policy> -b <policy> -pairs N -out games.jsonl [-corpus visits.jsonl.gz]
//	dzgorge coverage                                  which pool decks gorge fully supports
//	dzgorge pack     -in visits.jsonl.gz -out DIR     a visit corpus as dzg training shards (gorge/dzg)
//	dzgorge version                                   this build and its contract versions, as JSON
//	dzgorge help [command]                            usage; also -h / --help on any command
//
// The command line is package cli's (docs/cli.md): -name value and --name value
// both work, every flag can also come from GORGE_DZGORGE_<CMD>_<FLAG> or a
// --config JSON file (contract dzgorge-config), and --print-config prints the
// effective configuration.
//
// Every file and wire dzgorge reads or writes is a registered contract
// (package contracts, docs/contracts.md).
//
// A game draws two different decks from the pool's split. With -a != -b each deck
// pair is played twice on the same seed, the policies swapping seats, so each
// policy plays each deck once and goes first once (DraftZero's paired games).
// With -a == -b each pair is played once.
package main

import (
	"os"

	"github.com/adams-shaun/gorge/cli"
	"github.com/adams-shaun/gorge/contracts"
)

func main() {
	os.Exit(cli.Execute(rootCommand(), os.Args[1:], os.Environ(), os.Stdout, os.Stderr))
}

// rootCommand is dzgorge's command tree (package cli: GNU flags, single-dash
// long flags as Go's flag package spells them, GORGE_DZGORGE_<CMD>_<FLAG>
// environment variables and a --config JSON file; docs/cli.md).
func rootCommand() *cli.Command {
	return &cli.Command{
		Name:  "dzgorge",
		Short: "dzgorge plays DraftZero's FDN Limited decks on the gorge rules engine.",
		// The config file's $schema: contract dzgorge-config (docs/contracts.md).
		ConfigSchema: contracts.Format(contracts.DZGorgeConfig, contracts.DZGorgeConfigVersion),
		Sub:          []*cli.Command{playCommand(), coverageCommand(), packCommand(), versionCommand(), gonetCheckCommand()},
	}
}

// execute runs one subcommand in-process with an empty environment (tests).
func execute(sub string, args []string) int {
	return cli.Execute(rootCommand(), append([]string{sub}, args...), nil, os.Stdout, os.Stderr)
}

func runPack(args []string) int { return execute("pack", args) }
