package feat

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

func TestExtBeliefAllocsNothing(t *testing.T) {
	e := extVanillaEncoder()
	e.SetExt(ExtAll)
	bel := make([]uint8, 64)
	for i := range bel {
		bel[i] = 2
	}
	e.SetOppBelief(bel)
	s, pol := extVanillaGame(5)
	var d = new(dirac.Decision)
	for n := 0; n < 40; n++ {
		s.StepInto(d)
		s.Apply(pol.Pick(d))
	}
	s.StepInto(d)
	e.EncodeState(s, d)
	if a := testing.AllocsPerRun(50, func() { e.EncodeState(s, d) }); a != 0 {
		t.Errorf("allocs per encode with a belief: %v", a)
	}
}
