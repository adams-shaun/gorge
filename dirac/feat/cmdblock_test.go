package feat

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestCommanderBlockRelativeSeats: the Commander block is ordered relative to
// the deciding seat, the order policynet writes: block 0 is the viewer and
// block 1 the other seat. With seat 1 deciding, an absolute-order fill writes
// the two seats' commander cast columns swapped.
func TestCommanderBlockRelativeSeats(t *testing.T) {
	// Seat 0's deck has lands and a cheap commander; seat 1's has only
	// creatures, so seat 0 casts its commander and seat 1 never does. That
	// makes the two seats' cast columns different.
	deck := make([]kop.CardID, 100)
	for i := range deck {
		deck[i] = vanilla.Forest
	}
	deck[0] = vanilla.GrizzlyBears
	other := make([]kop.CardID, 100)
	for i := range other {
		other[i] = vanilla.GrizzlyBears
	}
	s := dirac.NewConfig(vanilla.Table, deck, other, 9031,
		dirac.Config{NoElide: true, Commander: true, Commanders: [dirac.MaxSeats][2]int16{{0, -1}, {0, -1}}})
	if !s.FeatCommanderGame() {
		t.Fatal("setup: not a Commander game")
	}
	pol := dirac.NewRandomPolicy(9031)
	var d dirac.Decision
	found := false
	for n := 0; n < 20000; n++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		if s.FeatCommanderCast(0) != s.FeatCommanderCast(1) {
			found = true
			break
		}
		s.Apply(pol.Pick(&d))
	}
	if !found {
		t.Fatal("precondition: no position where the two seats' commander cast counts differ")
	}

	e := NewEncoder(NewPool(vanilla.Table, make([]CardSpec, len(vanilla.Table.Cards))))
	e.Reserve()
	for _, viewer := range []uint8{0, 1} {
		st := e.EncodeStateFor(s, &d, viewer)
		for blk := uint8(0); blk < 2; blk++ {
			p := (viewer + blk) & 1
			base := denseCmd0 + int(blk)*commanderBlockWidth
			wantCasts := float32(s.FeatCommanderCast(p)) / 4
			if wantCasts > 1 {
				wantCasts = 1
			}
			if got := st.Dense[base+denseCmdCasts]; got != wantCasts {
				t.Fatalf("viewer %d block %d cast column = %v, want seat %d's %v", viewer, blk, got, p, wantCasts)
			}
			src := p ^ 1
			wantDmg := float32(s.FeatCommanderDamage(p, src)) / 21
			if wantDmg > 1 {
				wantDmg = 1
			}
			if got := st.Dense[base+denseCmdDamage0]; got != wantDmg {
				t.Fatalf("viewer %d block %d damage column = %v, want seat %d taking from seat %d: %v", viewer, blk, got, p, src, wantDmg)
			}
			wantField := float32(0)
			if cmd := s.FeatCommander(p); cmd != dirac.NoObj && e.po(uint8(cmd)).Zone == kop.ZoneBattlefield {
				wantField = 1
			}
			if got := st.Dense[base+denseCmdOnField]; got != wantField {
				t.Fatalf("viewer %d block %d on-field bit = %v, want seat %d's %v", viewer, blk, got, p, wantField)
			}
		}
	}
}
