package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugDestroyHealOff is the mutation switch: a ReplDestroyHeal replacement
// is lowered but never applied, so a destroy takes its victim (gorge's
// Destroy ReplaceWith$ body does not run).
var DebugDestroyHealOff bool

// destroyHealSrc is the source of the first ReplDestroyHeal that matches
// victim sl: another creature its controller controls with a +1/+1 counter.
func (s *State) destroyHealSrc(k *prep, sl Slot) Slot {
	g := k.gen
	if g == nil || g.anyRepl&rbDestroyHeal == 0 || g.destroyHeal == nil {
		return NoSlot
	}
	for m := s.statM; m.any(); m = m.rest() {
		b := m.first()
		sc := s.cardAt(b)
		if g.replBits[sc]&rbDestroyHeal == 0 || s.genLost(k, b) || b == sl {
			continue
		}
		if s.matchPerm(k, g.destroyHeal[sc], sl, s.perms[b].ctrl, b) {
			return b
		}
	}
	return NoSlot
}

// destroyHeal replaces victim sl's destruction: remove all its +1/+1
// counters and heal all damage marked on it (Dr. McCoy, Surly Surgeon).
// It runs before regeneration and the shield counter (gorge effDestroy's
// ReplaceDestroyLine, then ReplaceDestruction and ReplaceShieldCounter).
//
//go:noinline
func (s *State) destroyHeal(k *prep, sl Slot) bool {
	if DebugDestroyHealOff || s.destroyHealSrc(k, sl) == NoSlot {
		return false
	}
	p := &s.perms[sl]
	s.touchSl(sl)
	if p.flags&pfDeathtouched != 0 {
		p.cseen |= cseenDT // gorge lists the removed mark at 0
	}
	p.seeCtr(s)
	p.subCtrN(s, int(kop.CtrP1P1), p.ctrN(s, int(kop.CtrP1P1)))
	p.damage = 0
	p.flags &^= pfDeathtouched | pfDtMore
	return true
}

// destroyHealAll is OpDestroyAll's destroy-heal pass (before regeneration):
// each victim that a ReplDestroyHeal replaces leaves hit.
func (s *State) destroyHealAll(k *prep, hit slotMask) slotMask {
	if k.gen == nil || k.gen.anyRepl&rbDestroyHeal == 0 {
		return hit
	}
	var out slotMask
	for m := hit; m.any(); m = m.rest() {
		sl := m.first()
		if !s.destroyHeal(k, sl) {
			out = out.with(sl)
		}
	}
	return out
}
