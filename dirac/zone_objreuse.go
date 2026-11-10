package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Object id reuse. Two 100-card decks take 200 of the MaxObj ids, so a long
// token game fills the object table while most of its tokens have long since
// ceased to exist (CR 111.7, 704.5d). Once the table is full, newToken hands
// out the lowest id of such a token again. The rule is mirrored by the
// lockstep oracle's identity (oracle/project.go), so it reads only what the
// rules engine's own state shows too: a ceased token or copy that did not
// change zones this turn (it ceased in an earlier turn) and that no live
// record names. Below a full table nothing is reused, so a game that never
// fills it keeps every id it always had.

// objTableFull reports whether no object id can be minted: every id is in
// use and none is free for reuse.
func (s *State) objTableFull() bool {
	return int(s.nobj) >= MaxObj && s.freeTokenObj() == NoObj
}

// freeTokenObj is the lowest reusable object id, NoObj when there is none.
// Tokens and copies are the object table's suffix (ceaseTokensGen); reuse
// keeps it one, as only suffix ids are handed out again.
func (s *State) freeTokenObj() Obj {
	i0 := int(s.nobj)
	for i0 > 0 && s.objs[i0-1].of&(ofToken|ofCopy) != 0 {
		i0--
	}
	for i := i0; i < int(s.nobj); i++ {
		if s.objFree(Obj(i)) {
			return Obj(i)
		}
	}
	return NoObj
}

// objFree reports whether o is a token or copy that has ceased to exist in
// an earlier turn and that nothing names. ver 0 is a mint not placed in any
// zone yet (newToken's first id, a dungeon); zoneTurn is set on every
// placement and on a reuse, so an id handed out this turn is never handed
// out again before the turn ends.
func (s *State) objFree(o Obj) bool {
	ob := &s.objs[o]
	return ob.of&(ofToken|ofCopy) != 0 && ob.of&ofPending == 0 && ob.zone == kop.ZoneNone &&
		ob.ver != 0 && !s.g.zoneTurn.has(o) && !s.objReferenced(o)
}

// reuseObj clears the per-object marks a reused id must not inherit: a
// new object is not damaged this turn, has no mechanic marks and no
// departed-attachment record (CR 400.7).
func (s *State) reuseObj(o Obj) {
	s.g.zoneTurn.set(o)
	s.g.dmgTurn.clr(o)
	for i := range s.cold.mo {
		s.cold.mo[i].clr(o)
	}
	// a departed attachment's record names the old object; its host is
	// ver-checked (attLKIHost), the attachment itself is not
	for i := range s.g.attLKI {
		if r := &s.g.attLKI[i]; r.obj == o {
			r.obj, r.host, r.ver = NoObj, NoObj, 0
		}
	}
}

// heldReferenced reports whether a record that outlives a resolution names
// o: a delayed trigger, a dungeon, an emblem's source, an imprint, an
// until-leaves or exiled-with link, a lingering attachment, an overflow
// counter, or the as-enters ask in progress. A record that keeps the
// object's ver (a target, a may-play, a once-per-turn use, an LKI snapshot)
// cannot match a reused id, whose ver moves on (newToken).
func (s *State) heldReferenced(o Obj) bool {
	for i := 0; i < int(s.ndly); i++ {
		if d := &s.dly[i]; d.obj == o || d.creator == o {
			return true
		}
	}
	g := &s.g
	if g.pre.obj == o || g.replaced == o {
		return true
	}
	for p := range g.dObj {
		if g.room[p] != 0 && g.dObj[p] == o {
			return true
		}
	}
	for p := range g.nemb {
		for i := 0; i < int(g.nemb[p]) && i < maxEmbTotal; i++ {
			if s.embObj(uint8(p), i) == o {
				return true
			}
		}
	}
	for i := 0; i < int(g.nimp) && i < maxImp; i++ {
		if g.imp[i] == o {
			return true
		}
	}
	for i := 0; i < int(g.nlinks) && i < len(g.links); i++ {
		if g.links[i].src == o || g.links[i].obj == o {
			return true
		}
	}
	for i := 0; i < int(g.nexw) && i < len(s.cold.exw); i++ {
		if r := &s.cold.exw[i]; r.o == o || r.src == o {
			return true
		}
	}
	for i := 0; i < int(g.nlinger) && i < maxLinger; i++ {
		if r := &g.linger[i]; r.obj == o && g.lingerM.has(r.sl) { // lingerM is the record's liveness
			return true
		}
	}
	for i := 0; i < int(g.nctrOver) && i < maxCtrOver; i++ {
		if g.ctrOver[i].obj == o {
			return true
		}
	}
	return false
}
