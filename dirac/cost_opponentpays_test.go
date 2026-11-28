package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestEachPayerAPNAPOrder: an UnlessPayer$ Opponent or Player walk asks in
// APNAP order (CR 101.4): the active player first, then turn order, the
// controller left out of an opponents' walk and a departed seat skipped.
// Before the fix the kernel asked from seat 0 (and, before that, only the
// controller's next opponent).
func TestEachPayerAPNAPOrder(t *testing.T) {
	var decks [MaxSeats][]CardID
	for p := 0; p < 4; p++ {
		decks[p] = vanilla.Decks[p%len(vanilla.Decks)]
	}
	for _, tc := range []struct {
		name         string
		obj          kop.Ref
		active, ctrl uint8
		gone         []uint8
		want         []uint8
	}{
		{"opponents, controller's turn", kop.RefEachOpponent, 2, 2, nil, []uint8{3, 0, 1}},
		{"opponents, opponent's turn", kop.RefEachOpponent, 1, 2, nil, []uint8{1, 3, 0}},
		{"opponents, seat 0 active", kop.RefEachOpponent, 0, 2, nil, []uint8{0, 1, 3}},
		{"opponents, last seat active", kop.RefEachOpponent, 3, 0, nil, []uint8{3, 1, 2}},
		{"opponents, a departed seat", kop.RefEachOpponent, 1, 2, []uint8{3}, []uint8{1, 0}},
		{"opponents, departed active seat", kop.RefEachOpponent, 1, 2, []uint8{1}, []uint8{3, 0}},
		{"players", kop.RefEachPlayer, 2, 0, nil, []uint8{2, 3, 0, 1}},
		{"players, a departed seat", kop.RefEachPlayer, 2, 0, []uint8{3}, []uint8{2, 0, 1}},
	} {
		s := new(State)
		s.InitN(vanilla.Table, decks, 4, 1, Config{})
		if s.nSeats() != 4 {
			t.Fatalf("precondition: %d seats", s.nSeats())
		}
		s.active = tc.active
		s.res.e.ctrl = tc.ctrl
		for _, p := range tc.gone {
			s.pl[p].lost = true
			if !s.out(p) {
				t.Fatalf("precondition: seat %d has not left", p)
			}
		}
		in := &kop.Instr{Op: kop.OpUnlessPaid, Obj: tc.obj}
		var got []uint8
		q := s.eachPayers(in)
		for p := q.at(0); p != noSeat && len(got) <= MaxSeats; p = s.eachPayAfter(in, p) {
			got = append(got, p)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s (active %d, controller %d): asked %v, want %v", tc.name, tc.active, tc.ctrl, got, tc.want)
		}
	}
}

// TestExecPayStartSoleOpponent: a walk of one payer is the ordinary
// single-payer ask, several are a walk, and no living payer asks nobody
// (never seat 255).
func TestExecPayStartSoleOpponent(t *testing.T) {
	var decks [MaxSeats][]CardID
	for p := 0; p < 4; p++ {
		decks[p] = vanilla.Decks[p%len(vanilla.Decks)]
	}
	for _, tc := range []struct {
		name  string
		gone  []uint8
		payer uint8
		walk  bool
	}{
		{"three opponents walk", nil, noSeat, true},
		{"one opponent left", []uint8{0, 3}, 1, false},
		{"none left", []uint8{0, 1, 3}, noSeat, false},
	} {
		s := new(State)
		s.InitN(vanilla.Table, decks, 4, 1, Config{})
		s.active, s.res.e.ctrl = 2, 2
		for _, p := range tc.gone {
			s.pl[p].lost = true
		}
		k := &prep{t: vanilla.Table, eachPay: true}
		in := &kop.Instr{Op: kop.OpUnlessPaid, Obj: kop.RefEachOpponent}
		if p, walk := s.execPayStart(k, in); p != tc.payer || walk != tc.walk {
			t.Errorf("%s: payer %d walk %v, want %d %v", tc.name, p, walk, tc.payer, tc.walk)
		}
	}
}

// TestManaUnlessPayersAPNAPOrder: a mana ability's any-player unless asks
// from the active player on, in turn order (CR 101.4), as gorge's
// UnlessPayers does.
func TestManaUnlessPayersAPNAPOrder(t *testing.T) {
	var decks [MaxSeats][]CardID
	for p := 0; p < 4; p++ {
		decks[p] = vanilla.Decks[p%len(vanilla.Decks)]
	}
	s := new(State)
	s.InitN(vanilla.Table, decks, 4, 1, Config{})
	s.active = 2
	if s.nSeats() != 4 {
		t.Fatalf("precondition: %d seats", s.nSeats())
	}
	var got []uint8
	s.castWinUnlessStart()
	for s.winUnlessPayer() != 0 && len(got) <= MaxSeats {
		got = append(got, s.winUnlessPayer()-1)
		s.winUnlessSetPayer(0)
		s.castWinUnlessNext()
	}
	if want := []uint8{2, 3, 0, 1}; !slices.Equal(got, want) {
		t.Fatalf("asked %v, want %v", got, want)
	}
}
