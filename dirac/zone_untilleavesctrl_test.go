package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestUntilLeavesReturnCountersFourSeat: a departure batch can hold sources
// from any seat, and each source's until-leaves return rider compares the
// returning card's owner with *that source's* last controller (CR 610.3).
// The departure frame packs one controller per held source; at four seats a
// controller needs two bits. The old one-bit packing read seats 2 and 3 as
// seats 0 and 1, so their returned cards got no counters (the rider's
// "under your control" test saw a stranger's card).
func TestUntilLeavesReturnCountersFourSeat(t *testing.T) {
	// A generated table with a card whose until-leaves exile carries the
	// +1/+1 rider (the shape kgen lowers for Lagrella, the Magpie). The
	// rider is read only for a generated table (gen.anyLinkCtr), so a
	// committed set's table takes one appended test card.
	set := cardsettest.Set(t, "blb")
	deck := []cardset.Entry{{Name: "Plains", Count: 40}}
	decks := [MaxSeats][]cardset.Entry{deck, deck, deck, deck}
	tbl, da, err := set.MatchupN(decks[:])
	if err != nil {
		t.Fatal(err)
	}
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	linker := kop.CardID(len(cards))
	cards = append(cards, kop.CardDef{
		Name:      "Test Until Leaves",
		Types:     kop.TCreature,
		Power:     2,
		Toughness: 2,
		Abilities: []kop.Ability{{
			Prog: []kop.Instr{{Op: kop.OpExileUntilLeaves, A: kop.Const(2)}},
		}},
	})
	tbl.Cards = cards
	plainsID := da[0][0]
	var lib [20]CardID
	for i := range lib {
		lib[i] = plainsID
	}
	s := new(State)
	s.InitN(tbl, [MaxSeats][]CardID{lib[:], lib[:], lib[:], lib[:]}, MaxSeats, 1, Config{NoElide: true})
	for p := uint8(0); p < MaxSeats; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	k := s.k()
	if !k.gen.anyLinkCtr {
		t.Fatal("the test table's until-leaves rider was not prepared")
	}

	newObj := func(c CardID, owner uint8) Obj {
		o := Obj(s.nobj)
		s.nobj++
		s.objs[o] = object{card: c, chosen: 255, slot: NoSlot, acts: owner << objOwnerShift}
		return o
	}
	newPerm := func(c CardID, p uint8) (Obj, Slot) {
		o := newObj(c, p)
		sl := s.enter(k, o, p, false, kop.ZoneNone)
		s.perms[sl].flags &^= pfSick
		return o, sl
	}
	link := func(src Obj, ret Obj) {
		l := &s.g.links[s.g.nlinks]
		l.src, l.sver, l.obj = src, s.objs[src].ver, ret
		s.g.nlinks++
	}
	var mask slotMask
	var sources, returns [2]Obj
	for i, seat := range []uint8{2, 3} {
		var srcSlot Slot
		sources[i], srcSlot = newPerm(linker, seat)
		returns[i] = newObj(plainsID, seat)
		s.place(returns[i], kop.ZoneExile, false)
		link(sources[i], returns[i])
		mask.set(srcSlot)
	}
	// One departure batch: both sources leave together, so the frame holds
	// both controllers at once.
	s.leaveAll(k, mask, kop.ZoneGraveyard)
	for i, seat := range []uint8{2, 3} {
		o := returns[i]
		if z := s.objs[o].zone; z != kop.ZoneBattlefield {
			t.Fatalf("seat %d's returned card is in zone %d, want the battlefield", seat, z)
		}
		sl := s.objs[o].slot
		if got := s.perms[sl].p1; got != 2 {
			t.Errorf("seat %d's returned card has %d +1/+1 counters, want 2 (controller packed as another seat)", seat, got)
		}
		if got := s.perms[sl].ctrl; got != seat {
			t.Errorf("seat %d's returned card is controlled by seat %d, want %d", seat, got, seat)
		}
	}
}
