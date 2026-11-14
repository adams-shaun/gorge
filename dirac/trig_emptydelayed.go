package dirac

// Empty-set delayed triggers, out of line of the interpreters. CR 603.7:
// a delayed trigger is created even when the set of objects it would affect
// is empty (gorge engine/effects/delayed_trigger.go effDelayedTrigger: a
// RememberObjects$ value "REPLACES that capture, including with an empty
// set"); at the next end step it goes on the stack and does nothing.

// delayEmptyGen registers a generated delayed program with no remembered
// object (fireDelayedGen enqueues it with objs[0] == NoObj). The caller
// checked maxDProg.
//
//go:noinline
func (s *State) delayEmptyGen(e *stackEntry, ab uint8) {
	d := dprog{card: e.card, ab: ab, ctrl: e.ctrl, src: e.obj, sv: s.objs[e.obj].ver, n: 1}
	d.objs[0] = NoObj
	s.g.dp[s.g.ndp] = d
	s.g.dpLKI[s.g.ndp] = e.li
	s.g.ndp++
}

// delayEmpty registers OpDelayedTrigger over no object: the source's own
// delayed trigger with act OpNop (resolveDelayed ignores it).
//
//go:noinline
func (s *State) delayEmpty(e *stackEntry) {
	if e.obj == NoObj {
		return
	}
	if s.ndly >= MaxDelayed {
		s.capFull(StallDelayed)
		return
	}
	s.dly[s.ndly] = delayed{ctrl: e.ctrl, obj: e.obj, ver: objWord(e.srcVer), creator: NoObj}
	s.ndly++
}
