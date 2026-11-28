package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPaymentStrandedPlanStallPropagates: paymentStrandedPlan's dry walk stops
// at a restricted source the payment may spend (Test Owed's plain {C} cannot
// make the {W} the activation owes, its restricted {W} can), so it probes the
// real walk on a fork (windowTapRider). The restriction-kind table is already
// full when the probe reaches Test Courtyard's restricted red-green ability,
// whose activation cannot allocate a batch: windowGapAnyColour latches
// StallRestrSrc on the fork. paymentStrandedPlan declares it on the real state
// instead of reversing the activation as a plain "stranded" outcome (the game
// draws as the unmodelled stall).
func TestPaymentStrandedPlanStallPropagates(t *testing.T) {
	set := cardsettest.Set(t, "town")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the town pool", name)
		}
		return CardID(set.PoolID(i))
	}
	restrF := func() kop.FilterID {
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TInstant, Ctrl: kop.CtrlYou})
		return kop.FilterID(len(tbl.Filters) - 1)
	}
	// Eight filler restricted sources register the restriction-kind table.
	for i := 0; i < 8; i++ {
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: fmt.Sprintf("Test Fill %d", i), Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbMana, AFlags: kop.AfManaNonSpell, Cost: kop.Cost{Flags: kop.CostTap},
				ManaColors: kop.ColC, ManaN: 1, ManaRestrict: restrF()},
		}})
	}
	// The owed source: a plain {C} first ability, then a restricted {W} the
	// payment may spend. The plain unit cannot make the owed {W}, so the dry
	// walk leaves it to the full probe.
	owed := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Owed", Types: kop.TArtifact, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1},
		{Kind: kop.AbMana, AFlags: kop.AfManaNonSpell, Cost: kop.Cost{Flags: kop.CostTap},
			ManaColors: kop.ColW, ManaN: 1, ManaRestrict: restrF()},
	}})
	// The probe's first restricted gap: its red-green ability's kind is past
	// the eight registered ones, so allocating its batch fails.
	courtyard := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Courtyard", Types: kop.TLand, Abilities: []kop.Ability{
		{Kind: kop.AbMana, AFlags: kop.AfManaNonSpell, Cost: kop.Cost{Flags: kop.CostTap},
			ManaColors: kop.ColR | kop.ColG, ManaN: 1, ManaRestrict: restrF()},
	}})
	engine := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Engine", Types: kop.TCreature, Abilities: []kop.Ability{
		{Kind: kop.AbActivated, Cost: kop.Cost{Flags: kop.CostTapX, N: 1,
			Mana: kop.ManaCost{Generic: 4, Pips: [kop.NumManaKinds]uint8{kop.W: 1}}}},
	}})
	sc := newScenTableLib(t, &tbl, id("Forest"), Config{NoElide: true})
	s := sc.s
	owedO := sc.perm(owed, 0)
	sc.perm(courtyard, 0)
	bear := sc.perm(id("Grizzly Bears"), 0)
	eng := sc.perm(engine, 0)
	b := s.bld()
	b.e = stackEntry{kind: seAbility, card: engine, ab: 0, ctrl: 0, obj: eng, srcVer: s.objs[eng].ver, tgt: noTargets}
	b.picks[0], b.pickKs[0], b.pickN = bear, pkTapX, 1
	s.ctl = ctlBuild
	s.pushBuild(sc.k)

	cost := s.buildCost(sc.k)
	if cost.Generic != 4 || cost.Pips[kop.W] != 1 {
		t.Fatalf("precondition: activation cost %+v, want {4}{W}", cost)
	}
	if !s.windowCensusComplete(sc.k, &s.bld().e) {
		t.Fatal("precondition: the window census does not price the sources")
	}
	if !s.committedPicks() {
		t.Fatal("precondition: the tap-X pick is not committed")
	}
	if _, owedMask, ok := s.windowRestrOwed(sc.k, 0, sc.slot(owedO), &cost); !ok || owedMask != 1<<kop.W {
		t.Fatalf("precondition: the owed source is not owed its {W} (ok %v mask %#x)", ok, owedMask)
	}
	if ok, known, restr, _, _ := s.windowDryWalk(sc.k, 0, &cost, false); ok || known || !restr {
		t.Fatalf("precondition: dry walk ok=%v known=%v restr=%v, want false/false/true", ok, known, restr)
	}

	if !s.manaStep(sc.k) {
		t.Fatal("manaStep did not reverse the stranded activation")
	}
	if !s.Stalled() || s.StallReason() != StallRestrSrc {
		t.Fatalf("the strand dropped the probe's stall silently: stalled %v reason %s, want stalled %s",
			s.Stalled(), StallName(s.StallReason()), StallName(StallRestrSrc))
	}
}
