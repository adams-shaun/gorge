package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"github.com/adams-shaun/gorge/engine/cardc/diff"
	"github.com/adams-shaun/gorge/engine/cardc/gorgebe"
	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
	"github.com/adams-shaun/gorge/engine/cardc/slice"
	"github.com/adams-shaun/gorge/engine/cards"
)

// diffCmd prints one TSV line per difference between each served card and
// Forge's, then one line per family, then a totals line.
func diffCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	sl := fs.String("slice", "proof", "fdn, legacy or proof (fdn then legacy)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "cardc: "+format+"\n", a...)
		return 1
	}
	var slices []string
	switch *sl {
	case "fdn", "legacy":
		slices = []string{*sl}
	case "proof":
		slices = []string{"fdn", "legacy"}
	default:
		return fail("diff -slice must be fdn, legacy or proof, not %q", *sl)
	}
	root, err := mtgish.ModuleRoot()
	if err != nil {
		return fail("%v", err)
	}
	forge, err := cards.OpenCorpus(filepath.Join(root, ".cards"))
	if err != nil {
		return fail("no Forge corpus (run make fetch-cards): %v", err)
	}
	reg, err := slice.Build(root, forge, slices...)
	if errors.Is(err, slice.ErrNoData) {
		return fail("no mtgish data (run make mtgish-fetch)")
	}
	if err != nil {
		return fail("%v", err)
	}
	type tally struct {
		rows  int
		cards map[string]bool
	}
	fam := map[string]*tally{}
	served, differing, total := 0, 0, 0
	for _, e := range reg.Entries() {
		if e.Origin != gorgebe.OriginCardc {
			continue
		}
		served++
		rows := diff.Card(e.Name, e.Card, e.Forge)
		if len(rows) > 0 {
			differing++
		}
		for _, r := range rows {
			total++
			fmt.Fprintf(stdout, "%s\t%d\t%s\t%s\t%s\t%s\t%s\n", r.Card, r.Face, r.Item, r.Field, r.Family, r.Cardc, r.Forge)
			t := fam[r.Family]
			if t == nil {
				t = &tally{cards: map[string]bool{}}
				fam[r.Family] = t
			}
			t.rows++
			t.cards[r.Card] = true
		}
	}
	names := make([]string, 0, len(fam))
	for n := range fam {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(stdout, "family\t%s\trows=%d\tcards=%d\n", n, fam[n].rows, len(fam[n].cards))
	}
	fmt.Fprintf(stdout, "total\tslice=%s\tserved=%d\tclean=%d\tdiffering=%d\trows=%d\n", *sl, served, served-differing, differing, total)
	return 0
}
