package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Sorcerous Spyglass at a pod (CR 606, gorge effChoosePlayer): the as-enters
// chain is ChoosePlayer -> RevealHand (Look$ over the chosen player) ->
// NameCard -> Cleanup, and the choice is asked among the LIVING OPPONENTS,
// not answered as the one opponent. The kernel enters program is OpExt
// ExtChoosePlayer ahead of OpExt ExtChooseName B = 2, so at four seats the
// controller is asked over the three opponents in AliveFrom order, the
// look_ack reads the chosen player's hand (posed only while it is
// non-empty), and the name ask follows over any card; the name overwrites
// the chosen byte the player rode until the look_ack (gorge's
// ClearChosenPlayer$ Cleanup clears it at the same point).

// spyglassPodScen seats n players of the spyglass pool, clears the opening
// hands (the seat 1 one stays empty, the seat 2 one holds one card), and
// stands at seat 0's first main phase.
func spyglassPodScen(t *testing.T, n uint8) (*scen, CardID) {
	t.Helper()
	set := cardsettest.Set(t, "spyglass")
	ctrl := []cardset.Entry{{Name: "Sorcerous Spyglass", Count: 4}, {Name: "Island", Count: 36}}
	rest := []cardset.Entry{{Name: "Island", Count: 40}}
	decks := make([][]cardset.Entry, n)
	decks[0] = ctrl
	for p := 1; p < int(n); p++ {
		decks[p] = rest
	}
	tbl, da, err := set.MatchupN(decks)
	if err != nil {
		t.Fatal(err)
	}
	spy := da[0][0]
	if tbl.Cards[spy].Name != "Sorcerous Spyglass" {
		t.Fatalf("deck card 0 is %q, want the Spyglass", tbl.Cards[spy].Name)
	}
	var cds [MaxSeats][]CardID
	for p := 0; p < int(n); p++ {
		cds[p] = da[p]
	}
	s := new(State)
	s.InitN(tbl, cds, n, 1, Config{NoElide: true})
	for p := uint8(0); p < n; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libTop(o)
		}
	}
	s.draw(2) // seat 2's hand is the non-empty one
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, spy
}

// spyglassLibObj is seat p's Spyglass in its library.
func spyglassLibObj(t *testing.T, s *State, p uint8, spy CardID) Obj {
	t.Helper()
	pl := &s.pl[p]
	for i := pl.libLo; i < pl.libHi; i++ {
		if s.objs[pl.lib[i]].card == spy {
			return pl.lib[i]
		}
	}
	t.Fatalf("seat %d: no Spyglass in the library", p)
	return NoObj
}

// enterSpyglass puts a Spyglass of seat 0 onto the battlefield.
func enterSpyglass(t *testing.T, sc *scen, spy CardID) Slot {
	t.Helper()
	o := spyglassLibObj(t, sc.s, 0, spy)
	sc.s.detach(o)
	return sc.s.enter(sc.k, o, 0, false, kop.ZoneLibrary)
}

// TestSpyglassChoosesAmongOpponentsPod: at four seats the entering Spyglass
// asks over the three opponents in AliveFrom order; the look_ack poses only
// at the CHOSEN player's non-empty hand and the name ask follows, over any
// card name; a chosen player with an empty hand is posed no look_ack.
func TestSpyglassChoosesAmongOpponentsPod(t *testing.T) {
	sc, spy := spyglassPodScen(t, MaxSeats)
	s := sc.s
	sl := enterSpyglass(t, sc, spy)
	d := s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenPlayer || d.Player != 0 || d.N != 3 {
		t.Fatalf("ask 1: kind %d sub %d player %d n %d, want seat 0's choice among the three opponents", d.Kind, d.Sub, d.Player, d.N)
	}
	for i, want := range []uint8{1, 2, 3} {
		if m := d.Options[i].Mode(); m != want {
			t.Fatalf("ask 1 option %d is seat %d, want %d (AliveFrom order)", i, m, want)
		}
	}
	s.Apply(d.Options[1]) // seat 2, the one holding a card
	d = s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenLookAck || d.N != 1 {
		t.Fatalf("ask 2: kind %d sub %d n %d, want the look_ack at seat 2's hand", d.Kind, d.Sub, d.N)
	}
	s.Apply(d.Options[0])
	d = s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenName || d.Player != 0 || d.N == 0 {
		t.Fatalf("ask 3: kind %d sub %d player %d n %d, want the any-card name ask", d.Kind, d.Sub, d.Player, d.N)
	}
	named := d.Options[0].Mode()
	s.Apply(d.Options[0])
	if c, ok := s.chosenNameOf(sl); !ok || c != CardID(named) {
		t.Fatalf("the Spyglass named %d (ok %v), want the answered card %d", c, ok, named)
	}

	// The look goes at the chosen player, not the next alive seat: with
	// seat 2's hand emptied the same answer poses no look_ack, the name ask
	// straight away (gorge's lookless hand).
	ho := s.pl[2].hand[0]
	s.detach(ho)
	s.objs[ho].zone = kop.ZoneLibrary
	s.pl[2].libTop(ho)
	sl2 := enterSpyglass(t, sc, spy)
	d = s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenPlayer || d.Player != 0 || d.N != 3 {
		t.Fatalf("ask 4: kind %d sub %d player %d n %d, want the opponents again", d.Kind, d.Sub, d.Player, d.N)
	}
	s.Apply(d.Options[1])
	d = s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenName || d.N == 0 {
		t.Fatalf("ask 5: kind %d sub %d n %d, want the name ask straight away (seat 2's hand is empty)", d.Kind, d.Sub, d.N)
	}
	named2 := d.Options[0].Mode()
	s.Apply(d.Options[0])
	if c, ok := s.chosenNameOf(sl2); !ok || c != CardID(named2) {
		t.Fatalf("the second Spyglass named %d (ok %v), want the answered card %d", c, ok, named2)
	}
}
