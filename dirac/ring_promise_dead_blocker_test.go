package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRingPromiseFiresAfterBlockerDies: a registered level-3 promise fires at
// the end-of-combat step even when its blocker died after the registration
// (combat damage before end of combat): CR 603.7 keeps a delayed trigger that
// names an object that has since left the battlefield on the stack, resolving
// to nothing — gorge's fire-time scan (checkDelayedTriggers) only requires the
// source to still resolve, and its sacrifice body skips an object off the
// battlefield. The CR 400.7 half is resolveDelayed's own re-check, not a
// fire-time skip; a fire-time skip would drop the trigger and with it the
// CR 603.3b order ask two same-controller promises owe their controller.
func TestRingPromiseFiresAfterBlockerDies(t *testing.T) {
	sc, id := newRingBearerScen(t)
	b := sc.perm(id["Grizzly Bears"], 1)
	e := &stackEntry{trigObj: b, srcVer: sc.s.objs[b].ver, ctrl: 0, trigAmt: ringBlockAmt}
	sc.s.ringPromiseReg(sc.k, e)
	sl := sc.slot(b)
	sc.s.leave(sc.k, sl, kop.ZoneGraveyard, false)
	sc.s.fireRingPromises(sc.k)
	if sc.s.npend != 1 {
		t.Fatalf("queued %d pending triggers after the blocker died, want 1", sc.s.npend)
	}
	if got := sc.s.pend[0].ctrl; got != 0 {
		t.Fatalf("the fired promise's controller is seat %d, want the emblem owner 0", got)
	}
	if sc.s.objs[b].zone != kop.ZoneGraveyard {
		t.Fatalf("the dead blocker's zone is %v, want graveyard (the sacrifice no-ops at resolution)", sc.s.objs[b].zone)
	}
}
