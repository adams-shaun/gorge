package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/adams-shaun/gorge/engine/cardc/gorgebe"
	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
	"github.com/adams-shaun/gorge/engine/cards"
)

// compile builds the gorge backend's registry over a slice and prints one TSV
// line per card: name, origin (cardc or forge), reason.
func compile(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("compile", flag.ContinueOnError)
	fs.SetOutput(stderr)
	slice := fs.String("slice", "fdn", "fdn, blb, legacy or decks")
	data := fs.String("data", mtgish.DataPath(), "mtgish.lines.json")
	out := fs.String("out", "", "directory for the served scripts (default .cards/cardc/<ref>/scripts)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "cardc: "+format+"\n", a...)
		return 1
	}
	root, err := mtgish.ModuleRoot()
	if err != nil {
		return fail("%v", err)
	}
	names, err := mtgish.SliceNames(root, *slice)
	if err != nil {
		return fail("%v", err)
	}
	if names == nil {
		return fail("compile needs a slice of names: fdn, blb, legacy or decks")
	}
	forge, err := cards.OpenCorpus(filepath.Join(root, ".cards"))
	if err != nil {
		return fail("no Forge corpus (run make fetch-cards): %v", err)
	}
	if *data == "" {
		return fail("no mtgish data: run make mtgish-fetch")
	}
	if _, err := os.Stat(*data); err != nil {
		return fail("no mtgish data (run make mtgish-fetch): %v", err)
	}
	all, err := mtgish.LoadAll(*data)
	if err != nil {
		return fail("%v", err)
	}
	ix := mtgish.NewIndex(all)
	var compiled []*ir.Card
	var missing []string
	for _, n := range names {
		c, ok := ix.Lookup(n)
		if !ok {
			missing = append(missing, n)
			continue
		}
		compiled = append(compiled, mtgish.Compile(c))
	}
	if len(missing) > 0 {
		fmt.Fprintf(stderr, "cardc: %d names of slice %s do not resolve:\n", len(missing), *slice)
		for _, n := range missing {
			fmt.Fprintln(stderr, "  "+n)
		}
		return 1
	}
	fb, err := gorgebe.ReadFallbacks(filepath.Join(root, "engine", "cardc", "testdata", "fallback"))
	if err != nil {
		return fail("%v", err)
	}
	reg, err := gorgebe.Build(forge, compiled, fb)
	if err != nil {
		return fail("%v", err)
	}
	dir := *out
	if dir == "" {
		lock, err := mtgish.ReadLock(filepath.Join(root, "engine", "cardc", "MTGISH.lock"))
		if err != nil {
			return fail("%v", err)
		}
		dir = filepath.Join(root, ".cards", "cardc", lock.Ref, "scripts")
	}
	if reg.Served() > 0 {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fail("%v", err)
		}
	}
	for _, e := range reg.Entries() {
		origin := "forge"
		if e.Origin == gorgebe.OriginCardc {
			origin = "cardc"
			if err := os.WriteFile(filepath.Join(dir, gorgebe.Stem(e.Name)+".txt"), e.Script, 0o644); err != nil {
				return fail("%v", err)
			}
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", e.Name, origin, e.Reason)
	}
	n := len(reg.Entries())
	fmt.Fprintf(stdout, "total\tslice=%s\tcards=%d\tcardc=%d\tforge=%d\n", *slice, n, reg.Served(), n-reg.Served())
	return 0
}
