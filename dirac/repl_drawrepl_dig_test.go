package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The library-look Draw replacement (Tomorrow, Azami's Familiar; Underrealm
// Lich): the body is one choose-one Dig, "look at the top three, put one in
// hand and the rest in the graveyard instead". Its choose is asked inside the
// body, so the draw site must wait on it (repl_drawprogram.go: the ask leaves
// coldDraw live and the resumed body clears it).

// digBodyHost is a ReplDrawProgram host whose body is that Dig.
func digBodyHost(tb *spTable) CardID {
	return addCard(tb, mayProg(0, kop.Instr{Op: kop.OpDigEx, Obj: kop.RefTriggerPlayer,
		A: kop.Const(3), B: kop.Const(1), C: int16(kop.ZoneHand) | int16(kop.ZoneGraveyard)<<3}))
}

// digTop3 returns seat 0's top three library cards and fails unless they are
// three distinct objects (the choice must be between different cards).
func digTop3(t *testing.T, s *State) (top [3]Obj) {
	t.Helper()
	pl := &s.pl[0]
	if pl.libLen() < 8 {
		t.Fatalf("precondition: library = %d, want at least 8", pl.libLen())
	}
	copy(top[:], pl.lib[pl.libLo:pl.libLo+3])
	if top[0] == top[1] || top[1] == top[2] || top[0] == top[2] {
		t.Fatalf("precondition: the top three %v are not distinct", top)
	}
	return top
}

// digAsk fails unless the pending decision is seat 0's choose among three.
func digAsk(t *testing.T, sc *scen) Decision {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Player != 0 || int(d.N) != 3 {
		t.Fatalf("decision kind %d sub %d player %d with %d options (stalled %d, res.ask %d, nask %d), want seat 0's choose among 3", d.Kind, d.Sub, d.Player, d.N, sc.s.g.stalled, sc.s.res.ask, sc.s.g.nask)
	}
	return d
}

// replDrawDigScenarios runs as TestReplDrawProgramScenarios' "choose-one Dig
// body" case.
func replDrawDigScenarios(t *testing.T) {
	t.Run("the turn draw asks the body's choose and the answer clears the draw", func(t *testing.T) {
		tb := newSPTable()
		host := digBodyHost(tb)
		sc := tb.scen(t)
		if !sc.onBF(sc.perm(host, 0)) {
			t.Fatal("precondition: the replacement host is not on the battlefield")
		}
		s := sc.s
		hand, grave, lib := s.pl[0].handN, s.pl[0].graveN, s.pl[0].libLen()
		top := digTop3(t, s)
		toDrawStep(sc)
		d := digAsk(t, sc)
		if ds := s.drawState(); len(ds) == 0 || !ds[0].active {
			t.Fatal("the body's ask is pending but coldDraw is not live")
		}
		s.Apply(d.Options[1])
		picked := 0
		for _, o := range top {
			switch s.objs[o].zone {
			case kop.ZoneHand:
				picked++
			case kop.ZoneGraveyard:
			default:
				t.Fatalf("a top card is in zone %d, want hand or graveyard", s.objs[o].zone)
			}
		}
		if picked != 1 || s.pl[0].handN != hand+1 || s.pl[0].graveN != grave+2 || s.pl[0].libLen() != lib-3 {
			t.Fatalf("picked %d, hand %d (was %d), graveyard %d (was %d), library %d (was %d): want one in hand, two milled",
				picked, s.pl[0].handN, hand, s.pl[0].graveN, grave, s.pl[0].libLen(), lib)
		}
		if ds := s.drawState(); len(ds) > 0 && (ds[0].active || ds[0].may) {
			t.Fatal("coldDraw is still live after the body finished")
		}
		if s.g.stalled != 0 {
			t.Fatalf("stalled %d: the answered body stalled", s.g.stalled)
		}
		if d := s.Step(); d.Kind != DkPriority || s.g.drawFirst {
			t.Fatalf("decision kind %d (drawFirst %v): want the draw step's priority after the body", d.Kind, s.g.drawFirst)
		}
	})
	t.Run("a resolving two-card draw stalls rather than skip the body", func(t *testing.T) {
		// The spell's OpDraw lends the body its resCtx, which cannot carry
		// the body's pc past the ask: the kernel declares the gap.
		tb := newSPTable()
		host := digBodyHost(tb)
		spell := addCard(tb, kop.CardDef{Name: "Test Draw Two Then Life", Types: kop.TSorcery,
			Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
				{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(2)},
				{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(5)},
			}}}})
		sc := tb.scen(t)
		if !sc.onBF(sc.perm(host, 0)) {
			t.Fatal("precondition: the replacement host is not on the battlefield")
		}
		o := sc.hand(spell, 0)
		s := sc.s
		digTop3(t, s)
		sc.castCard(t, o)
		sc.passBoth(t)
		if s.g.stalled != StallDrawRepl {
			t.Fatalf("stalled = %d, want StallDrawRepl (%d): the body's ask was dropped or answered unasked", s.g.stalled, StallDrawRepl)
		}
	})
}
