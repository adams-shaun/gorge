package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAffordsHybridAllocs: the hybrid and twobrid affordability checks
// resolve their pips on the stack (affordsPlain breaks the affords cycle
// that moved each resolution to the heap) and keep their answers.
func TestAffordsHybridAllocs(t *testing.T) {
	var a avail
	a.addUnits(uint8(kop.ColU), 1)
	a.addUnits(uint8(kop.ColR), 1)
	two := kop.ManaCost{Hyb: [len(kop.ManaCost{}.Hyb)]kop.ColorMask{kop.ColW | kop.ColU, kop.ColB | kop.ColR}} // {W/U}{B/R}
	one := kop.ManaCost{Generic: 1, Hyb: [len(kop.ManaCost{}.Hyb)]kop.ColorMask{kop.ColW | kop.ColB}}          // {1}{W/B}
	tb := kop.ManaCost{Hyb: [len(kop.ManaCost{}.Hyb)]kop.ColorMask{kop.HybTwobrid}, Tb: uint16(kop.R + 1)}     // {2/R}
	for _, c := range []struct {
		name          string
		cost          kop.ManaCost
		want, wantAny bool // the any-colour rider: any coloured unit pays a coloured pip
	}{{"{W/U}{B/R}", two, true, true}, {"{1}{W/B}", one, false, true}, {"{2/R}", tb, true, true}} {
		got, gotAny := a.affords(&c.cost, pipMaskOf(&c.cost), 0), a.affordsAnyColor(&c.cost, 0)
		if got != c.want || gotAny != c.wantAny {
			t.Errorf("%s: affords %v, affordsAnyColor %v, want %v, %v", c.name, got, gotAny, c.want, c.wantAny)
		}
		if n := testing.AllocsPerRun(20, func() { a.affords(&c.cost, pipMaskOf(&c.cost), 0); a.affordsAnyColor(&c.cost, 0) }); n != 0 {
			t.Errorf("%s: %.0f allocations, want 0", c.name, n)
		}
	}
}
