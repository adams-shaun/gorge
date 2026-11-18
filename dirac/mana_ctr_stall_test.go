package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// ctrStallScen: seat 0 holds a counter-cost mana source (a plain "{T}: Add
// {C}" first ability, then "{T}, Remove a +1/+1 counter: Add {G}") that a
// reflected tap-extra carrier reaches (Judge of Currents' shape,
// kop.StTapReflect), a +1/+1 counter on the source, and a {C}{G} instant in
// hand. Every tap the counter-cost chain makes goes through ctrAb1Do or
// ctrGapDo, which call tapExtraStall: the carrier's extra is not added, so the
// source's tap latches StallTapExtra in the search's fork. The cast needs both
// colours, which no one line makes (a line taps the source once), so every
// line fails.
func ctrStallScen(t *testing.T, each bool) (*scen, Obj, Slot) {
	t.Helper()
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}} // a generated table (prepareGen's gate)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand, Ctrl: kop.CtrlYou})
	mine := kop.FilterID(len(tbl.Filters) - 1)
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Ctr Source", Types: kop.TLand, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1},
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap | kop.CostRemoveCtr, Ctr: kop.CtrP1P1, N: 1}, ManaColors: kop.ColG, ManaN: 1},
	}})
	judge := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Judge", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, B: 1, StFlags: kop.StTapReflect, Filter: mine}},
	}})
	if each {
		// a census gap beside the counter-cost source: tapXBoard is false, so
		// scriptOffers reaches scriptDeadEnd's counter-cost search (not the
		// tap-another one)
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Each", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColW | kop.ColU, ManaEach: true, ManaN: 1, AFlags: kop.AfManaEachColor},
		}})
	}
	ember := CardID(len(tbl.Cards))
	cost := kop.ManaCost{}
	cost.Pips[kop.C] = 1
	cost.Pips[kop.G] = 1
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Ember", Types: kop.TInstant, Cost: cost})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	land := sc.perm(src, 0)
	sl := sc.slot(land)
	sc.s.perms[sl].seeCtr(sc.s)
	sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrP1P1), 1)
	sc.perm(judge, 0)
	if each {
		sc.perm(CardID(len(tbl.Cards)-2), 0) // the each source, before the ember card
	}
	emberO := sc.hand(ember, 0)
	if !sc.k.gen.anyTapExtra {
		t.Fatal("precondition: the table has no tap-extra carrier")
	}
	if !sc.s.scriptCtr(sc.k, 0).has(sl) {
		t.Fatal("precondition: the source is not a live counter-cost gap")
	}
	return sc, emberO, sl
}

// TestCtrScriptStallPropagates: the counter-cost search's only gap is the
// Test Ctr Source, whose every chain tap latches StallTapExtra on the node's
// fork (tapExtraStall: the reflected carrier's extra is not added by the
// counter-cost activation). The search reports the stall and declares nothing
// on the real state; scriptOffers declares it where it finally drops the play
// (scriptDeadEnd runs ctrScript on a fork, and unfork discards a declaration
// made there: the ticket's swallow, end to end). The game draws as the
// unmodelled stall instead of silently losing the cast gorge offers
// (TestTapXScriptStallPropagates's mechanism).
func TestCtrScriptStallPropagates(t *testing.T) {
	sc, emberO, sl := ctrStallScen(t, true)
	s, k := sc.s, sc.k
	a := MakeAction(AkCast, TCode(emberO), 0, 0)
	if s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
	}
	if !s.scriptCtr(k, 0).has(sl) {
		t.Fatal("precondition: the source is no live counter-cost gap")
	}
	if !s.scriptGaps(k, 0).any() || s.scriptCtrModelled(k, 0).any() || s.tapXBoard(k, 0) {
		t.Fatalf("precondition: scriptOffers takes the tap-another route (gaps %v ctrModelled %v tapXBoard %v)",
			s.scriptGaps(k, 0).any(), s.scriptCtrModelled(k, 0), s.tapXBoard(k, 0))
	}
	// the search: its only line stalls, the search reports the stall and
	// declares nothing on the real state (the declaration is the drop site's)
	dead, stall := s.scriptDeadEnd(k, a, 0, slotMask{})
	if !dead {
		t.Fatal("the search found a payment path for an unpayable {C}{G} cast")
	}
	if stall != StallTapExtra {
		t.Fatalf("the search reported the failed line as plain no-payment: stall %s, want %s",
			StallName(stall), StallName(StallTapExtra))
	}
	if s.Stalled() {
		t.Fatalf("the search declared the stall itself (want the drop site's): %s", StallName(s.StallReason()))
	}
	// the offer walk: the play is dropped and the latched stall declared on the
	// real state, not silently swallowed
	var d, capSpent Decision
	s.scriptOffers(k, &d, &capSpent, 0)
	if hasOption(&d, a) {
		t.Fatal("the offer walk kept the cast whose only payment path stalls")
	}
	if !s.Stalled() || s.StallReason() != StallTapExtra {
		t.Fatalf("the offer walk dropped the play silently: stalled %v reason %s, want stalled %s",
			s.Stalled(), StallName(s.StallReason()), StallName(StallTapExtra))
	}
}
