package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestCommanderDigTakeToTopKeepsLibrary: a dig that puts its pick back on top
// of the library, with the picked commander answered "let it go", leaves the
// commander in the library list and takes only the unpicked card out of the
// window for the dig's rest placement.
func TestCommanderDigTakeToTopKeepsLibrary(t *testing.T) {
	tbl, cmd := commanderTable(0)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	f := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Test Commander Dig Top",
		Types: kop.TInstant,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{
			Op: kop.OpDigEx, Obj: kop.RefController, A: kop.Const(2), B: kop.Const(-1),
			C: int16(kop.ZoneLibrary) | int16(kop.DigTakeToTop)<<3, F: f,
		}}}},
	})
	card := CardID(len(tbl.Cards) - 1)
	sc := cmdZoneResScen(t, tbl, cmd, card)
	s := sc.s
	land := sc.obj(v.Plains, 0)
	o := s.pl[0].cmd
	s.detach(o)
	s.objs[o].zone = kop.ZoneLibrary
	s.objs[o].ver++
	s.pl[0].libTop(o)
	s.place(land, kop.ZoneLibrary, false) // [land, commander, ...] or the reverse

	s.runProg(sc.k)
	if s.res.ask != raGenCmdZone {
		t.Fatalf("ask %d, want raGenCmdZone", s.res.ask)
	}
	d := s.Step()
	if d.Kind != DkChoose || d.Sub != ChCommanderZone {
		t.Fatalf("decision kind=%d sub=%d, want DkChoose/ChCommanderZone", d.Kind, d.Sub)
	}
	s.Apply(d.Options[1])
	if s.Stalled() {
		t.Fatalf("the answer stalled: %s", StallName(s.StallReason()))
	}
	pl := &s.pl[0]
	lib := pl.lib[pl.libLo:pl.libHi]
	for _, x := range []Obj{o, land} {
		if s.objs[x].zone != kop.ZoneLibrary || !slices.Contains(lib, x) {
			t.Fatalf("object %d: zone %v, in library list %v; want both", x, s.objs[x].zone, slices.Contains(lib, x))
		}
	}
}
