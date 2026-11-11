package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CR 903.9a: a commander put into a graveyard or exile since the last
// state-based check may be put into the command zone by its owner. It is a
// state-based action with a choice, posed at the start of a pass (before the
// lose checks) and only while no other ask of the pass is outstanding. The
// owner's answer is DkChoose/ChCommanderZone: mode 0 moves it to the command
// zone, mode 1 leaves it where it is.

// cmdZoneDue reports whether seat p's commander in slot j sits in a
// graveyard or exile and has arrived there since the last ask (CR 903.9a's
// "put into that zone since the last time state-based actions were
// checked"). cmdAsk holds the object's ver at the last ask; a zone change
// bumps ver (CR 400.7), so a declined commander is not asked again until it
// moves.
func (s *State) cmdZoneDue(p uint8, j int) bool {
	o := s.cmdObj(p, j)
	if o == NoObj {
		return false
	}
	z := s.objs[o].zone
	return (z == kop.ZoneGraveyard || z == kop.ZoneExile) && s.objs[o].ver != *s.cmdAskAt(p, j)
}

// cmdZoneFirstDue returns the due commander (as ctlCmdZone0's offset: seat +
// 2*slot) that arrived first, as gorge asks in arrival order. The per-object
// stamp is the low 8 bits of the touch count at its last write: among a
// burst of moves (well under 256 object writes) the larger distance from the
// current count is the earlier arrival. A tie keeps seat order, then slot
// order.
func (s *State) cmdZoneFirstDue() (uint8, bool) {
	best, bestD, ok := uint8(0), uint8(0), false
	for j := 0; j < 2; j++ {
		for p := uint8(0); p < 2; p++ {
			if !s.cmdZoneDue(p, j) {
				continue
			}
			c := p + 2*uint8(j)
			d := uint8(s.touchN) - s.stamp[s.cmdObj(p, j)]
			if !ok || d > bestD || (d == bestD && c&1 < best&1) {
				best, bestD, ok = c, d, true
			}
		}
	}
	return best, ok
}

// cmdZoneAsk poses the pass-start CR 903.9a ask when it is owed and no other
// ask of the pass (a parked move or a queued as-enters ask) is outstanding.
// A legend ask already posed is ctlLegend, which the control-state switch
// below rejects; a legend duplicate merely owed this pass does not defer the
// commander ask (gorge's guard is legendBatch, parked only later in the
// pass). It reports whether it posed one; the sba loop then returns and Step
// builds the decision from the ctlCmdZone* state.
func (s *State) cmdZoneAsk() bool {
	if s.cfg&cfgCommander == 0 || s.over {
		return false
	}
	switch s.ctl {
	case ctlPriority, ctlStepBegin, ctlDivide:
		// the ordinary pass, and the combat damage pass (combat.go defers
		// its priority round and re-poses this ask after opening it)
	default:
		return false
	}
	if s.parkedWaiting() || s.g.nask != 0 {
		return false
	}
	c, ok := s.cmdZoneFirstDue()
	if !ok {
		return false
	}
	s.ctl = ctlCmdZone0 + c
	return true
}

// askCmdZone builds the owner's two-option decision about the commander c
// names (ctlCmdZone0's offset: seat + 2*slot) and records the asked object's
// version, so a decline is not re-asked until the card moves again.
func (s *State) askCmdZone(_ *prep, d *Decision, c uint8) {
	p, j := c&1, int(c>>1&1)
	o := s.cmdObj(p, j)
	*s.cmdAskAt(p, j) = s.objs[o].ver
	d.Kind, d.Sub, d.Player = DkChoose, ChCommanderZone, p
	d.add(MakeAction(AkChoose, uint8(o), 0, 0)) // put it into the command zone
	d.add(MakeAction(AkChoose, uint8(o), 0, 1)) // leave it
}

// applyCmdZone answers the ask: mode 0 moves the commander from its graveyard
// or exile to the command zone (a zone change, so ver bumps and it leaves the
// list); mode 1 leaves it. The SBA loop then runs again.
func (s *State) applyCmdZone(k *prep, c, mode uint8) {
	s.ctl = ctlPriority
	if mode != 0 {
		return
	}
	p, j := c&1, int(c>>1&1)
	o := s.cmdObj(p, j)
	if o == NoObj {
		return
	}
	z := s.objs[o].zone
	if (z != kop.ZoneGraveyard && z != kop.ZoneExile) || s.objs[o].ver != *s.cmdAskAt(p, j) {
		return
	}
	from := s.detach(o)
	s.placeFromK(k, o, kop.ZoneCommand, false, from, false)
}

// CR 903.9b: a commander that would be put into its owner's hand or library
// from anywhere may be put into the command zone instead. The move is parked
// before anything happens to the object (it stays in its zone, as gorge parks
// the MoveZone event), the owner is asked at once, and the answered move runs
// where it was going to happen, so the resolution continues after the answer.
// Graveyard and exile destinations are CR 903.9a's state-based action and are
// not replaced here.

// Ask kinds parked in genState.pre while the owner decides. preCmdZone is the
// wait; the answered states drive the re-run: Move lets the parked move happen,
// Cmd rewrites its destination to the command zone, and OK is the one move
// placeFromK then lets through (and clears).
const (
	preCmdZone     uint8 = 10
	preCmdZoneMove uint8 = 11
	preCmdZoneCmd  uint8 = 12
	preCmdZoneOK   uint8 = 13
)

// akCmdZone marks genState.pre as a commander-zone park (turn_gen.go's ask
// kinds).
const akCmdZone uint8 = 16

// cmdZoneCommander reports whether o is a commander of its owner's seat (either
// of a partner pair) and this is a Commander game: CR 903.9b asks only about
// the owner's commanders.
func (s *State) cmdZoneCommander(o Obj) bool {
	if s.cfg&cfgCommander == 0 || o == NoObj {
		return false
	}
	return s.cmdSlot(s.objs[o].owner(), o) >= 0
}

// cmdZoneAnswered reports whether o's parked move has its answer, not yet
// consumed by the move.
func (s *State) cmdZoneAnswered(o Obj) bool {
	p := &s.g.pre
	return p.kind == akCmdZone && p.obj == o && (p.state == preCmdZoneMove || p.state == preCmdZoneCmd)
}

// cmdZoneReplace is the CR 903.9b gate every zone change to a hand or a
// library passes before it touches the object. z and bottom are the move as
// the effect said it; true means the move must not happen now (it was parked,
// or a stall ended the game). An answered move: mode 1 leaves the destination
// as it was, mode 0 rewrites it to the command zone. The OK marker it leaves
// is what placeFromK consumes to let the move through.
//
// Outside a resolution the answer has no program to resume, so the guard
// stalls (StallLowering): a cost, an SBA or a cleanup that would move a
// commander to a hand or a library is not reachable from the five precons.
func (s *State) cmdZoneReplace(_ *prep, o Obj, z *kop.Zone, bottom *bool) bool {
	if s.cfg&cfgCommander == 0 || o == NoObj {
		return false
	}
	if p := &s.g.pre; p.kind == akCmdZone && p.obj == o {
		switch p.state {
		case preCmdZoneCmd:
			*z, *bottom = kop.ZoneCommand, false
			p.state = preCmdZoneOK
			return false
		case preCmdZoneMove:
			p.state = preCmdZoneOK
			return false
		case preCmdZoneOK:
			return false
		}
	}
	if (*z != kop.ZoneHand && *z != kop.ZoneLibrary) || !s.cmdZoneCommander(o) {
		return false
	}
	if !s.res.active || s.over || s.g.pre.state != 0 {
		s.stall(StallLowering)
		return true
	}
	s.g.pre = preAsk{obj: o, kind: akCmdZone, state: preCmdZone}
	s.res.ask, s.res.askP = raGenCmdZone, s.objs[o].owner()
	return true
}

// cmdZoneOptions poses the parked move's two-option ask, in the same order and
// with the same modes as the CR 903.9a ask: 0 into the command zone, 1 let it
// go to where the effect would put it.
func (s *State) cmdZoneOptions(d *Decision) {
	d.Sub = ChCommanderZone
	o := s.g.pre.obj
	d.add(MakeAction(AkChoose, uint8(o), 0, 0))
	d.add(MakeAction(AkChoose, uint8(o), 0, 1))
}

// cmdZoneAnswer records the owner's answer and runs the instruction that
// parked the move again, now with the answer consumed by cmdZoneReplace: the
// commander moves where it was going (or to the command zone) and the
// resolution continues after it. A second commander in the same sweep parks
// again on the same instruction and is asked next.
func (s *State) cmdZoneAnswer(k *prep, a Action) {
	p := &s.g.pre
	if p.kind != akCmdZone || p.state != preCmdZone {
		s.res.ask = raNone
		s.stall(StallLowering) // a hand-built decision with no parked move
		return
	}
	if a.Mode() == 0 {
		p.state = preCmdZoneCmd
	} else {
		p.state = preCmdZoneMove
	}
	s.res.ask = raNone
	in := &s.prog(k)[s.res.pc]
	switch in.Op {
	case kop.OpChangeZoneAll, kop.OpChangeZoneAllEx:
		if !s.execChangeZoneAll(k, in, true) {
			return // parked again
		}
	case kop.OpDigUntil:
		if !s.execDigUntil(k, in, true) {
			return
		}
	case kop.OpDigEx:
		if s.finishDigAt(k, true, false) {
			return
		}
	case kop.OpSearchEx, kop.OpChooseMove:
		s.finishSearchNow(k)
	default:
		s.exec(k, in)
	}
	if s.res.ask != raNone {
		return // parked again (or another ask)
	}
	s.continueRes(k, 0)
}
