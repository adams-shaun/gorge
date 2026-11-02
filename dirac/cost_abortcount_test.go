package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A token sacrificed for mana during a cast that is then reversed ends the no-progress
// interval: gorge's token ceases at the state-based check after the reversal, which
// clears its abort count (rules emit, CR 733.2), so the card may be cast again. A
// reversal with no such token leaves the interval open.
func TestReversalAfterAManaTokenIsProgress(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Plains", Count: 8}})
	spell := sc.hand(id["Valley Floodcaller"], 0)
	reverse := func(token Obj) {
		sc.s.Step()
		sc.s.ctl = ctlBuild
		e := &sc.s.bld().e
		e.kind, e.obj, e.card, e.ctrl = seSpell, spell, id["Valley Floodcaller"], 0
		sc.s.bld().bfl = 0
		if token != NoObj {
			sc.s.ceaseManaToken(token)
		}
		sc.s.reverseCast(sc.k)
	}
	reverse(NoObj)
	if !sc.s.noProgress() {
		t.Fatal("a plain reversal ended the no-progress interval")
	}
	reverse(sc.obj(id["Plains"], 0))
	if sc.s.noProgress() {
		t.Fatal("a reversal after a mana token's sacrifice kept the no-progress interval open")
	}
}
