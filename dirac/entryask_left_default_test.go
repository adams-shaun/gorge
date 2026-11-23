package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// leftReturnScen seats MaxSeats players of the pool, every deck four copies
// of the named card beside copies of the land, and stands at seat 0's first
// main phase. Seat 1's land exiles seat 2's copy of the named card until it
// leaves (recordExileLink) and seat 1 has lost: the sweep's exile return is the
// entry no resolution run serves (coldEntryNoRun), so the returned card's
// as-enters asks must be answered unasked with gorge's defaults, never
// queued and posed.
func leftReturnScen(t *testing.T, pool, card, land string, extra ...cardset.Entry) (*scen, CardID) {
	t.Helper()
	set := cardsettest.Set(t, pool)
	entries := []cardset.Entry{{Name: card, Count: 4}, {Name: land, Count: 36}}
	entries = append(entries, extra...)
	decks := make([][]cardset.Entry, MaxSeats)
	for p := 0; p < MaxSeats; p++ {
		decks[p] = entries
	}
	tbl, da, err := set.MatchupN(decks)
	if err != nil {
		t.Fatal(err)
	}
	asked := da[0][0]
	if tbl.Cards[asked].Name != card {
		t.Fatalf("deck card 0 is %q, want %q", tbl.Cards[asked].Name, card)
	}
	var cds [MaxSeats][]CardID
	for p := 0; p < MaxSeats; p++ {
		cds[p] = da[p]
	}
	s := new(State)
	s.InitN(tbl, cds, MaxSeats, 1, Config{NoElide: true})
	for p := uint8(0); p < MaxSeats; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libTop(o)
		}
	}
	k := s.k()
	sc := &scen{s: s, k: k}

	// seat 3's hand holds one card: the returned card's default player pick
	// is the first living opponent of its controller, so the look the chain
	// makes is at seat 3's non-empty hand
	s.draw(3)

	// seat 2's copy of the card sits in exile
	eo := Obj(NoObj)
	{
		pl := &s.pl[2]
		for i := pl.libLo; i < pl.libHi; i++ {
			if s.objs[pl.lib[i]].card == asked {
				eo = pl.lib[i]
				break
			}
		}
		if eo == Obj(NoObj) {
			t.Fatalf("seat 2: no %s in the library", card)
		}
		s.detach(eo)
		s.objs[eo].zone = kop.ZoneExile
		pl.exile[pl.exileN] = eo
		pl.exileN++
	}

	// seat 1's land exiles it until that land leaves
	var isle Obj
	{
		pl := &s.pl[1]
		for i := pl.libLo; i < pl.libHi; i++ {
			o := pl.lib[i]
			if k.cards[s.objs[o].card].land {
				s.detach(o)
				isle = o
				break
			}
		}
		if isle == Obj(NoObj) {
			t.Fatal("seat 1: no Island in the library")
		}
	}
	s.enter(k, isle, 1, false, kop.ZoneLibrary)
	s.recordExileLink(isle, eo)
	return sc, asked
}

// leftReturnField is the battlefield slot of seat 2's returned copy of the
// named card after the sweep, NoSlot when it is not there.
func leftReturnField(t *testing.T, sc *scen, asked CardID) Slot {
	t.Helper()
	for m := sc.s.occ; m.any(); m = m.rest() {
		sl := m.first()
		p := &sc.s.perms[sl]
		if p.ctrl == 2 && sc.s.objs[p.obj].card == asked {
			return sl
		}
	}
	t.Fatalf("seat 2: the returned %s is not on the battlefield", "card")
	return NoSlot
}

// TestEnterAsksDefaultAfterControllerLeft: a player leaves the game while a
// spell is resolving, and the exile return their sweep makes re-enters the
// cards their permanent exiled. gorge runs that replacement body straight
// off the emit with no resolution run, so every as-enters ask of the
// returned card defaults to its first option and is never posed: the player
// pick at The Rack is the first living opponent in turn order, the
// Spyglass's look at that opponent is acknowledged unasked, and its name is
// the first sorted card name of the matchup.
func TestEnterAsksDefaultAfterControllerLeft(t *testing.T) {
	// The Rack: the player pick is the one as-enters ask, and it rides the
	// permanent's chosen byte until a reader (its upkeep trigger) reads it.
	sc, rack := leftReturnScen(t, "chosenopp", "The Rack", "Island")
	s := sc.s
	pre := s.outMask()
	s.pl[1].lost = true
	s.sweepDeparted(sc.k, pre)
	if s.g.nask != 0 {
		t.Fatalf("the sweep queued %d asks, want none", s.g.nask)
	}
	sl := leftReturnField(t, sc, rack)
	if p := s.chosenPlayerOf(sl); p != 3 {
		t.Fatalf("the Rack chose player %d, want 3: the first living opponent of seat 2 in turn order (seat 1 has left)", p)
	}
	if d := s.Step(); d.Kind == DkChoose {
		t.Fatalf("a decision was posed after the sweep: kind %d sub %d player %d n %d, want the game carrying on unasked", d.Kind, d.Sub, d.Player, d.N)
	}

	// Sorcerous Spyglass: the chain is ChoosePlayer -> RevealHand (Look$ over
	// the chosen player) -> NameCard -> Cleanup; the look's acknowledgment
	// and the name default the same way, and the name overwrites the chosen
	// byte the player rode.
	sc, spy := leftReturnScen(t, "spyglass", "Sorcerous Spyglass", "Island")
	s = sc.s
	pre = s.outMask()
	s.pl[1].lost = true
	s.sweepDeparted(sc.k, pre)
	if s.g.nask != 0 {
		t.Fatalf("the sweep queued %d asks, want none", s.g.nask)
	}
	sl = leftReturnField(t, sc, spy)
	first := s.k().t.NameCardsAny[0]
	if c, ok := s.chosenNameOf(sl); !ok || c != CardID(first) {
		t.Fatalf("the Spyglass named %d (ok %v), want %d: the first sorted card name of the matchup", c, ok, first)
	}
	if d := s.Step(); d.Kind == DkChoose {
		t.Fatalf("a decision was posed after the sweep: kind %d sub %d player %d n %d, want the game carrying on unasked", d.Kind, d.Sub, d.Player, d.N)
	}
}
