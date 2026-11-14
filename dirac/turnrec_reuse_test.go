package dirac

import (
	"math/bits"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestNotWasDamagedOffBattlefield: the negated wasDealtDamageThisTurn word
// reads the per-turn damage record by object, so a creature that left the
// battlefield (the destroyed target of Faller's Faithful, in the graveyard)
// keeps its answer, as gorge's Object.WasDealtDamageThisTurn does, and a
// card with no object holds no record.
func TestNotWasDamagedOffBattlefield(t *testing.T) {
	sc, id := newSetScenTweak(t, "turnrec", []cardset.Entry{{Name: "Faller's Faithful", Count: 1}, {Name: "Plains", Count: 8}}, nil)
	ff := id["Faller's Faithful"]
	var f kop.FilterID
	for i := range sc.k.t.Filters {
		if fl := &sc.k.t.Filters[i]; fl.Pred == kop.PredNotF2 && fl.PredN == int16(bits.TrailingZeros32(uint32(kop.F2WasDamaged))) {
			f = kop.FilterID(i)
		}
	}
	if f == 0 {
		t.Fatal("no !wasDealtDamageThisTurn filter in the table")
	}
	gy := sc.grave(ff, 1, 1)[0]
	if !sc.s.matchCardAt(sc.k, f, ff, 1, 1, gy) {
		t.Fatal("an undamaged card in the graveyard did not match")
	}
	if !sc.s.matchCardAt(sc.k, f, ff, 1, 1, NoObj) {
		t.Fatal("a card with no object holds no record, so it matches")
	}
	sc.s.g.dmgTurn.set(gy)
	if sc.s.matchCardAt(sc.k, f, ff, 1, 1, gy) {
		t.Fatal("a damaged creature's graveyard card matched")
	}
	// the battlefield side answers the same record
	o := sc.perm(ff, 1)
	sl := sc.slot(o)
	if !sc.s.matchPerm(sc.k, f, sl, 1, NoSlot) {
		t.Fatal("an undamaged permanent did not match")
	}
	sc.s.g.dmgTurn.set(o)
	if sc.s.matchPerm(sc.k, f, sl, 1, NoSlot) {
		t.Fatal("a damaged permanent matched")
	}
}

// TestResolvedThisTurnActivated: Count$ResolvedThisTurn on an activated
// ability (Soulbright Seeker's {R} pump) counts each resolution, this one
// included, so the third resolution of the turn adds {R}{R}{R}{R}; an
// activation that fizzles (its only target gone) counts nothing, as gorge
// emits no Resolve event for it.
func TestResolvedThisTurnActivated(t *testing.T) {
	sc, id := newSetScenTweak(t, "turnrec", []cardset.Entry{{Name: "Soulbright Seeker", Count: 2}, {Name: "Plains", Count: 8}}, nil)
	ss := id["Soulbright Seeker"]
	ab := -1
	for j, a := range sc.k.t.Cards[ss].Abilities {
		if a.Kind == kop.AbActivated && a.AFlags&kop.AfResolveCount != 0 {
			ab = j
		}
	}
	if ab < 0 {
		t.Fatal("Soulbright Seeker's activated ability does not count its resolutions")
	}
	s := sc.s
	a, b := sc.perm(ss, 0), sc.perm(ss, 0)
	s.pl[0].pool[kop.R] = 4
	// aim pays R for a's pump aimed at target and leaves it on the stack
	aim := func(target Obj) {
		t.Helper()
		if !sc.activateAb(a, uint8(ab)) {
			t.Fatal("the activation was not offered")
		}
		d := s.Step()
		if d.Kind != DkTarget {
			t.Fatalf("decision kind %d after the activation, want the target ask", d.Kind)
		}
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Target() == TCode(uint8(target)) {
				s.Apply(d.Options[i])
				return
			}
		}
		t.Fatalf("target %d not offered: %v", target, d.Options[:d.N])
	}
	pool := func() int { return int(s.pl[0].pool[kop.R]) }
	step := func(want int, what string) {
		t.Helper()
		sc.resolveAll(t)
		if pool() != want {
			t.Fatalf("pool R = %d after %s, want %d", pool(), what, want)
		}
	}

	aim(a)
	step(3, "the first resolution")
	// the target is gone as the pump resolves: it fizzles, so it counts nothing
	aim(b)
	s.destroy(sc.k, sc.slot(b))
	if sc.onBF(b) {
		t.Fatal("precondition: the second Seeker is still on the battlefield")
	}
	step(2, "the fizzled activation")
	// the next resolution is only the second: no mana (a counted fizzle would
	// make it the third and add RRRR)
	aim(a)
	step(1, "the second resolution")
	// the third adds RRRR to what is left after paying
	aim(a)
	step(0+4, "the third resolution")
	// and a fourth is not the third
	s.pl[0].pool[kop.R] = 1
	aim(a)
	step(0, "the fourth resolution")
}
