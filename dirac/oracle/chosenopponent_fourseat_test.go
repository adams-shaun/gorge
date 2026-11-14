package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestChosenOpponentFourSeat: the four cards that choose an opponent as they
// enter (The Rack, Black Vise, Entropic Specter, Nyxathid) hold at any seat
// count. Each lowers with an enters program opening with the ExtChoosePlayer
// ask (the controller chooses among the living opponents, the answer kept in
// the permanent's chosen byte), not an in-place "the next seat" answer, and
// none is two-seat or pod-limited, so a three- or four-seat pod does not stall
// on StallTwoSeat at its start. The four-seat play itself (the ask posed, an
// answer past the first opponent) is TestFourSeatChosenOpponentLockstep under
// the dirac_mp tag.
func TestChosenOpponentFourSeat(t *testing.T) {
	set := chosenOppPool(t)
	for _, name := range chosenOppCards {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s is not in the chosenopp pool", name)
		}
		d := set.Def(i)
		asks := 0
		for ai := range d.Abilities {
			a := &d.Abilities[ai]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplEntersProgram {
				continue
			}
			for j := range a.Prog {
				if a.Prog[j].Ext() == kop.ExtChoosePlayer {
					asks++
				}
			}
		}
		if asks != 1 {
			t.Errorf("%s: %d ExtChoosePlayer instructions in its enters program, want 1", name, asks)
		}
		if kop.CardTwoSeat(d) || kop.CardPodLimited(d) {
			t.Errorf("%s: two-seat or pod-limited, a four-seat pod stalls on it at its start", name)
		}
	}
}
