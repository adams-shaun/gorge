package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugBiteExcessOff is the lockstep mutation switch (test only): a
// kop.BiteExcessCtrl bite deals all its damage to the creature, as before
// ExcessDamage$ lowered.
var DebugBiteExcessOff bool

// biteHits adds OpBite's hits of source a's power pa on b to hb. Under
// kop.BiteExcessCtrl (DealDamage ExcessDamage$, Ram Through; gorge effects
// emitObjectDamageExcess) whose condition F matches the source, damage above
// what is lethal to b before the hit (excessLethalGen; 1 to a creature from a
// deathtouch source, CR 120.10, 702.2c) is dealt to b's controller instead:
// the creature hit, then the player hit, in the one batch.
func (s *State) biteHits(k *prep, in *kop.Instr, hb *hitBatch, a, b Slot, pa int) {
	if in.A.K != kop.VConst || in.A.N&kop.BiteExcessCtrl == 0 || DebugBiteExcessOff ||
		in.F != 0 && !s.matchPermGen(k, in.F, a, s.res.e.ctrl, NoSlot, layerFull) {
		hb.add(s, a, b, pa)
		return
	}
	lethal := max(s.excessLethalGen(k, b, s.kw(k, a)&kop.KwDeathtouch != 0), 0)
	if pa <= lethal {
		hb.add(s, a, b, pa)
		return
	}
	hb.add(s, a, b, lethal)
	if hb.n < len(hb.h) {
		hb.h[hb.n] = dmgHit{src: a, n: int16(min(pa-lethal, 1<<14)), pl: s.perms[b].ctrl + 1}
		hb.n++
	}
}
