package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCondSplitLuckyGemTapExtraCastOffer: a lucky Condition-split source
// (Gemstone Caverns' gate on) beside a StatTapExtra reflect carrier (Mana
// Flare's "add one mana of any type that land produced", CR 106.12a) prices
// the reflected extra of the pick in every census read, and a {3}{R} cast
// the census funds beside a Swamp is offered: gorge's census folds the extra
// into each production, so the kernel's prefix search must float it too
// (tapDeferFloat floats the extra of the tap's pick, not addExtra's silent
// drop).
func TestCondSplitLuckyGemTapExtraCastOffer(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := CardID(len(tbl.Cards))
	gem := n
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Gemstone", Types: kop.TLand, Abilities: []kop.Ability{{Kind: kop.AbMana,
		Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1, SplitGate: kop.SplitGateCounter, SplitCtr: kop.CtrLuck, SplitCols: kop.ColAny}}})
	flare := n + 1
	fid := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand})
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Flare", Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbStatic,
		Static: kop.Static{Kind: kop.StatTapExtra, StFlags: kop.StTapReflect, B: 1, Filter: fid}}}})
	bog := n + 2
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bog", Types: kop.TLand, Abilities: []kop.Ability{{Kind: kop.AbMana,
		Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: 1 << kop.B, ManaN: 1}}})
	blazer := n + 3
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Blazer", Types: kop.TCreature, Cost: kop.ManaCost{Generic: 3, Pips: [kop.NumManaKinds]uint8{kop.R: 1}}})
	sc := newScenTableLib(t, tbl, bog, Config{NoElide: true})
	if !sc.k.gen.anyCondSplit {
		t.Fatal("precondition: the table has no Condition-split source")
	}
	if !sc.k.gen.anyTapExtra {
		t.Fatal("precondition: the table has no tap-extra carrier")
	}
	gemO := sc.perm(gem, 0)
	sc.s.perms[sc.slot(gemO)].addCtrN(sc.s, int(kop.CtrLuck), 1)
	sc.perm(bog, 0)
	sc.perm(flare, 0)
	blazerO := sc.hand(blazer, 0)
	if !sc.s.planDeferred(sc.k, 0, sc.slot(gemO)) || !sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(gemO)) {
		t.Fatal("precondition: the lucky gemstone is not a planner-deferred prefix gap")
	}
	// the gaps census prices four mana: the Bog's {B} and the extra of its
	// tap (two of mask {B}), the lucky gemstone's census mask ({C} beside the
	// arm's any colour) and the extra of its pick (two of mask {C}|any)
	var a avail
	sc.s.availFor(sc.k, 0, &a)
	if a.total != 2 || a.n != 1 || a.mask[0] != 1<<kop.B || a.cnt[0] != 2 {
		t.Fatalf("the plain census prices %v (units %v), want two {B} beside the gemstone's gap", a.total, a.mask[:a.n])
	}
	var gaps avail
	was := sc.s.g.availX
	sc.s.g.availX = availGaps | availPlain // the gaps walk's census (plainGapsOptions)
	sc.s.availFor(sc.k, 0, &gaps)
	sc.s.g.availX = was
	if gaps.total != 4 || gaps.n != 2 || gaps.mask[0] != 1<<kop.B || gaps.cnt[0] != 2 || gaps.mask[1] != 0x3f || gaps.cnt[1] != 2 {
		t.Fatalf("the gaps census prices %v (units %v/%v), want two {B} and two of the gemstone's census mask", gaps.total, gaps.mask[:gaps.n], gaps.cnt[:gaps.n])
	}
	sc.do(t, AkCast, TCode(blazerO), scAny)
}
