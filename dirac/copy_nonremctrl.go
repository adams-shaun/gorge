package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Fractured Identity's "each player other than its controller creates a
// token that's a copy of it" (kop.CopyNonRemCtrl). Its old script spells the
// source Remembered with Controller$ NonRememberedController; the new one
// spells it Defined$ Targeted with Controller$ NonTargetedController. gorge
// resolves the owners as every living player from the resolving controller in
// turn order (AliveFrom) but the copied permanent's last-known controller
// (definedSpec NonRememberedController / NonTargetedController), sizes
// each owner's copies through the CreateToken replacements before the first
// one enters, and mints them owner by owner inside one zone batch
// (effCopyPermanent; CR 111.1, 603.6a). The kernel walks the same owners: the
// copies are made one owner at a time, every owner's replacements read from
// the permanents present when the walk began (tokFrz), and their enters
// checks held to the walk's end. A copy with an as-enters choice or an Aura
// copy's host parks the walk on its owner's ask; the parked token's owner is
// the walk's place, and the answer resumes it at the next owner
// (nonRemResume).

// DebugNonRemOneOpp is the mutation switch: the copies go to the next living
// seat after the remembered controller alone, the old two-seat reading.
// Tests only.
var DebugNonRemOneOpp bool

// nonRemExcl is the player who gets no copy: the copied permanent's
// controller. A remembered object reads its RememberLKI$ capture once it left
// the battlefield; a target (Fractured Identity's new Defined$ Targeted
// script) reads the last-known controller from the target snapshot, taken as
// the resolution exiled it (CR 608.2h, 400.7). A live battlefield object
// reads its current controller.
func (s *State) nonRemExcl(in *kop.Instr) uint8 {
	o := s.objRefObj(in.Obj)
	if o == NoObj {
		return noSeat
	}
	ctrl := s.objs[o].owner()
	if s.objs[o].zone == kop.ZoneBattlefield {
		ctrl = s.perms[s.objs[o].slot].ctrl
	} else if l := s.movedTargetLKIRec(o); l != nil {
		ctrl = l.ctrl
	} else if l := s.remLKIRec(o); l != nil {
		ctrl = l.ctrl
	}
	return ctrl
}

// copyNonRem makes card c's copies for each owner of the walk from the
// resolving controller from, starting after seat after (noSeat: from the
// first). True when a copy parked the walk on an ask.
func (s *State) copyNonRem(k *prep, in *kop.Instr, c CardID, from, after uint8) bool {
	e := &s.res.e
	excl := s.nonRemExcl(in)
	q := s.aliveFrom(from)
	if DebugNonRemOneOpp {
		q = seatOne(s.nextAlive(excl))
	}
	i := uint8(0)
	if after != noSeat {
		for i < q.n && q.s[i] != after {
			i++
		}
		i++
	} else {
		s.g.tokFrz, s.g.tokFrzOn = s.statM, true // copy_eachtarget.go tokSrcM
	}
	hr := s.holdEntries() // gorge's zone batch: the owners' copies enter together (CR 603.6a)
	saveLoop := s.g.inLoop
	s.g.inLoop = true
	defer func() {
		s.g.inLoop = saveLoop
		s.holdClose(k, hr)
		e.ctrl = from
	}()
	for ; i < q.n; i++ {
		if q.s[i] == excl {
			continue
		}
		e.ctrl = q.s[i]
		if s.copyCardTokenFor(k, in, c) {
			return true // the parked copy's owner is the walk's place (copyCtrl)
		}
		if s.g.stalled != 0 {
			break
		}
	}
	s.g.tokFrzOn = false
	return false
}

// nonRemResume goes on with a CopyNonRemCtrl walk once its parked copy is
// answered and that owner's copies are made: the owners after owner, from
// the resolving controller from. True when a later copy parks it again.
func (s *State) nonRemResume(k *prep, in *kop.Instr, from, owner uint8) bool {
	if in.Op != kop.OpCopyAsTokenEx || uint16(in.B.N)&kop.CopyNonRemCtrl == 0 {
		return false
	}
	c := s.objs[s.res.lastTok].card
	return s.copyNonRem(k, in, c, from, owner)
}
