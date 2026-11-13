package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// plotRestrScen: the restrictedsource table plus two plot creatures, Plot W (cast {W}, plot
// {W}) and Plot 1 (cast {W}, plot {1}); every hand and the battlefield empty.
func plotRestrScen(t *testing.T, names ...string) (sc *scen, id map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "restrictedsource")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not in the restrictedsource table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id = map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for i := range tbl.Cards {
		if c := &tbl.Cards[i]; c.Types&kop.TToken != 0 && c.Types&kop.TArtifact != 0 && len(c.Abilities) == 1 && c.Abilities[0].Kind == kop.AbMana && c.Abilities[0].ManaRestrict != 0 {
			id["Powerstone Token"] = CardID(i)
		}
	}
	w := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.W: 1}}
	for _, pc := range []struct {
		name string
		plot kop.ManaCost
	}{{"Plot W", w}, {"Plot 1", kop.ManaCost{Generic: 1}}} {
		id[pc.name] = CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: pc.name, Types: kop.TCreature, Power: 1, Toughness: 1, Cost: w, HasPlot: true, PlotCost: pc.plot})
	}
	if len(names) > 1 {
		if _, ok := id["Powerstone Token"]; !ok {
			t.Fatal("no Powerstone token in the table")
		}
	}
	return newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true}), id
}

func plotOffered(d *Decision, o Obj) bool {
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkPlot && a.Source() == TCode(uint8(o)) {
			return true
		}
	}
	return false
}

// TestPlotRestrictedManaClass (gorge; CR 116.2, 106.6):
// the plot special action is not a spell payment. A Spell.Creature
// restricted source (a Village) or restricted mana floating (State.rb) neither
// offers it nor is spent by it; a Powerstone (a non-spell restriction) does
// both.
func TestPlotRestrictedManaClass(t *testing.T) {
	// a Village alone: the creature is castable on its restricted {W}, its plot is not
	sc, id := plotRestrScen(t, "Lupinflower Village")
	sc.perm(id["Lupinflower Village"], 0)
	o := sc.hand(id["Plot W"], 0)
	d := sc.s.Step()
	if !castOffered(&d, o) || plotOffered(&d, o) {
		t.Fatalf("(a) Village only: cast offered %v, plot offered %v; want the cast, not the plot", castOffered(&d, o), plotOffered(&d, o))
	}

	// Spell.Creature mana floating and nothing else: plot is not offered
	sc, id = plotRestrScen(t, "Lupinflower Village")
	sc.perm(id["Lupinflower Village"], 0)
	o = sc.hand(id["Plot W"], 0)
	s, k := sc.s, sc.k
	f := sc.k.t.Cards[id["Lupinflower Village"]].Abilities
	var rf kop.FilterID
	for i := range f {
		if f[i].Kind == kop.AbMana && f[i].ManaRestrict != 0 {
			rf = f[i].ManaRestrict
		}
	}
	for sl := s.ctrlM[0]; sl.any(); sl = sl.rest() {
		s.perms[sl.first()].flags |= pfTapped // the Village is spent: only the floating mana remains
	}
	if !s.addRestr(k, 0, rf, int(kop.W), 1) {
		t.Fatal("(b) restricted mana did not float")
	}
	d = s.Step()
	if plotOffered(&d, o) {
		t.Fatalf("(b) Spell.Creature mana floating: plot offered %v", d.Options[:d.N])
	}

	// a Powerstone: a nonartifact card's plot is offered and paid
	sc, id = plotRestrScen(t, "Swamp", "Thran Spider")
	ps := sc.perm(id["Powerstone Token"], 0)
	o = sc.hand(id["Plot 1"], 0)
	d = sc.s.Step()
	if !plotOffered(&d, o) {
		t.Fatalf("(c) Powerstone: plot not offered: %v", d.Options[:d.N])
	}
	sc.do(t, AkPlot, TCode(o), scAny)
	if sc.s.objs[o].zone != kop.ZoneExile || !sc.tapped(ps) {
		t.Fatalf("(c) Powerstone plot: zone %d, Powerstone tapped %v; want exile and tapped", sc.s.objs[o].zone, sc.tapped(ps))
	}

	// restricted mana from a Powerstone's restriction floating: the plot is
	// funded by it and spends it
	sc, id = plotRestrScen(t, "Swamp", "Thran Spider")
	s, k = sc.s, sc.k
	o = sc.hand(id["Plot 1"], 0)
	pf := k.t.Cards[id["Powerstone Token"]].Abilities[0].ManaRestrict
	if pf == 0 || !slices.Contains(k.gen.restrF[:], pf) {
		t.Fatalf("(d) the Powerstone's restriction %d is not a registered one %v", pf, k.gen.restrF)
	}
	if !s.addRestr(k, 0, pf, int(kop.C), 1) {
		t.Fatal("(d) restricted mana did not float")
	}
	d = s.Step()
	if !plotOffered(&d, o) {
		t.Fatalf("(d) Powerstone mana floating: plot not offered: %v", d.Options[:d.N])
	}
	sc.do(t, AkPlot, TCode(o), scAny)
	if s.objs[o].zone != kop.ZoneExile || s.restrN(0) != 0 {
		t.Fatalf("(d) zone %d, restricted %v; want exile and the restricted mana spent", s.objs[o].zone, s.rb[0])
	}
}
