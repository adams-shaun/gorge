package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newRevealScen is newScenTableLib over the reveal pool: Daring Buccaneer
// (R, Reveal<1/Pirate> or {2}), Dragon's Eye Savants (Morph, Reveal<1/Card.Blue>),
// Swab Goblin (a Pirate), Merfolk of the Pearl Trident (blue), a library
// of Islands.
func newRevealScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Daring Buccaneer", Count: 2},
		{Name: "Dragon's Eye Savants", Count: 2},
		{Name: "Swab Goblin", Count: 4},
		{Name: "Merfolk of the Pearl Trident", Count: 4},
		{Name: "Mountain", Count: 4},
		{Name: "Island", Count: 4},
	}
	set := cardsettest.Set(t, "reveal")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTableLib(t, tbl, id["Island"], Config{NoElide: true, GorgeAltAdd: true}), id
}

// castsOf reports whether the priority decision offers p's cast of o.
func (sc *scen) castsOf(o Obj) bool {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(o) {
			return true
		}
	}
	return false
}

// castRevealRun casts o and answers every ask with its first option, taking
// the reveal pick's option count into picks (-1: none was posed) and its
// candidates into cands.
func (sc *scen) castRevealRun(t *testing.T, o Obj) (picks int, cands []Obj) {
	t.Helper()
	picks = -1
	sc.do(t, AkCast, TCode(o), scAny)
	for i := 0; i < 12; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			return picks, cands
		}
		if d.Kind == DkChoose && d.Sub == ChCostReveal {
			picks = int(d.N)
			for j := 0; j < int(d.N); j++ {
				cands = append(cands, Obj(d.Options[j].Source()))
			}
		}
		sc.s.Apply(d.Options[0])
	}
	t.Fatal("the cast did not settle")
	return
}

// TestRevealPickCastRoute: Reveal<1/Spec> as a cast-route cost (gorge
// revealCostAsk, turnUpRevealAsk). A cast whose reveal part has no candidate
// but the cast card is not offered; one candidate is revealed unasked; two or
// more pose the "revealcost" pick without the cast card. A face-down
// permanent's keyword turn-up reveals the same way, a manifested copy's
// printed-mana-cost turn-up reveals nothing.
func TestRevealPickCastRoute(t *testing.T) {
	t.Run("cast alone in hand", func(t *testing.T) {
		sc, id := newRevealScen(t)
		sc.perm(id["Mountain"], 0)
		bucc := sc.hand(id["Daring Buccaneer"], 0)
		if sc.castsOf(bucc) {
			t.Fatal("Daring Buccaneer is offered with no other Pirate to reveal and {R} the only mana ({2} more is owed otherwise)")
		}
	})
	t.Run("cast one candidate", func(t *testing.T) {
		sc, id := newRevealScen(t)
		sc.perm(id["Mountain"], 0)
		bucc := sc.hand(id["Daring Buccaneer"], 0)
		sc.hand(id["Swab Goblin"], 0)
		if !sc.castsOf(bucc) {
			t.Fatal("Daring Buccaneer is not offered with a Pirate to reveal")
		}
		picks, _ := sc.castRevealRun(t, bucc)
		if picks != -1 {
			t.Fatalf("the lone candidate was asked (%d options)", picks)
		}
		if sc.s.objs[bucc].zone != kop.ZoneStack {
			t.Fatalf("the cast did not reach the stack (zone %v)", sc.s.objs[bucc].zone)
		}
	})
	t.Run("cast two candidates", func(t *testing.T) {
		sc, id := newRevealScen(t)
		sc.perm(id["Mountain"], 0)
		bucc := sc.hand(id["Daring Buccaneer"], 0)
		a, b := sc.hand(id["Swab Goblin"], 0), sc.hand(id["Swab Goblin"], 0)
		picks, cands := sc.castRevealRun(t, bucc)
		if picks != 2 || len(cands) != 2 || cands[0] == bucc || cands[1] == bucc || cands[0] == cands[1] {
			t.Fatalf("reveal pick options %v (%d), want the two other Pirates %d and %d, never the cast card %d", cands, picks, a, b, bucc)
		}
		if sc.s.objs[bucc].zone != kop.ZoneStack {
			t.Fatalf("the cast did not reach the stack (zone %v)", sc.s.objs[bucc].zone)
		}
		for _, o := range []Obj{a, b} {
			if sc.s.objs[o].zone != kop.ZoneHand {
				t.Fatalf("a revealed card left the hand (zone %v)", sc.s.objs[o].zone)
			}
		}
	})
	t.Run("turn-up two candidates", func(t *testing.T) {
		sc, id := newRevealScen(t)
		sav := sc.obj(id["Dragon's Eye Savants"], 0)
		sl := sc.s.enterFaceDown(sc.k, sav, 0, kop.MorphMorph)
		sc.s.npend = 0
		if sc.turnUpOffers(0) != 0 {
			t.Fatal("the turn-up is offered with no blue card to reveal")
		}
		sc.hand(id["Merfolk of the Pearl Trident"], 0)
		sc.hand(id["Merfolk of the Pearl Trident"], 0)
		if n := sc.turnUpOffers(0); n != 1 {
			t.Fatalf("turn-up offers = %d, want 1", n)
		}
		sc.s.Apply(MakeAction(AkTurnFaceUp, TCode(sav), 0, 0))
		if !sc.s.faceDown(sc.k, sl) {
			t.Fatal("the turn-up completed before the reveal was chosen")
		}
		d := sc.s.Step()
		if d.Kind != DkChoose || d.Sub != ChCostReveal || d.N != 2 {
			t.Fatalf("decision kind %d sub %d options %v, want the two-card ChCostReveal pick", d.Kind, d.Sub, d.Options[:d.N])
		}
		sc.s.Apply(d.Options[1])
		if sc.s.faceDown(sc.k, sl) {
			t.Fatal("Savants is still face down after the reveal")
		}
		if sc.s.pl[0].handN != 2 {
			t.Fatalf("hand %d after the reveal, want both blue cards kept", sc.s.pl[0].handN)
		}
	})
	t.Run("turn-up one candidate", func(t *testing.T) {
		sc, id := newRevealScen(t)
		sav := sc.obj(id["Dragon's Eye Savants"], 0)
		sl := sc.s.enterFaceDown(sc.k, sav, 0, kop.MorphMorph)
		sc.s.npend = 0
		sc.hand(id["Merfolk of the Pearl Trident"], 0)
		sc.hand(id["Mountain"], 0)
		sc.s.Apply(MakeAction(AkTurnFaceUp, TCode(sav), 0, 0))
		if sc.s.faceDown(sc.k, sl) {
			t.Fatal("the lone blue card was not revealed unasked")
		}
	})
	t.Run("manifested copy", func(t *testing.T) {
		sc, id := newRevealScen(t)
		topCard(sc, id["Dragon's Eye Savants"])
		sc.s.res.e.ctrl = 0
		in := kop.Instr{Op: kop.OpManifest, Obj: kop.RefController, A: kop.Const(1)}
		if !sc.s.exec(sc.k, &in) {
			t.Fatal("Manifest suspended")
		}
		sc.s.pl[0].pool[kop.U], sc.s.pl[0].pool[kop.C] = 1, 1
		modes := func() map[uint8]bool {
			var d Decision
			sc.s.offerTurnFaceUp(sc.k, &d, 0)
			m := map[uint8]bool{}
			for i := 0; i < int(d.N); i++ {
				m[d.Options[i].Mode()] = true
			}
			return m
		}
		if m := modes(); !m[1] || m[3] || len(m) != 1 {
			t.Fatalf("turn-up modes %v with no blue card in hand, want the printed mana cost (1) alone", m)
		}
		sc.hand(id["Merfolk of the Pearl Trident"], 0)
		if m := modes(); !m[1] || !m[3] || len(m) != 2 {
			t.Fatalf("turn-up modes %v with a blue card in hand, want the mana cost (1) and the morph reveal (3)", m)
		}
	})
}
