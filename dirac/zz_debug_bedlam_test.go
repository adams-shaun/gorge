package dirac

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Throwaway diagnostic for the Bedlam Reveler lockstep divergence.
func TestZZDebugBedlamPrice(t *testing.T) {
	var out strings.Builder
	pr := func(f string, a ...any) { fmt.Fprintf(&out, f, a...) }
	set := cardsettest.Synth(t, "tail-73")
	deck := []cardset.Entry{
		{Name: "Bedlam Reveler", Count: 1},
		{Name: "Wanderer's Intervention", Count: 1},
		{Name: "Ephara's Dispersal", Count: 1},
		{Name: "Devastate", Count: 1},
		{Name: "Plains", Count: 3},
		{Name: "Mountain", Count: 1},
		{Name: "Shadowblood Ridge", Count: 1},
		{Name: "Untaidake, the Cloud Keeper", Count: 1},
		{Name: "Plains", Count: 30},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var plains CardID
	for c := CardID(0); c < CardID(len(tbl.Cards)); c++ {
		if tbl.Cards[c].Name == "Plains" {
			plains = c
			break
		}
	}
	sc := newScenTableLib(t, tbl, plains, Config{})
	for _, n := range []string{"Plains", "Plains", "Plains", "Mountain", "Shadowblood Ridge", "Untaidake, the Cloud Keeper"} {
		sc.perm(id[n], 0)
	}
	for _, n := range []string{"Wanderer's Intervention", "Ephara's Dispersal", "Devastate"} {
		sc.s.place(sc.obj(id[n], 0), kop.ZoneGraveyard, false)
	}
	br := id["Bedlam Reveler"]
	ho := sc.hand(br, 0)
	cd := &tbl.Cards[br]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		pr("ab[%d] kind=%v static.kind=%v stflags=%x zone=%v A=%d VA=%v VB=%v\n",
			i, a.Kind, a.Static.Kind, a.Static.StFlags, a.Static.Zone, a.Static.A, a.Static.VA, a.Static.VB)
	}
	base := cd.Cost
	adj := sc.s.adjustCostGen(sc.k, 0, br, ho, base)
	pr("base=%v adj=%v\n", base, adj)
	// what the expr evaluates alone
	for i := range tbl.Exprs {
		pr("expr[%d] kind=%v N=%d F=%d\n", i, tbl.Exprs[i].Kind, tbl.Exprs[i].N, tbl.Exprs[i].F)
	}
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatReduceCost && a.Static.StFlags&kop.StDynamic != 0 {
			v := sc.s.evalV(sc.k, a.Static.VA, evalCtx{src: ho, srcSl: NoSlot, ctrl: 0})
			pr("evalV = %d\n", v)
		}
	}
	d := sc.s.Step()
	pr("options (%d): %v\n", d.N, d.Options[:d.N])
	for i := range d.Options[:d.N] {
		a := &d.Options[i]
		if a.Kind() == AkCast && a.Source() == TCode(uint8(ho)) {
			fmt.Println("CAST OFFERED")
			return
		}
	}
	pr("cast NOT offered\n")
	t.Fatal("DEBUG\n" + out.String())
}
