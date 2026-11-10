package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The many-object delayed return: OpReturn with kop.RetDelayedMany is the body
// of a DpMany program over a target slot of several objects exiled by the
// registering chain (Temporal Tribulation's "Return those cards to the
// battlefield", Hide on the Ceiling). gorge's one delayed trigger remembers the
// whole set and its ChangeZone returns it as one effect, so the objects come
// back in registration order as ONE entering event (CR 603.6a, 603.7): the walk
// is the remembered-set return (returnRemembered), run over the objects the
// entry still names. An object that changed zones since registration is a new
// one and is skipped (CR 400.7, 603.7c; gorge dropDelayedRememberedMoved).

// DebugDelayedManyFirstOnly is the mutation switch: the body returns only the
// first object the entry names.
var DebugDelayedManyFirstOnly bool

// delayedManyLive loads the objects the resolving entry names, in registration
// order, that are still the objects the program remembered into the remembered
// set (dpManyCarry puts objects 2..4 in the carriers, object 1 in trigObj).
func (s *State) delayedManyLive() {
	e := &s.res.e
	s.g.nrem = 0
	for i := 0; i < 4; i++ {
		var o Obj
		var ver uint8
		switch i {
		case 0:
			o, ver = e.trigObj, e.remVer
		case 1:
			o = Obj(e.trigAmt - 1)
			if e.trigAmt <= 0 {
				o = NoObj
			}
		case 2:
			o = Obj(e.x - 1)
			if e.x == 0 {
				o = NoObj
			}
		default:
			o = Obj(e.trigMV - 1)
			if e.trigMV == 0 {
				o = NoObj
			}
		}
		if i > 0 {
			ver = e.tver[i-1]
		}
		if o == NoObj || s.objs[o].ver != ver {
			continue
		}
		s.remember(o)
		if DebugDelayedManyFirstOnly {
			return
		}
	}
}

// returnDelayedMany runs the body: the live objects are the remembered set for
// the walk, and it reports a pending ask (a held entry parked on a choice),
// whose answer re-enters here with the walk's position kept.
//
//go:noinline
func (s *State) returnDelayedMany(k *prep, in *kop.Instr) bool {
	if s.g.retAt == 0 {
		s.delayedManyLive()
	}
	if s.returnRemembered(k, in) {
		return true
	}
	s.g.nrem, s.g.remCnt = 0, 0
	return false
}
