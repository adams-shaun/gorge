package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAbilityStrandCountsRestrictedManaOnce: manaStep's activated-ability
// branch shortens the cost by the floating restricted mana it admits
// (restrActNow/restrShortAdm) so the planner taps only for what that mana
// leaves, but paymentStranded's dry walk discounts the same batches itself
// (windowSpellSettled). Handed the already shortened cost the walk counts the
// mana twice: an activation costing {2}{U} with one admitted restricted {U}
// floating needs two more from the plain pool, while the double discount
// settles on one and clears a payment the planner cannot fund (CR 733.1
// reverses it, and gorge's census counts the restricted mana once).
func TestAbilityStrandCountsRestrictedManaOnce(t *testing.T) {
	set := cardsettest.Set(t, "town")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	castOrAct := kop.FilterID(len(tbl.Filters) - 1)
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the town pool", name)
		}
		return CardID(set.PoolID(i))
	}
	spring := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Restricted Spring", Types: kop.TArtifact, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColU, ManaN: 1,
			ManaRestrict: castOrAct, ManaRestrictAb: castOrAct},
	}})
	engine := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Engine", Types: kop.TCreature, Abilities: []kop.Ability{
		{Kind: kop.AbActivated, Cost: kop.Cost{Flags: kop.CostTapX, N: 1,
			Mana: kop.ManaCost{Generic: 2, Pips: [kop.NumManaKinds]uint8{kop.U: 1}}}},
	}})
	sc := newScenTableLib(t, &tbl, id("Forest"), Config{NoElide: true})
	s := sc.s

	// The Spring's {U} is already floating restricted, as an earlier tap left it.
	springSl := sc.slot(sc.perm(spring, 0))
	c := s.cardAt(springSl)
	rab := &sc.k.t.Cards[c].Abilities[sc.k.gen.restr(c)]
	if !s.floatRestrAb(sc.k, 0, springSl, rab, int(kop.U), 1) {
		t.Fatal("precondition: the Spring's {U} did not float restricted")
	}
	s.perms[springSl].flags |= pfTapped // the tap that produced it
	if s.restrN(0) != 1 || s.restrOf(0, int(kop.U)) != 1 {
		t.Fatalf("precondition: restricted %v, want one {U}", s.rb[0])
	}

	// Exactly one usable source, and it makes only one mana.
	forest := sc.slot(sc.perm(id("Forest"), 0))
	// A committed tap-X pick: the activation's cost taps another creature.
	bear := sc.perm(id("Grizzly Bears"), 0)
	eng := sc.perm(engine, 0)
	b := s.bld()
	b.e = stackEntry{kind: seAbility, card: engine, ab: 0, ctrl: 0, obj: eng, srcVer: s.objs[eng].ver, tgt: noTargets}
	b.picks[0], b.pickKs[0], b.pickN = bear, pkTapX, 1
	s.ctl = ctlBuild
	s.pushBuild(sc.k)

	want := kop.ManaCost{Generic: 2, Pips: [kop.NumManaKinds]uint8{kop.U: 1}}
	cost := s.buildCost(sc.k)
	if cost != want {
		t.Fatalf("precondition: activation cost %+v, want %+v", cost, want)
	}
	m := s.restrActNow(sc.k, 0)
	if m == 0 {
		t.Fatal("precondition: the floating restricted {U} is not admitted for the activation")
	}
	if short := s.restrShortAdm(0, m, cost); short.Generic != 2 || short.Pips[kop.U] != 0 {
		t.Fatalf("precondition: the admitted batch leaves %+v, want {2}", short)
	}
	if !s.windowCensusComplete(sc.k, &s.bld().e) {
		t.Fatal("precondition: the window census does not price the sources")
	}
	if !s.committedPicks() {
		t.Fatal("precondition: the tap-X pick is not committed")
	}
	// The dry walk on the full cost needs the two generic the restricted {U}
	// leaves: one Forest is short.
	if ok, known, _ := s.windowDryRestr(sc.k, 0, &cost, false); ok || !known {
		t.Fatalf("precondition: dry walk on the full cost ok=%v known=%v, want false/true", ok, known)
	}

	if !s.manaStep(sc.k) {
		t.Fatal("manaStep did not reverse the activation: the dry walk counted the floating restricted {U} twice and cleared a payment the planner cannot fund")
	}
	if s.perms[forest].flags&pfTapped != 0 {
		t.Fatal("the reversed activation kept the Forest tapped")
	}
	if s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("pool %v after the reversal, want empty", s.pl[0].pool)
	}
}
