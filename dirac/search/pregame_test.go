package search

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestDealUnseen: DealUnseen keeps each seat's hidden multiset, moves the
// real hands, and deals the same cards to every object whatever the real
// hands held (the two states differ only in which hidden cards sit in the
// hands); NewEpoch never repeats or returns 0.
func TestDealUnseen(t *testing.T) {
	var a, b []dirac.CardID
	for i := 0; i < 40; i++ {
		a = append(a, []dirac.CardID{vanilla.Plains, vanilla.GrizzlyBears, vanilla.SerraAngel}[i%3])
		b = append(b, []dirac.CardID{vanilla.Island, vanilla.Shock}[i%2])
	}
	var s1 dirac.State
	s1.Init(vanilla.Table, a, b, 11, dirac.Config{Mulligans: 2})
	var s2 dirac.State
	s2.CopyFrom(&s1)
	var buf [2 * dirac.MaxDeck]dirac.CardID
	for p := uint8(0); p < 2; p++ {
		r := dirac.NewRand(uint64(p)+90, 91)
		n := s2.HiddenCards(p, buf[:])
		s2.DealHidden(p, &r, buf[:n])
	}
	hidden := func(s *dirac.State, p uint8) []dirac.CardID {
		n := s.HiddenCards(p, buf[:])
		return slices.Clone(buf[:n])
	}
	before := [2][]dirac.CardID{hidden(&s1, 0), hidden(&s1, 1)}
	sr := NewSearcher(DefaultOptions(), nil, nil)
	var c1, c2 dirac.State
	c1.CopyFrom(&s1)
	c2.CopyFrom(&s2)
	sr.DealUnseen(&c1, 0, 5, 6)
	sr.DealUnseen(&c2, 0, 5, 6)
	moved := false
	for p := uint8(0); p < 2; p++ {
		h1, h2 := hidden(&c1, p), hidden(&c2, p)
		if !slices.Equal(h1, h2) {
			t.Fatalf("seat %d: deals differ with the real hands: %v vs %v", p, h1, h2)
		}
		moved = moved || !slices.Equal(h1, before[p])
		slices.Sort(h1)
		b := slices.Clone(before[p])
		slices.Sort(b)
		if !slices.Equal(h1, b) {
			t.Fatalf("seat %d: hidden multiset changed", p)
		}
	}
	if !moved {
		t.Fatal("DealUnseen dealt the real hands back")
	}
	e0, e1 := NewEpoch(), NewEpoch()
	if e0 == 0 || e1 == 0 || e0 == e1 {
		t.Fatalf("NewEpoch %d, %d", e0, e1)
	}
}
