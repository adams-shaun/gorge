package hbot

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestLiteFillsMatchFull: at every decision of vanilla self-play (hold off
// and on), the answer over fillPriority's and fillCombat's censuses equals
// the answer over the whole board (fill).
func TestLiteFillsMatchFull(t *testing.T) {
	for _, hold := range []bool{false, true} {
		bt := New(NewFacts(v.Table))
		ref := New(NewFacts(v.Table))
		ref.b.forceFull = true
		if hold {
			bt.EnableHoldInstant()
			ref.EnableHoldInstant()
		}
		n := 0
		for seed := uint64(1); seed <= 40; seed++ {
			decks := [2]int{int(seed) % len(v.Decks), int(seed/3) % len(v.Decks)}
			playSelf(t, bt, seed, decks, func(s *dirac.State, d *dirac.Decision, i int) {
				if d.N <= 1 {
					return
				}
				n++
				if j := ref.Pick(s, d); j != i {
					t.Fatalf("hold %v seed %d kind %d: lite %d, full %d", hold, seed, d.Kind, i, j)
				}
			})
		}
		if n < 1000 {
			t.Fatalf("hold %v: only %d decisions", hold, n)
		}
	}
}
