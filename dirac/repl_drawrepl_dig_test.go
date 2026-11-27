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
	t.Run("a resolving two-card draw asks each card's choose", func(t *testing.T) {
		// The spell's OpDraw lends the body its resCtx, which cannot carry
		// the body's pc past the ask; the draw is resumable through the
		// OpDraw loop, so the body parks its choose on a context of its own
		// and the answer re-enters the loop past the asked card, then the
		// spell's program goes on (repl_drawbody.go drawBodyParkRes,
		// drawBodyEndRes).
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
		pl := &s.pl[0]
		if pl.libLen() < 8 {
			t.Fatalf("precondition: library = %d, want at least 8", pl.libLen())
		}
		var top [6]Obj
		copy(top[:], pl.lib[pl.libLo:pl.libLo+6])
		hand, grave, lib, life := pl.handN, pl.graveN, pl.libLen(), pl.life
		sc.castCard(t, o)
		sc.passBoth(t)
		d := digAsk(t, sc) // the first card's choose
		s.Apply(d.Options[0])
		d = digAsk(t, sc) // the second card's choose, after the loop's next card
		s.Apply(d.Options[1])
		if d := s.Step(); d.Kind != DkPriority {
			t.Fatalf("decision kind %d (stalled %d, res.ask %d): want priority after the spell", d.Kind, s.g.stalled, s.res.ask)
		}
		toHand, toGrave := 0, 0
		for _, x := range top {
			switch s.objs[x].zone {
			case kop.ZoneHand:
				toHand++
			case kop.ZoneGraveyard:
				toGrave++
			}
		}
		// +1 hand: the two bodies' picks (the captured hand still holds the
		// cast spell); +5 grave: four milled by the bodies, plus the
		// sorcery's rest.
		if toHand != 2 || toGrave != 4 || pl.handN != hand+1 || pl.graveN != grave+5 ||
			pl.libLen() != lib-6 || pl.life != life+5 {
			t.Fatalf("hand %d grave %d life %d lib %d (top cards %d to hand, %d to grave): want two drawn one card at a time through the body, then the spell's life gain",
				pl.handN, pl.graveN, pl.life, pl.libLen(), toHand, toGrave)
		}
		if s.g.stalled != 0 {
			t.Fatalf("stalled %d: the answered body stalled", s.g.stalled)
		}
		if ds, pk := s.drawState(), s.drawParkRecord(); len(ds) > 0 && ds[0].active ||
			len(pk) > 0 && pk[0].on {
			t.Fatal("coldDraw still live after the body and the spell finished")
		}
	})
	t.Run("a resolving two-card draw asks the rest's arrange too", func(t *testing.T) {
		// Sages of the Anima's shape: one taken to hand, the rest to the
		// bottom in an order the player picks (raGenDigArrange), twice: the
		// arrange's answer ends the body and re-enters the OpDraw loop
		// (drawBodyEndRes).
		tb := newSPTable()
		// B = -2 (take any number) with DigOptional: the pick closes on
		// Done (Sages of the Anima's "put any number into your hand").
		host := addCard(tb, mayProg(0, kop.Instr{Op: kop.OpDigEx, Obj: kop.RefTriggerPlayer,
			A: kop.Const(3), B: kop.Const(-2),
			C: int16(kop.ZoneHand) | int16(kop.ZoneLibrary|kop.DigOptional)<<3}))
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
		pl := &s.pl[0]
		if pl.libLen() < 8 {
			t.Fatalf("precondition: library = %d, want at least 8", pl.libLen())
		}
		var top [6]Obj
		copy(top[:], pl.lib[pl.libLo:pl.libLo+6])
		bottom := make([]Obj, 6)
		copy(bottom, pl.lib[pl.libHi-6:pl.libHi])
		hand, grave, lib, life := pl.handN, pl.graveN, pl.libLen(), pl.life
		sc.castCard(t, o)
		sc.passBoth(t)
		arrangeAsk := func(want int) Decision {
			t.Helper()
			d := sc.s.Step()
			if d.Kind != DkChoose || int(d.N) != want {
				t.Fatalf("arrange decision kind %d player %d with %d options (stalled %d, res.ask %d), want seat 0's order among %d",
					d.Kind, d.Player, d.N, sc.s.g.stalled, sc.s.res.ask, want)
			}
			return d
		}
		var picked [4]Obj
		// take one of three, close the pick with Done, then order the two
		// left, as the lockstep bot does (Sages of the Anima: "any number").
		// picked collects the four arrange picks in order.
		takeOne := func(base int) {
			t.Helper()
			for round := 0; round < 2; round++ {
				d := sc.s.Step()
				if d.Kind != DkChoose || !isDigAsk(s.res.ask) {
					t.Fatalf("pick decision kind %d with %d options (stalled %d, res.ask %d): want the Dig choose",
						d.Kind, d.N, s.g.stalled, s.res.ask)
				}
				pick, done := -1, -1
				for i := 0; i < int(d.N); i++ {
					switch d.Options[i].Kind() {
					case AkChoose:
						if pick < 0 {
							pick = i
						}
					case AkDone:
						done = i
					}
				}
				if pick < 0 || done < 0 {
					t.Fatalf("pick decision with %d options offers no card or no Done (stalled %d, res.ask %d)", d.N, s.g.stalled, s.res.ask)
				}
				if round == 0 {
					s.Apply(d.Options[pick]) // take one
					continue
				}
				s.Apply(d.Options[done]) // "any number": close the pick
			}
			for i := 0; i < 2; i++ {
				d := arrangeAsk(2 - i)
				s.Apply(d.Options[0])
				picked[base+i] = Obj(d.Options[0].Source())
			}
		}
		takeOne(0) // the first drawn card's body
		takeOne(2) // the second drawn card's body
		if d := s.Step(); d.Kind != DkPriority {
			t.Fatalf("decision kind %d (stalled %d, res.ask %d): want priority after the spell", d.Kind, s.g.stalled, s.res.ask)
		}
		toHand, inLib := 0, 0
		for _, x := range top {
			switch s.objs[x].zone {
			case kop.ZoneHand:
				toHand++
			case kop.ZoneLibrary:
				inLib++
			default:
				t.Fatalf("a top card sits in zone %d (want hand or library): the dig lost it", s.objs[x].zone)
			}
		}
		if toHand != 2 || inLib != 4 || pl.handN != hand+1 || pl.graveN != grave+1 ||
			pl.libLen() != lib-2 || pl.life != life+5 { // the two taken cards leave the library
			t.Fatalf("hand %d grave %d life %d lib %d (%d to hand, %d still in the library): want two taken one card at a time, four arranged to the bottom, then the spell's life gain",
				pl.handN, pl.graveN, pl.life, pl.libLen(), toHand, inLib)
		}
		for _, x := range bottom {
			if s.objs[x].zone != kop.ZoneLibrary {
				t.Fatalf("a pre-existing bottom card moved to zone %d", s.objs[x].zone)
			}
		}
		for i, x := range picked { // the arranged cards are the library's new bottom four, in pick order
			if pl.lib[int(pl.libHi)-4+i] != x {
				t.Fatalf("bottom %d is %d, want the %dth arrange pick %d", i, pl.lib[int(pl.libHi)-4+i], i+1, x)
			}
		}
		if s.g.stalled != 0 {
			t.Fatalf("stalled %d: the answered body stalled", s.g.stalled)
		}
	})
}
