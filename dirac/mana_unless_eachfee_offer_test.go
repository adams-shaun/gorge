package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// unlessFeeOffer runs an UnlessPaid {W}{U}{B}{R}{G} mana election with the
// named cards on the payer's battlefield, and reports whether the ask offers
// the pay mode: pfNoPay set is decline only.
func unlessFeeOffer(t *testing.T, names []string) (noPay bool, opts int) {
	t.Helper()
	set := cardsettest.Synth(t, "tail-49")
	deck := []cardset.Entry{{Name: "Plains", Count: 20}, {Name: "Island", Count: 4}}
	for _, n := range names {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	prog := []kop.Instr{{Op: kop.OpUnlessPaid, Obj: kop.RefTargetController,
		A: kop.Const(0), B: kop.Const(4681), C: kop.PayMana}} // 4681 = one W U B R G pip
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Counter", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: prog}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	var tgt Obj
	var basin Slot
	hasBasin := false
	for _, n := range names {
		cid, ok := id[n]
		if !ok {
			t.Fatalf("card %q not in the set", n)
		}
		o := sc.perm(cid, 0)
		sc.s.perms[sc.slot(o)].flags &^= pfTapped // the board is untapped at the ask (Power Depot enters tapped)
		if n == "Overflowing Basin" {
			basin, hasBasin = sc.slot(o), true
		}
		if tgt == 0 {
			tgt = o
		}
	}
	// The premise: the fee source is a filter-first mana ability, not a plain
	// census unit, so only the filter payment search can price its fee.
	if c := sc.s.cardAt(basin); hasBasin && (sc.k.gen.filt(c) < 0 || !sc.s.filtFirstOnly(sc.k, basin) || !sc.s.scriptFilt(sc.k, 0).has(basin)) {
		t.Fatalf("precondition: Overflowing Basin is not the filter-first fee gap the test prices (filt %d, filtFirstOnly %v, scriptFilt %v)",
			sc.k.gen.filt(c), sc.s.filtFirstOnly(sc.k, basin), sc.s.scriptFilt(sc.k, 0))
	}
	sc.s.res.e = stackEntry{kind: seSpell, obj: tgt, srcVer: sc.s.objs[tgt].ver, card: src, ctrl: 0, nt: 1,
		tgt: [kop.MaxTargets]TCode{TCode(tgt)}, tver: [kop.MaxTargets]uint8{sc.s.objs[tgt].ver}}
	sc.s.res.pc = 0
	if sc.s.exec(sc.k, &prog[0]) || sc.s.res.ask != raGenPay {
		t.Fatalf("precondition: no pay ask (ask %d)", sc.s.res.ask)
	}
	var d Decision
	sc.s.resolveOptionsGen(sc.k, &d)
	return sc.s.g.pickF&pfNoPay != 0, int(d.N)
}

// TestUnlessEachManaFeeOffer: Overflowing Basin's "{1}, {T}: Add {G}{U}" pays
// an unless cost's {W}{U}{B}{R}{G} when Power Depot's {C} funds its fee, so
// the pay mode must be offered even though the greedy unless window cannot
// find that plan. CR 605.3b: the fee is paid from the mana another source
// floated first, and the source's production covers two of the cost's pips.
func TestUnlessEachManaFeeOffer(t *testing.T) {
	t.Parallel()
	// Precondition: without the fee source the ask is decline only.
	if noPay, opts := unlessFeeOffer(t, []string{"Power Depot", "Lantern of Revealing", "Plains", "Swamp"}); !noPay || opts != 1 {
		t.Fatalf("precondition: the board without Overflowing Basin must be decline only (noPay %v opts %d)", noPay, opts)
	}
	// Without the {C} funder the fee cannot be paid: still decline only.
	if noPay, opts := unlessFeeOffer(t, []string{"Overflowing Basin", "Lantern of Revealing", "Plains", "Swamp"}); !noPay || opts != 1 {
		t.Fatalf("precondition: Overflowing Basin without a fee funder must be decline only (noPay %v opts %d)", noPay, opts)
	}
	// The chain: Power Depot's {C} pays the Basin's {1}, the Basin adds G and
	// U, Lantern adds R, Plains W and Swamp B -- the pay mode is offered.
	if noPay, opts := unlessFeeOffer(t, []string{"Overflowing Basin", "Power Depot", "Lantern of Revealing", "Plains", "Swamp"}); noPay || opts != 2 {
		t.Fatalf("the fee-funded chain must offer pay (noPay %v opts %d)", noPay, opts)
	}
}
