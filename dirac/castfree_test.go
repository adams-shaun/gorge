package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCastFreeIndexSkipsInfeasible: a free cast's mode index is its place
// among the cards gorge's modes list offers (effects/play.go indexes the
// candidates dense after the CR 601.2c filter), so a remembered card whose
// mandatory targets cannot be chosen is not counted (pair 2014: Etali
// exiled an uncastable Refute and a castable Firebrand Archer; before, the
// Archer's index came out 1 while gorge's was 0).
func TestCastFreeIndexSkipsInfeasible(t *testing.T) {
	deck := []cardset.Entry{{Name: "Refute", Count: 1}, {Name: "Firebrand Archer", Count: 1}, {Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	rememberer := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Rememberer", Types: kop.TCreature, Power: 2, Toughness: 2,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigAttacks, Flags: kop.TfSelf},
			Prog: []kop.Instr{
				{Op: kop.OpDigEx, Obj: kop.RefEachPlayer, A: kop.Const(1), B: kop.Const(-1), C: 525},
				{Op: kop.OpCastFree, Obj: kop.RefRemembered, C: 5},
			}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	rem := sc.perm(rememberer, 0)
	refute := sc.obj(id["Refute"], 0)
	sc.s.objs[refute].zone = kop.ZoneLibrary
	sc.s.pl[0].libTop(refute)
	archer := sc.obj(id["Firebrand Archer"], 1)
	sc.s.objs[archer].zone = kop.ZoneLibrary
	sc.s.pl[1].libTop(archer)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(rem), scAny)
	sc.do(t, AkDone, scAny, scAny)
	pass := func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return Action(0), false
	}
	var asked *Decision
	for i := 0; i < 40 && asked == nil; i++ {
		d := sc.s.Step()
		if d.Kind == DkNone {
			break
		}
		if d.Kind == DkChoose && d.Sub == ChGenCastFree {
			asked = &d
			break
		}
		if a, ok := pass(&d); ok {
			sc.s.Apply(a)
			continue
		}
		sc.s.Apply(d.Options[0])
	}
	if asked == nil {
		t.Fatalf("no free-cast ask posed")
	}
	if asked.Player != 0 {
		t.Fatalf("free-cast ask player %d, want 0", asked.Player)
	}
	if asked.N != 2 || asked.Options[0].Kind() != AkDone ||
		asked.Options[1].Kind() != AkChoose || Obj(asked.Options[1].Source()) != archer {
		t.Fatalf("free-cast options %v, want the Archer then done", asked.Options[:asked.N])
	}
	if got := sc.s.CastFreeIndex(archer); got != 0 {
		t.Fatalf("CastFreeIndex(Archer) = %d, want 0 (gorge's dense index)", got)
	}
}

// TestFreeHandCastPlansNoPayment: a spell cast from the hand without paying its
// mana cost (Buster Sword) has nothing to pay, so the payment surface taps nothing
// for it, and one its controller could not pay for still begins. A Gift spell is
// one the planner never prices, so it would be lowered if it were paid for.
func TestFreeHandCastPlansNoPayment(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Starforged Sword", Count: 1}, {Name: "Plains", Count: 20}})
	s := sc.s
	s.cfg |= cfgGorgeLowering
	land := sc.perm(id["Plains"], 0)
	sword := sc.hand(id["Starforged Sword"], 0)
	s.g.freeQ[0], s.g.nfq, s.g.freeP = sword, 1, seatWord(kop.ZoneHand)<<freeZoneShift
	s.g.hold = true
	s.Step()
	if s.bld().e.obj != sword || s.bld().e.flags&sfFree == 0 {
		t.Errorf("building object %d, flags %#x: want the free cast of %d", s.bld().e.obj, s.bld().e.flags, sword)
	}
	if sc.tapped(land) {
		t.Error("the land was tapped for a free cast")
	}
}
