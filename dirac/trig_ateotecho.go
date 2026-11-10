package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A copy token's own "At the beginning of the end step, sacrifice this token"
// (CopyPermanent | AtEOTTrig$ Sacrifice) is a triggered ability of the token
// (CR 707.2 copiable value; gorge trigger_granted.go
// checkGrantedAtEOTTriggers -> queueTrigger), so a Panharmonicon static whose
// filter matches the token makes it trigger an additional time (CR 603.2d;
// gorge statics_echo.go queueTriggerLimited). The kernel records it as a
// delayed record whose creator is the token itself; fireDelayed echoes it like
// a keyword trigger of the token's slot (panharmKeyword's shape). An AtEOT$
// rider is a delayed trigger sourced on its creator and is never echoed.

// ownTrig reports whether delayed record d is a copy token's own AtEOTTrig$
// sacrifice trigger (the only record whose creator is its affected object).
func (d *delayed) ownTrig() bool {
	return d.creator == d.obj && d.obj != NoObj && d.act == uint8(kop.OpSacrifice) && d.when == 0
}

// echoOwnDelayed echoes the pending entry at index m, just enqueued for
// delayed record d, once per matching Panharmonicon static when d is the
// token's own trigger and the token is still the same battlefield permanent.
func (s *State) echoOwnDelayed(k *prep, d *delayed, m uint8) {
	if !d.ownTrig() || k.gen == nil || !k.gen.anyPanharm || s.npend <= m {
		return
	}
	ob := &s.objs[d.obj]
	if ob.zone != kop.ZoneBattlefield || ob.ver != d.ver {
		return
	}
	sl := ob.slot
	if sl == NoSlot || !s.occ.has(sl) || s.perms[sl].obj != d.obj {
		return
	}
	p := &s.perms[sl]
	s.panharmEcho(k, trigListener{obj: d.obj, sl: sl, ctrl: p.ctrl, card: s.cardAt(sl)}, kop.TrigPhase, NoObj, p.ctrl, m)
}
