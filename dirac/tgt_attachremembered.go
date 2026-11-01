package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// attachObj is OpAttachObj (gorge effAttach with an Object$ other than the
// source): the permanent in Ref C becomes attached to the permanent Obj;
// nothing when either is not on the battlefield (attachObjectGone, the
// destination walk's battlefield rule) or they are the same permanent.
func (s *State) attachObj(k *prep, in *kop.Instr) {
	src := s.objRef(k, kop.Ref(in.C))
	dst := s.objRef(k, in.Obj)
	if kop.Ref(in.C) == kop.RefRemembered && s.g.nrem > 1 {
		s.attachRemembered(k, dst) // tgt_attachremembered.go
		return
	}
	if dst == NoSlot {
		dst = s.attachPickSelf(k, s.res.pc) // tgt_attachpick.go
	}
	if src == NoSlot || dst == NoSlot || src == dst {
		return
	}
	s.attachTo(k, src, dst)
	s.touchSl(src)
}

// attachRemembered is OpAttachObj with Ref C = RefRemembered over several
// remembered objects (gorge attachObjectRemembered / rememberedAttachObjects:
// EVERY object of the resolution's remembered set is fastened to the
// destination, each by its own legality). The kgen guard admits the shape
// behind a single one-object writer, so a second entry only appears when a
// replacement multiplies the writer (a token doubler: one remembered Sturdy
// Shield per token made). An object no longer on the battlefield, or the
// destination itself, attaches nothing (attachObjectGone, CR 701.3b).
func (s *State) attachRemembered(k *prep, dst Slot) {
	if dst == NoSlot {
		return
	}
	for i := 0; i < int(s.g.nrem); i++ {
		o := s.g.rem[i]
		if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
			continue
		}
		src := s.objs[o].slot
		if src == dst {
			continue
		}
		s.attachTo(k, src, dst)
		s.touchSl(src)
	}
}
