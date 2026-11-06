package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantNeedFirstColours: a plain {1}{G} cast's announced colours (need)
// order the grant search's colour ask (gorge's scriptOrderAnswers on the
// prefix search's asks, potential_script.go): a budget-capped search tries
// a colour the cast needs before the colours it does not, so the first
// path it finds answers the ask with G. With no need (an alternative cost
// leaves the set empty) the search keeps the engine order and answers W.
// The answer set is the same either way: the found path is the first one
// the search order reaches.
func TestGrantNeedFirstColours(t *testing.T) {
	sc, _, _ := tokScen(t, Config{NoElide: true}, "Springleaf Parade", tokToken, "Forest")
	s, k := sc.s, sc.k
	cost := kop.ManaCost{Generic: 1}
	cost.Pips[kop.G] = 1
	need := grantNeedColours(cost)
	if !need[kop.G] || need[kop.W] || need[kop.C] {
		t.Fatalf("need %v, want only G ({1}{G} announces green)", need)
	}
	var g grantSrch
	if !s.grantSearch(k, 0, &cost, NoSlot, payDesc{}, need, &g) || g.np != 1 || g.path[0].col != kop.G {
		t.Fatalf("need G: found %v steps %d col %d, want one step making G", g.np == 1, g.np, g.path[0].col)
	}
	// the same shape with an empty need (an alternative cost): the engine
	// order, W first
	if !s.grantSearch(k, 0, &cost, NoSlot, payDesc{}, [kop.NumManaKinds]bool{}, &g) || g.np != 1 || g.path[0].col != kop.W {
		t.Fatalf("no need: found %v steps %d col %d, want one step making W (W U B R G C order)",
			g.np == 1, g.np, g.path[0].col)
	}
}
