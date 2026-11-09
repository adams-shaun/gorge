package dirac

// Emblem durations that outlive the turn: Duration$ UntilTheEndOfYourNextTurn,
// AsLongAsInPlay and AsLongAsControl (kop.Emblem's flags). The first needs one
// bit per emblem in State.g.embF; the host-bound forms need no State of their
// own.

// embCompact removes player p's emblems whose index bit is in drop, compacting
// the per-player arrays and moving the UntilEndOfNextTurn flag bits with their
// survivors. maxEmb is 4, so one uint8 names every index.
func (s *State) embCompact(p, drop uint8) {
	bits := s.g.embF[p]
	n := 0
	var keep uint8
	for i := 0; i < int(s.g.nemb[p]); i++ {
		if drop&(1<<uint(i)) != 0 {
			continue
		}
		s.g.emb[p][n], s.g.embC[p][n], s.g.embO[p][n], s.g.embN[p][n] = s.g.emb[p][i], s.g.embC[p][i], s.g.embO[p][i], s.g.embN[p][i]
		s.g.embT[p][n] = s.g.embT[p][i]
		if bits&(1<<uint(i)) != 0 {
			keep |= 1 << uint(n)
		}
		n++
	}
	for i := n; i < int(s.g.nemb[p]); i++ {
		s.g.emb[p][i], s.g.embC[p][i], s.g.embO[p][i], s.g.embN[p][i] = 0, 0, 0, 0
		s.g.embT[p][i] = 0
	}
	s.g.nemb[p] = uint8(n)
	s.g.embF[p] = keep
}

// embTurnStart marks player p's UntilEndOfNextTurn emblems at p's turn start:
// their controller has begun a turn since their creation, so the cleanup of
// THIS turn drops them. This mirrors gorge's boundary:
// engine/rules/layers_effect.go:91 freezes UntilTurn = nextTurnFor(controller)
// for UntilTheEndOfYourNextTurn (without UntilYourNextTurn's preceding-cleanup
// -1), and EndOfTurnCleanup (:510) drops the registration when e.G.Turn
// reaches it -- the cleanup of that next turn, not its end step.
//
//go:noinline
func (s *State) embTurnStart(k *prep, p uint8) {
	for i := 0; i < int(s.g.nemb[p]); i++ {
		if k.t.Emblems[s.g.emb[p][i]-1].UntilEndOfNextTurn {
			s.g.embF[p] |= 1 << uint(i)
		}
	}
}

// dropEndOfNextTurnEmblems drops the UntilEndOfNextTurn emblems whose
// controller's next turn has begun (embF bit set) and is now at its cleanup.
// gorge's EndOfTurnCleanup does the same when e.G.Turn reaches UntilTurn
// (engine/rules/layers_effect.go:510); see embTurnStart for the boundary.
//
//go:noinline
func (s *State) dropEndOfNextTurnEmblems(k *prep) {
	for p := 0; p < 2; p++ {
		var drop uint8
		for i := 0; i < int(s.g.nemb[p]); i++ {
			if s.g.embF[p]&(1<<uint(i)) != 0 && k.t.Emblems[s.g.emb[p][i]-1].UntilEndOfNextTurn {
				drop |= 1 << uint(i)
			}
		}
		if drop != 0 {
			s.embCompact(uint8(p), drop)
		}
	}
}

// embCtrlDrop ends player p's AsLongAsControl emblems whose host is object o
// now that o's controller is to: the emblem's player no longer controls the
// host, so the effect has ended for good (CR 611.2b; gorge's
// expireAsLongAsControl), not merely stopped applying.
//
//go:noinline
func (s *State) embCtrlDrop(k *prep, o Obj, to uint8) {
	for p := 0; p < 2; p++ {
		var drop uint8
		for i := 0; i < int(s.g.nemb[p]); i++ {
			if k.t.Emblems[s.g.emb[p][i]-1].AsLongAsControl && s.g.embO[p][i] == o && uint8(p) != to {
				drop |= 1 << uint(i)
			}
		}
		if drop != 0 {
			s.embCompact(uint8(p), drop)
		}
	}
}
