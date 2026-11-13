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

// entryAtEOT registers the sweep's next-end-step promise on entrant o. A
// sacrifice over several entrants is one delayed triggered ability naming all
// of them (gorge scheduleAtEOTGroup, CR 603.7): later is true for every
// entrant after the first, which joins the first one's trigger.
func (s *State) entryAtEOT(r *sweepRiders, o Obj, later bool) {
	if !r.eot || DebugAteotMoversSkip {
		return
	}
	if later && !r.eotExile && s.sacAtEndJoin(o, dlyUntracked) {
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
// __kwUnearth/__kwAtEOT/__kwMayFlashSac/__kwDecayed bodies). A record the
// affected object registered on itself (gorge's Creator == 0, no |CR=) names
// that object's incarnation at registration through Defined$ Self, so a card
// that left and returned is a new object and is not sacrificed (CR 603.7c,
// 400.7); the stale trigger is still put on the stack and does nothing. A
// creator-sourced record (a reanimation spell, a CopyPermanent token, a sweep)
// sacrifices its affected object by id, whichever incarnation is on the
// battlefield when it resolves: a card reanimated twice has the first promise
// sacrifice the second incarnation at once (even in response to its own
// reanimation, at the end step the promise fired in), and the second promise
// then finds it gone.
func (s *State) resolveSacUntracked(k *prep, e *stackEntry) {
	head := dlyRef(e.obj, e.trigObj)
	ob := &s.objs[head]
	var hit slotMask
	if ob.zone == kop.ZoneBattlefield && !s.sacBlocked(k, ob.slot) &&
		!(head == e.obj && ob.ver != e.srcVer) {
		hit = hit.with(ob.slot)
	}
	s.sacAtEndFollowers(k, head, hit)
}

// staleOrdered: a next-end-step action whose gorge body is incarnation-
// tracked (__kwWarpExile, __kwUnearth, __kwAtEOTDestroy; AtEOT$ Exile reuses
// the warp body, OpExile). Registered over a permanent that has since left,
// it still takes part in the end step's trigger order, is put on the stack and
// resolves doing nothing (CR 603.7, resolveDelayed's version check).
func staleOrdered(op kop.Op) bool {
	return op == opDashReturn || op == kop.OpWarpExile || op == kop.OpUnearth || op == kop.OpDestroy || op == kop.OpExile
}
