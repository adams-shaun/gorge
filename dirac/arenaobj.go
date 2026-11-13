package dirac

// The object-width half of the mechanic-bit arena
// (arena.go).

// mObj reports whether mechanic m's mask holds o; false when the table does
// not use m (and for a hand-written table, which has no genPrep).
func (s *State) mObj(k *prep, m mech, o Obj) bool {
	if k.gen == nil {
		return false
	}
	i := k.gen.ix[m]
	return int(i) < len(s.cold.mo) && s.cold.mo[i].has(o)
}

// mObjSet sets o in mechanic m's mask (a no-op when the table does not use m).
func (s *State) mObjSet(k *prep, m mech, o Obj) {
	if k.gen == nil {
		return
	}
	if i := k.gen.ix[m]; int(i) < len(s.cold.mo) {
		s.cold.mo[i].set(o)
	}
}

// mObjClr clears o in mechanic m's mask.
func (s *State) mObjClr(k *prep, m mech, o Obj) {
	if k.gen == nil {
		return
	}
	if i := k.gen.ix[m]; int(i) < len(s.cold.mo) {
		s.cold.mo[i].clr(o)
	}
}

// mObjZero empties mechanic m's whole mask.
func (s *State) mObjZero(k *prep, m mech) {
	if k.gen == nil {
		return
	}
	if i := k.gen.ix[m]; int(i) < len(s.cold.mo) {
		s.cold.mo[i] = objBits{}
	}
}

// fbIx is mechFBLo's arena index, the hi plane's being the next one
// (arenaAssign assigns the pair together, lo first); arenaNone when the table
// grants no flashback. One ix load serves both planes.
func (k *prep) fbIx() uint8 {
	if k.gen == nil {
		return arenaNone
	}
	return k.gen.ix[mechFBLo]
}

// grantFB records a flashback grant on o until end of turn. Up to three
// grants are kept: gorge lists the keyword once per granting effect. The
// count is two bit planes (lo bit 0, hi bit 1); the fourth grant saturates
// at 3. The grant is part of o's view, so every caller marks o changed
// (touch).
func (s *State) grantFB(k *prep, o Obj) {
	i := k.fbIx()
	if int(i) >= len(s.cold.mo) {
		return
	}
	lo, hi := &s.cold.mo[i], &s.cold.mo[i+1]
	switch l, h := lo.has(o), hi.has(o); {
	case !l && !h: // 0 -> 1
		lo.set(o)
	case l && !h: // 1 -> 2
		lo.clr(o)
		hi.set(o)
	case !l && h: // 2 -> 3
		lo.set(o)
	}
	// both planes set is 3, and a further grant saturates there
}

// clearFB ends a granted flashback on o (a zone change or the cleanup).
// The grant is part of o's view: a caller that clears an actual grant must
// mark o changed. castFlagsGen is the exception to a bare touch: its clear
// may still be undone, so it also keeps heldTouch in step (noProgress).
func (s *State) clearFB(k *prep, o Obj) {
	if i := k.fbIx(); int(i) < len(s.cold.mo) {
		s.cold.mo[i].clr(o)
		s.cold.mo[i+1].clr(o)
	}
}

// fbGranted reports whether o holds any granted flashback.
func (s *State) fbGranted(k *prep, o Obj) bool {
	i := k.fbIx()
	return int(i) < len(s.cold.mo) && (s.cold.mo[i].has(o) || s.cold.mo[i+1].has(o))
}

// fbGrants is o's flashback grants (0 to 3): the two bit planes as a count.
func (s *State) fbGrants(k *prep, o Obj) uint8 {
	i := k.fbIx()
	if int(i) >= len(s.cold.mo) {
		return 0
	}
	var n uint8
	if s.cold.mo[i].has(o) {
		n |= 1
	}
	if s.cold.mo[i+1].has(o) {
		n |= 2
	}
	return n
}

// setFBGrants restores o's flashback grants (a cast undone, CR 733.1).
// The caller marks o changed (touch): the restored grants are part of its
// view.
func (s *State) setFBGrants(k *prep, o Obj, n uint8) {
	i := k.fbIx()
	if int(i) >= len(s.cold.mo) {
		return
	}
	if n > 3 {
		n = 3 // grantFB saturated at three; a restored count keeps that
	}
	lo, hi := &s.cold.mo[i], &s.cold.mo[i+1]
	lo.clr(o)
	hi.clr(o)
	if n&1 != 0 { // seat-ok: a count's low bit
		lo.set(o)
	}
	if n&2 != 0 {
		hi.set(o)
	}
}

// fbClearAll ends every granted flashback at the cleanup, marking each
// card whose grant ends changed first (its view lists the grant).
func (s *State) fbClearAll(k *prep) {
	if i := k.fbIx(); int(i) < len(s.cold.mo) {
		s.touchBits(&s.cold.mo[i])
		s.touchBits(&s.cold.mo[i+1])
		s.cold.mo[i] = objBits{}
		s.cold.mo[i+1] = objBits{}
	}
}
