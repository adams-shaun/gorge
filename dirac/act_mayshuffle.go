package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Cold, out of line.

// DebugShuffleOwnOnly switches OpMayShuffleMove's shuffle to the effect
// controller's library alone (the Those Old Scientists mutation test: the
// lockstep must then diverge on a card shuffled from another player's
// graveyard).
var DebugShuffleOwnOnly bool

// parentTargetOK reports whether code is in the graveyard of the player
// named by t.ParentPlus1 (kgen's TargetsWithDefinedController$ ParentTarget:
// "target card in target player's graveyard"). The parent slot is a player
// target chosen earlier in the same cast/resolution entry.
//
//go:noinline
func (s *State) parentTargetOK(t *kop.TargetSpec, code TCode) bool {
	e := s.curEntry()
	if e == nil || t.Kind != kop.TgtCard {
		return false
	}
	if t.ParentPlus1 == kop.ParentTrigPl {
		// the trigger entry's trigger player (kop.ParentTrigPl): the
		// damaged seat of a damage trigger
		return e.kind == seTrigger && code != NoTarget && !isPlayerOrNone(code) && s.objs[code].owner() == e.trigPl
	}
	i := int(t.ParentPlus1) - 1
	if i < 0 || i >= kop.MaxTargets {
		return false
	}
	p := e.tgt[i]
	if !isPlayerTarget(p) {
		return false
	}
	if code == NoTarget || isPlayerOrNone(code) {
		return false
	}
	return s.objs[code].owner() == CodeSeat(p)
}

// execOblige runs the oblige ops (execCold7's default branch, cold). done=false
// means the op is not one of these.
//
//go:noinline
func (s *State) execOblige(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpMayShuffleMove:
		return s.mayShuffleMove(k, in), true
	case kop.OpIncubate:
		s.incubate(k, in)
		return true, true
	}
	if ok, done := s.execEarthbend(k, in); done { // act_earthbend.go
		return ok, true
	}
	return s.execDread(k, in) // face_manifest.go
}

// askOptionsOblige builds the oblige resolution asks' options; false: not one.
//
//go:noinline
func (s *State) askOptionsOblige(k *prep, d *Decision) bool {
	switch s.res.ask {
	case raGenMayShuffleMove:
		d.Sub = ChGenConfirm // gorge objectPathMayShuffle yes/no
		d.add(MakeAction(AkChoose, 0, 0, 1))
		d.add(MakeAction(AkChoose, 0, 0, 0))
	case raGenManifestDread:
		// gorge's manifest_dread options: the top-two window in library order
		d.Sub = ChGenManifestDread
		for i := 0; i < int(s.res.peekN); i++ {
			d.add(MakeAction(AkChoose, TCode(s.res.peek[i]), 0, uint8(i)))
		}
	default:
		return s.askOptionsModalAct(k, d)
	}
	return true
}

// applyAskOblige applies an answer to an oblige resolution ask; false: not one.
//
//go:noinline
func (s *State) applyAskOblige(k *prep, a Action) bool {
	switch s.res.ask {
	case raGenMayShuffleMove:
		if a.Mode() == 1 {
			in := &s.prog(k)[s.res.pc]
			var refs [maxXTgt + 1]Obj
			var legal [maxXTgt + 1]bool
			n := s.shuffleRefs(k, &s.res.e, uint8(in.C), &refs, &legal)
			s.doMayShuffleMove(&refs, &legal, n, s.res.e.ctrl)
		}
		s.continueRes(k, 0)
	case raGenManifestDread:
		s.res.ask = raNone
		chosen := Obj(a.Source())
		other := s.res.peek[1]
		if chosen == other {
			other = s.res.peek[0]
		}
		if s.dreadMove(k, chosen, other) {
			s.ctl = ctlExileOrder // the other card's CR 616.1 order ask (repl_exilemark.go)
			return true
		}
		s.res.left--
		if s.dreadAsk(k) {
			s.continueRes(k, 0)
		}
	default:
		return s.applyAskModalAct(k, a)
	}
	return true
}

// mayShuffleMove is OpMayShuffleMove: the player named by Obj (the effect's
// controller; RefNone when A.N=0) may (A.N=1, gorge objectPathMayShuffle) or
// must (A.N=0) put the legal chosen cards of target slot C on top of their
// owners' libraries in pick order, then shuffle each distinct owner's
// library once. A slot with no chosen card does nothing and asks nothing.
func (s *State) mayShuffleMove(k *prep, in *kop.Instr) bool {
	e := &s.res.e
	slot := uint8(in.C)
	var refs [maxXTgt + 1]Obj
	var legal [maxXTgt + 1]bool
	n := s.shuffleRefs(k, e, slot, &refs, &legal)
	if n == 0 {
		return true
	}
	if s.value(k, in.A) != 0 {
		asker := s.playerRef(k, in.Obj)
		if asker > 1 {
			asker = e.ctrl
		}
		s.res.ask, s.res.askP = raGenMayShuffleMove, asker
		return false
	}
	s.doMayShuffleMove(&refs, &legal, n, e.ctrl)
	return true
}

// shuffleRefs collects target slot's chosen object referents in target order
// (the primary target, then the extras) and whether each is still legal to
// move. A referent that has left the game is skipped: gorge's
// objectPathMayShuffle/objectPathShuffleOwners see nil. Origin$ Graveyard is a
// precondition of the move (gorge changeZoneMoveSkip), so a referent an earlier
// step of the same resolution took elsewhere (Rite of Renewal's cards returned
// to hand) is not legal to move, though its owner's library is still shuffled.
func (s *State) shuffleRefs(k *prep, e *stackEntry, slot uint8, refs *[maxXTgt + 1]Obj, legal *[maxXTgt + 1]bool) int {
	n := 0
	if slot >= kop.MaxTargets {
		return 0
	}
	if c := e.tgt[slot]; c != NoTarget && isObjTarget(c) && s.objs[c].zone != kop.ZoneNone {
		refs[n] = Obj(c)
		legal[n] = s.res.bad&(1<<slot) == 0 && s.objs[c].zone == kop.ZoneGraveyard
		n++
	}
	x := s.xtOf(e)
	for j := 0; j < int(x.n); j++ {
		if x.slot[j] != slot {
			continue
		}
		c := x.code[j]
		if c == NoTarget || isPlayerOrNone(c) || s.objs[c].zone == kop.ZoneNone {
			continue
		}
		refs[n] = Obj(c)
		legal[n] = s.xtLegal(k, e, j) && s.objs[c].zone == kop.ZoneGraveyard
		n++
	}
	return n
}

// doMayShuffleMove performs OpMayShuffleMove's moves and shuffle: each legal
// chosen card is detached and put on top of its owner's library (the first
// chosen ends on top, gorge libraryOrderPlacementAt), then each distinct
// owner's library is shuffled once in referent order (gorge
// objectPathShuffleOwners). Cards chosen but illegal are not moved; the
// shuffle still happens (CR 701.24c). ctrl is the effect controller, read only
// by DebugShuffleOwnOnly.
func (s *State) doMayShuffleMove(refs *[maxXTgt + 1]Obj, legal *[maxXTgt + 1]bool, nr int, ctrl uint8) {
	var moved [maxXTgt + 1]Obj
	var movedFrom [maxXTgt + 1]kop.Zone
	n := 0
	for i := 0; i < nr; i++ {
		if !legal[i] {
			continue
		}
		moved[n] = refs[i]
		n++
	}
	// The whole election moves its chosen cards as one event: a "one or more"
	// leave-your-graveyard trigger fires once for the batch, as gorge's
	// ChangeZone opens a zone batch around several targets (CR 603.2c).
	var old uint8
	if n > 1 {
		old = s.batchOpen()
	}
	for i := 0; i < n; i++ {
		movedFrom[i] = s.detach(moved[i])
	}
	if n > 1 {
		s.batchClose(old)
	}
	if n > 0 {
		s.libTopInOrder(moved[:n], movedFrom[:n])
	}
	var owners [MaxSeats]uint8
	no := 0
	for i := 0; i < nr; i++ {
		o := s.objs[refs[i]].owner()
		seen := false
		for j := 0; j < no; j++ {
			if owners[j] == o {
				seen = true
				break
			}
		}
		if !seen {
			owners[no] = o
			no++
		}
	}
	for i := 0; i < no; i++ {
		if DebugShuffleOwnOnly && owners[i] != ctrl {
			continue
		}
		s.shuffleLib(owners[i])
	}
}
