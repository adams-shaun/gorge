// Command forgec fetches the Forge card corpus, compiles it into mtgcore's IR
// cache, and reports how much of it the engine can currently play.
//
// The corpus is GPL-3.0. forgec treats it as a build input: it is fetched into
// a gitignored directory and never redistributed.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd, args := os.Args[1], os.Args[2:]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	dir := fs.String("dir", ".cards", "working directory for the corpus and IR cache")
	ref := fs.String("ref", "master", "Forge git ref to fetch")
	top := fs.Int("top", 25, "how many missing primitives to list in report")
	fs.Parse(args)

	var err error
	switch cmd {
	case "fetch":
		_, err = cards.Fetch(*dir, *ref)
	case "compile":
		err = compile(*dir)
	case "report":
		err = report(*dir, *top)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "forgec:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: forgec fetch|compile|report [-dir .cards] [-ref master] [-top 25]")
	os.Exit(2)
}

func cachePath(dir string) string { return filepath.Join(dir, "ir.gob.gz") }

func compile(dir string) error {
	r, diags, err := cards.CompileDir(cards.CorpusDir(dir))
	if err != nil {
		return err
	}
	for _, d := range diags {
		fmt.Fprintf(os.Stderr, "%s: %s\n", d.Path, d.Msg)
	}
	if err := r.Save(cachePath(dir)); err != nil {
		return err
	}
	fmt.Printf("compiled %d cards, %d diagnostics -> %s\n", len(r.Cards), len(diags), cachePath(dir))
	return nil
}

func report(dir string, top int) error {
	r, err := cards.LoadRegistry(cachePath(dir))
	if err != nil {
		return err
	}
	if l, err := cards.ReadLock(dir); err == nil {
		fmt.Printf("corpus: %s @ %s (%s, %d files)\n", l.Ref, l.Commit, l.License, l.Files)
	}
	// Task 18 wires in the real primitive set; earlier milestones passed an
	// empty map here, which made every card "not playable" but still let the
	// report rank what to build next.
	cv := r.Coverage(effects.Supported())
	fmt.Printf("cards: %d  playable: %d (%.1f%%)\n",
		cv.Cards, cv.Supported, 100*float64(cv.Supported)/float64(cv.Cards))
	fmt.Println("\ntop missing primitives (cards unlocked):")
	for i, m := range cv.TopMissing(top) {
		fmt.Printf("%3d. %-32s %6d\n", i+1, m.Name, m.Cards)
	}
	return nil
}
