package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// rememberAll is kop.ExtRememberAll (gorge effPumpAll's RememberPumped$):
// each battlefield permanent matching F joins the remembered set in gorge's
// walk order (seat by seat, battlefield order). A full set stalls
// (remember's StallRemember).
//
//go:noinline
func (s *State) rememberAll(k *prep, in *kop.Instr) {
	if in.C&kop.RememberAllImprint != 0 {
		// Copy the triggering object into the imprinted set (gorge Pump
		// ImprintCards$ TriggeredCard); a source that keeps its list records
		// it in the per-source store (zone_imprint.go imprint).
		if o := s.objRefObj(in.Obj); o != NoObj && (in.C&kop.RememberAllImprintCard == 0 || s.objs[o].of&ofToken == 0 && !DebugImprintRefOff) {
			s.imprint(o)
		}
		return
	}
	src := s.objRef(k, kop.RefSelf)
	var hit slotMask
	for m := s.occ; m.any(); m = m.rest() {
		if sl := m.first(); s.matchPerm(k, in.F, sl, s.res.e.ctrl, src) {
			hit = hit.with(sl)
		}
	}
	if in.C == kop.RememberAllObjOrder {
		s.rememberByObj(hit)
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(hit, &ord)
	for _, sl := range ord[:n] {
		s.remember(s.perms[sl].obj)
	}
}

// rememberByObj remembers the permanents in hit in object order, the order
// gorge walks g.Objs for a Defined$ AttachedTo selector (attachedToDefined
// Selector), which the control changes and the attaches after it follow.
func (s *State) rememberByObj(hit slotMask) {
	var objs [MaxPerm]Obj
	n := 0
	for m := hit; m.any(); m = m.rest() {
		o := s.perms[m.first()].obj
		i := n
		for ; i > 0 && objs[i-1] > o; i-- {
			objs[i] = objs[i-1]
		}
		objs[i] = o
		n++
	}
	for _, o := range objs[:n] {
		s.remember(o)
	}
}

// DebugImprintRefOff is TestPheliaMutation's switch: a Return's pre-move
// imprint (kop.RememberAllImprintCard) adds nothing. Never set outside a test.
var DebugImprintRefOff bool
