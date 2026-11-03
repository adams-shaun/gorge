package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A token sacrificed for mana after a cast is pushed and then reversed ends the
// no-progress interval: gorge's token ceases at the state-based check after the
// reversal, which clears its abort count (rules emit, CR 733.2), so the card may be
// cast again. A reversal with no such token leaves the interval open, and so does one
// whose token was sacrificed before the push: it had already ceased by then (a
// Spree cast whose mode ask finds nothing payable, reversed twice, is held out).
func TestReversalAfterAManaTokenIsProgress(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Plains", Count: 8}})
	spell := sc.hand(id["Valley Floodcaller"], 0)
	reverse := func(token Obj, pushed bool) {
		sc.s.Step()
		sc.s.ctl = ctlBuild
		e := &sc.s.bld().e
		e.kind, e.obj, e.card, e.ctrl = seSpell, spell, id["Valley Floodcaller"], 0
		sc.s.bld().bfl = 0
		if pushed {
			sc.s.bld().bfl = bflPushed
		}
		if token != NoObj {
			sc.s.ceaseManaToken(token)
		}
		sc.s.bld().bfl &^= bflPushed // the spell is not on the stack in this scenario: no unpush
		sc.s.reverseCast(sc.k)
	}
	// a mana token sits in the graveyard when it ceases, as the real sacrifice leaves it
	reverse(NoObj, true)
	if !sc.s.noProgress() {
		t.Fatal("a plain reversal ended the no-progress interval")
	}
	reverse(sc.grave(id["Plains"], 0, 1)[0], false)
	if !sc.s.noProgress() {
		t.Fatal("a token sacrificed before the push ended the no-progress interval")
	}
	reverse(sc.grave(id["Plains"], 0, 1)[0], true)
	if sc.s.noProgress() {
		t.Fatal("a reversal after a mana token's sacrifice kept the no-progress interval open")
	}
}
