package dirac

// abortCount keeps gorge's per-card abort count for object o, whose cast
// or activation is being reversed (rules cast_commit.go abortCast: the
// count survives a no-progress reversal of ANOTHER card, and a card's
// second no-progress abort holds its option out, CR 733.2). A reversal
// after progress (noProg false) starts a new window: every earlier count
// is stale, as gorge's emit clears suppressedCast and castAborts on a
// state change.
func (s *State) abortCount(o Obj, noProg bool) {
	if !noProg {
		for i := 0; i < int(s.nobj); i++ {
			if s.objs[i].of&(ofAbort1|ofHeld) != 0 {
				s.objs[i].of &^= ofAbort1 | ofHeld
				s.touch(Obj(i)) // ObjHash reads of (marks.go)
			}
		}
	}
	ob := &s.objs[o]
	if ob.of&ofAbort1 != 0 || noProg && s.heldObj == o {
		ob.of |= ofHeld
	}
	ob.of |= ofAbort1
	s.touch(o)
}

// ceaseManaToken removes a token sacrificed for mana, and notes it on a pushed cast
// being built for the reversal's no-progress window. A token sacrificed before the
// push (a Spree cast's mana window runs first) has ceased by the time the spell
// reaches the stack, so its reversal leaves nothing to end the window. It ceases to
// exist where it sits (CR 704.5d): no graveyard-leave trigger fires for it.
func (s *State) ceaseManaToken(o Obj) {
	s.ceaseQuiet(o)
	if s.buildLive() && s.bld().bfl&bflPushed != 0 {
		s.bld().bfl |= bflTokenGone
	}
}
