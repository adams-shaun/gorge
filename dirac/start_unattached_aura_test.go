package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestInitNStartUnattachedAuraMatchesGorge: an Aura listed in Battlefield
// with no Attach entry is refused on entry (CR 303.4g) as gorge's applyStart
// refuses it: it stays in the library, listed in the library order, while the
// plain card beside it enters.
func TestInitNStartUnattachedAuraMatchesGorge(t *testing.T) {
	tbl, _, _, aura := startTable()
	var decks [MaxSeats][]CardID
	decks[0] = startDeck(v.Plains, aura, v.GrizzlyBears)
	decks[1] = startDeck(v.Island, v.GrizzlyBears)
	st := &Start{Seats: []StartSeat{{Battlefield: []int{1, 2}}}}
	s := new(State)
	s.InitNStart(tbl, decks, 2, 5, Config{NoElide: true}, st)
	if got := s.objs[2].zone; got != kop.ZoneBattlefield {
		t.Fatalf("plain card in zone %d, want the battlefield", got)
	}
	if got := s.objs[1].zone; got != kop.ZoneLibrary {
		t.Fatalf("unattached Aura in zone %d, want the library", got)
	}
	if !slices.Contains(libOf(s, 0), Obj(1)) {
		t.Fatal("unattached Aura is not in the seat's library order")
	}
	if slices.Contains(libOf(s, 0), Obj(2)) {
		t.Fatal("the entered card is still in the library order")
	}
}

// TestInitNStartUnattachedAuraRefusals: a Tapped, BackFace or Counters entry
// naming the Aura that stays in the library panics, as it names no permanent.
func TestInitNStartUnattachedAuraRefusals(t *testing.T) {
	tbl, _, _, aura := startTable()
	var decks [MaxSeats][]CardID
	decks[0] = startDeck(v.Plains, aura, v.GrizzlyBears)
	decks[1] = startDeck(v.Island, v.GrizzlyBears)
	bad := map[string]StartSeat{
		"tapped":   {Battlefield: []int{1}, Tapped: []int{1}},
		"backface": {Battlefield: []int{1}, BackFace: []int{1}},
		"counters": {Battlefield: []int{1}, Counters: []StartCounter{{Card: 1, N: 1}}},
	}
	for name, sd := range bad {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: want a panic", name)
				}
			}()
			s := new(State)
			s.InitNStart(tbl, decks, 2, 5, Config{NoElide: true}, &Start{Seats: []StartSeat{sd}})
		}()
	}
}
