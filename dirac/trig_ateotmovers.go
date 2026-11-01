package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// AtEOT$ on a ChangeZoneAll sweep onto the battlefield.
//
// gorge's effChangeZoneAll ends with scheduleAtEOT over the moved set: one
// next-end-step promise (sacrifice or exile) per object that moved. The
// kernel registers each entrant as it enters, through the sweep's MvEntryRiders
// hook (kop.EntryAtEOT in the instruction's A, decoded by sweepRidersOf), so
// the promise set is exactly the entrants.

// DebugAteotMoversSkip skips the sweep's per-entrant registration; the
// lockstep mutation test flips it and requires a divergence.
var DebugAteotMoversSkip bool

// entryAtEOT registers the sweep's next-end-step promise on entrant o.
func (s *State) entryAtEOT(r *sweepRiders, o Obj) {
	if !r.eot || DebugAteotMoversSkip {
		return
	}
	s.atEOTRider(o, r.eotExile)
}

// decodeEntryAtEOT reads the AtEOT bits out of a sweep's A and returns the
// counter word without them.
func decodeEntryAtEOT(a int16) (word uint16, eot, exile bool) {
	w := uint16(a)
	return w &^ (kop.EntryAtEOT | kop.EntryAtEOTExile), w&kop.EntryAtEOT != 0, w&kop.EntryAtEOTExile != 0
}

// opSacUntracked is the stack entry's trigAmt for a fired dlyUntracked
// record (kernel-internal, beside opLeaveReturn; never in an op dispatch).
const opSacUntracked = 231

// delayedAmt is the action a fired next-end-step record is enqueued with.
func delayedAmt(d delayed) int16 {
	if d.when == dlyUntracked {
		return opSacUntracked
	}
	return int16(d.act)
}

// resolveSacUntracked: gorge's __kwEncoreSacrifice (AtEOT$ Sacrifice) is
// untracked (events foldDelayedRegister tracks only the __kwDash/__kwWarp/
// __kwUnearth/__kwAtEOT/__kwMayFlashSac/__kwDecayed bodies). It sacrifices
// its object by id, whichever incarnation is on the battlefield when it
// resolves: a card reanimated twice has the first promise sacrifice the
// second incarnation at once (even in response to its own reanimation, at the
// end step the promise fired in), and the second promise then finds it gone.
func (s *State) resolveSacUntracked(k *prep, e *stackEntry) {
	ob := &s.objs[dlyRef(e.obj, e.trigObj)]
	if ob.zone != kop.ZoneBattlefield {
		return
	}
	sl := ob.slot
	s.fireSac(k, sl)
	s.leaveParked(k, sl)
}

// staleOrdered: a next-end-step action whose gorge body is incarnation-
// tracked (__kwWarpExile, __kwUnearth, __kwAtEOTDestroy; AtEOT$ Exile reuses
// the warp body, OpExile). Registered over a permanent that has since left,
// it still takes part in the end step's trigger order and foldDelayedPush
// then mints no trigger (takePending drops it).
func staleOrdered(op kop.Op) bool {
	return op == kop.OpWarpExile || op == kop.OpUnearth || op == kop.OpDestroy || op == kop.OpExile
}
