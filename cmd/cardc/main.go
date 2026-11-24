// Command cardc is the card compiler's tool. Subcommands:
//
//	cardc report [-slice fdn|blb|legacy|decks|all] [-data FILE] [-schema FILE]
//
// report prints one TSV line per Enum.Variant a slice uses
// (key, cards, registered, schema), sorted by key, then a totals line.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "compile" {
		return compile(args[1:], stdout, stderr)
	}
	if len(args) == 0 || args[0] != "report" {
		fmt.Fprintln(stderr, "usage: cardc report [-slice fdn|blb|legacy|decks|all] [-data FILE] [-schema FILE]")
		fmt.Fprintln(stderr, "       cardc compile [-slice fdn|blb|legacy|decks] [-data FILE] [-out DIR]")
		return 2
	}
	return report(args[1:], stdout, stderr)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func report(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	slice := fs.String("slice", "fdn", "fdn, blb, legacy, decks or all")
	data := fs.String("data", mtgish.DataPath(), "mtgish.lines.json")
	schemaPath := fs.String("schema", mtgish.SchemaPath(), "oracle_cards.d.ts")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "cardc: "+format+"\n", a...)
		return 1
	}
	if *data == "" {
		return fail("no data file: set -data or GORGE_MTGISH, or run make mtgish-fetch")
	}
	root, err := mtgish.ModuleRoot()
	if err != nil {
		return fail("%v", err)
	}
	all, err := mtgish.LoadAll(*data)
	if err != nil {
		return fail("%v", err)
	}
	schema, err := mtgish.ReadSchema(*schemaPath)
	if err != nil {
		return fail("%v", err)
	}
	cards := all
	if names, err := mtgish.SliceNames(root, *slice); err != nil {
		return fail("%v", err)
	} else if names != nil {
		ix := mtgish.NewIndex(all)
		cards = cards[:0:0]
		seen := map[*mtgish.Card]bool{}
		var missing []string
		for _, n := range names {
			c, ok := ix.Lookup(n)
			if !ok {
				missing = append(missing, n)
			} else if !seen[c] {
				seen[c] = true
				cards = append(cards, c)
			}
		}
		if len(missing) > 0 {
			fmt.Fprintf(stderr, "cardc: %d names of slice %s do not resolve:\n", len(missing), *slice)
			for _, n := range missing {
				fmt.Fprintln(stderr, "  "+n)
			}
			return 1
		}
	}
	census := mtgish.Census(cards)
	registered, notInSchema := 0, 0
	for _, vc := range census {
		inSchema := schema.Has(vc.Key)
		if vc.Registered {
			registered++
		}
		if !inSchema {
			notInSchema++
		}
		fmt.Fprintf(stdout, "%s\t%d\t%s\t%s\n", vc.Key, vc.Cards, yesNo(vc.Registered), yesNo(inSchema))
	}
	fmt.Fprintf(stdout, "total\tslice=%s\tcards=%d\tvariants=%d\tregistered=%d\tnot-in-schema=%d\n",
		*slice, len(cards), len(census), registered, notInSchema)
	if notInSchema > 0 {
		return fail("%d data variants are missing from the schema", notInSchema)
	}
	return 0
}
