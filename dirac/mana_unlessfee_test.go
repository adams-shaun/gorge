package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// unlessFeeScen runs an UnlessPaid {3} mana election (Mana Leak's shape, the
// target's controller the payer) with the named cards on the payer's
// battlefield, and reports whether the ask offers the pay mode: pfNoPay set
// is decline only.
func unlessFeeScen(t *testing.T, names []string) (noPay bool, opts int) {
	t.Helper()
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{{Name: "Plains", Count: 20},
		{Name: "Apprentice Wizard", Count: 4}, {Name: "Island", Count: 4},
		{Name: "Gilanra, Caller of Wirewood", Count: 4}, {Name: "Harvester Druid", Count: 4}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	prog := []kop.Instr{{Op: kop.OpUnlessPaid, Obj: kop.RefTargetController, A: kop.Const(3), C: kop.PayMana}}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Counter", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: prog}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	var tgt Obj
	for _, n := range names {
		o := sc.perm(id[n], 0)
		if tgt == 0 {
			tgt = o
		}
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

// CR 605.3b with CR 601.2g: a mana source whose activation carries a literal
// mana fee (Apprentice Wizard's "{U}, {T}: Add {C}{C}{C}") is payable inside
// an unless window when the fee is payable from the floating mana an earlier
// same-window activation produced: tap the Island for {U} first, then the
// wizard's fee takes it and its {C}{C}{C} covers the {3}. The unless window
// walk must be entered for such a source, or the pay mode is offered decline
// only where gorge's unless census prices it (kfuzz pair 25 of dr-mbc-baab:
// gorge offered pay, the kernel decline only, decision 345).
func TestUnlessFeeGapOpensTheWindowWalk(t *testing.T) {
	if noPay, opts := unlessFeeScen(t, []string{"Island"}); !noPay || opts != 1 {
		t.Fatalf("precondition: one {U} short of {3} must be decline only (noPay %v opts %d)", noPay, opts)
	}
	// The plain chain: the Island's {U} funds the wizard's fee, its {C}{C}{C}
	// pays the {3}.
	if noPay, opts := unlessFeeScen(t, []string{"Island", "Apprentice Wizard"}); noPay || opts != 2 {
		t.Fatalf("the wizard's fee-funded chain must offer pay (noPay %v opts %d)", noPay, opts)
	}
	// kfuzz pair 25's board: a restricted source beside the chain must not
	// hide the fee gap.
	if noPay, opts := unlessFeeScen(t, []string{"Island", "Apprentice Wizard", "Harvester Druid"}); noPay || opts != 2 {
		t.Fatalf("a restricted source beside the chain must not hide the fee gap (noPay %v opts %d)", noPay, opts)
	}
	// kfuzz pair 193's board, which already offered pay through the deferred
	// source's route: the fee disjunct must not change its verdict.
	if noPay, opts := unlessFeeScen(t, []string{"Island", "Apprentice Wizard", "Gilanra, Caller of Wirewood"}); noPay || opts != 2 {
		t.Fatalf("the deferred source's board must still offer pay (noPay %v opts %d)", noPay, opts)
	}
}
