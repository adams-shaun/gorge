package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// payKindSacHost is the pay kind (execPay's kind, pickZ) of a granted
// trigger's own Sac<1/OriginalHost> cost (Sunfire Torch's "you may sacrifice
// this Equipment"). The instruction's C nibble is full, so the cost rides
// PaySacSelf with B.N = kop.PayBSubSacHost and execPay widens it to this value.
const payKindSacHost = 18

// trigHost is the permanent that granted a granted trigger, plus one (0 =
// none): Forge's OriginalHost, read by Sac<1/OriginalHost>. It rides remVer,
// which only a delayed trigger (a remembered object's version) or a spell (the
// keywords it gained) otherwise uses, so the pending trigger and the stack entry
// carry no byte for it.
func (e *stackEntry) trigHost() objWord {
	if e.kind != seTrigger || e.ab == abDelayed {
		return 0
	}
	return e.remVer
}

// sacHostSlot is the permanent that granted the resolving trigger, when the
// payer can sacrifice it: the Equipment or Aura recorded on the stack entry as
// the trigger was queued (gorge binds the cost part to the granting static's
// host, and a part names only what its payer controls, CR 701.21a). NoSlot
// when it has left the battlefield or changed controller, so the cost is not
// payable and only the decline is offered.
//
//go:noinline
func (s *State) sacHostSlot(p uint8) Slot {
	e := &s.res.e
	h := e.trigHost()
	if h == 0 {
		return NoSlot
	}
	ob := &s.objs[h-1]
	if ob.zone != kop.ZoneBattlefield || ob.card != e.card || s.perms[ob.slot].ctrl != p {
		return NoSlot
	}
	return ob.slot
}

// paySacHost pays kop.PayBSubSacHost: the granting host is sacrificed, the one
// candidate settled unasked (gorge's triggered-cost window).
//
//go:noinline
func (s *State) paySacHost(k *prep) {
	if sl := s.sacHostSlot(s.g.pickP); sl != NoSlot {
		s.fireSac(k, sl)
		s.leaveParked(k, sl)
	}
}
