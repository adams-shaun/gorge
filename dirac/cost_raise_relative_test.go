package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRaiseCostPerTargetCost: a card-own StRaisePerTarget raise adds its
// pips once per target beyond the first of the cast being built (Officious
// Interrogation: {W}{U} each), counting a multi-pick extra like a slot.
func TestRaiseCostPerTargetCost(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	spell := CardID(len(tbl.Cards))
	wu := kop.ManaCost{}
	wu.Pips[kop.W], wu.Pips[kop.U] = 1, 1
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Interrogation", Types: kop.TInstant, Cost: wu, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatIncreaseCost, Zone: kop.ZoneCommand,
			StFlags: kop.StRaisePerTarget, B: 1<<(kop.ReduceColorBits*int(kop.W)) | 1<<(kop.ReduceColorBits*int(kop.U))}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	o := sc.hand(spell, 0)
	sc.s.ctl = ctlBuild
	defer func() { *sc.s.bld() = build{}; sc.s.ctl = 0 }()
	price := func(setup func(e *stackEntry)) kop.ManaCost {
		*sc.s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: spell, ctrl: 0}}
		e := &sc.s.bld().e
		e.tgt = noTargets
		setup(e)
		return sc.s.adjustCostGen(sc.k, 0, spell, o, wu)
	}
	double := kop.ManaCost{}
	double.Pips[kop.W], double.Pips[kop.U] = 2, 2
	cases := []struct {
		name  string
		setup func(e *stackEntry)
		want  kop.ManaCost
	}{
		{"no target", func(e *stackEntry) {}, wu},
		{"one target", func(e *stackEntry) { e.nt, e.tgt[0] = 1, uint8(PlayerTarget0) }, wu},
		{"two slots", func(e *stackEntry) { e.nt, e.tgt[0], e.tgt[1] = 2, uint8(PlayerTarget0), uint8(PlayerTarget0+1) }, double},
		{"slot plus extra", func(e *stackEntry) {
			e.nt, e.tgt[0] = 1, uint8(PlayerTarget0)
			x := sc.s.xtMut(e)
			x.n, x.code[0] = 1, uint8(PlayerTarget0+1)
		}, double},
	}
	for _, c := range cases {
		if got := price(c.setup); got != c.want {
			t.Errorf("%s: costs %+v, want %+v", c.name, got, c.want)
		}
	}
	if price(cases[1].setup) == price(cases[2].setup) {
		t.Fatal("one and two targets cost the same: the test cannot tell")
	}
}
