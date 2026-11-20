package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ashlingOffered: p0 holds Ashling, the Extinguisher in hand beside board.
func ashlingOffered(t *testing.T, board ...string) bool {
	t.Helper()
	set := cardsettest.Synth(t, "tail-49")
	names := map[string]bool{"Plains": true}
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	for _, n := range append([]string{"Ashling, the Extinguisher"}, board...) {
		if !names[n] {
			names[n] = true
			i, ok := set.Lookup(n)
			if !ok || !set.Supported()[i] {
				t.Fatalf("%s unsupported (lookup %v)", n, ok)
			}
			deck = append(deck, cardset.Entry{Name: n, Count: 4})
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Plains"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	sc := &scen{s: s, k: s.k()}
	for _, n := range board {
		sc.perm(id[n], 0)
	}
	h := sc.hand(id["Ashling, the Extinguisher"], 0)
	return sc.castOffered(0, h)
}

// TestFixedFeeOnlyFilterSecondPip: Ashling, the Extinguisher ({2}{B}{B}) beside
// a Swamp and Bog Initiate's "{1}: Add {B}" is payable (a Mountain funds the
// filter's {1}: {B} from the Swamp, {B} from Bog Initiate). The fixed {B}
// production poses no colour ask, so gorge's feeOnlyGapDoomed prunes the
// activation by the pip test alone (no free-tap-colour cut, which the Swamp
// would trip). With no second {B} anywhere the cast stays unoffered.
func TestFixedFeeOnlyFilterSecondPip(t *testing.T) {
	for _, c := range []struct {
		board []string
		want  bool
	}{
		{[]string{"Swamp", "Mountain", "Mountain", "Forest", "Bog Initiate"}, true},
		{[]string{"Swamp", "Mountain", "Mountain", "Mountain", "Mountain", "Bog Initiate"}, true},
		{[]string{"Swamp", "Mountain", "Forest", "Bog Initiate"}, false},
		{[]string{"Mountain", "Mountain", "Mountain", "Forest", "Forest", "Bog Initiate"}, false},
		{[]string{"Swamp", "Swamp", "Mountain", "Mountain"}, true},
	} {
		if got := ashlingOffered(t, c.board...); got != c.want {
			t.Errorf("Ashling offered over %v = %v, want %v", c.board, got, c.want)
		}
	}
}
