package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Deferred mana sources (Pyromancer's Goggles, kop.Ability.ManaDeferred:
// "{T}: Add {R}. When that mana is spent to cast a red instant or sorcery
// spell, copy that spell ..."). gorge's payment planner defers a mana
// ability carrying TriggersWhenSpent$ (pay.PlanHasSpecialProductionParam), so
// its census never lists it: autopay never taps it (pcard.manaAb stays -1)
// and only a potential play's prefix script, or the manual mana window, does.
// The census has a gap there, relaxable (gorge paymentPlanRelaxedAlternatives
// gives the source a free {R} alternative), so the kernel treats it as a
// scripted-prefix gap of the restricted kind (scriptRestr returns it with
// them: gapMask gfRestrScript|gfDeferred) whose mana floats in the plain pool
// and not in State.rb.
//
// The mana is an artifact's, typed (artTag). A cast that spends the floating
// {R} is paid by the deferred source (gorge ManaSpentSources): the rider reads
// that through the observed-source record of the cast's build (obsSl,
// Tf2ManaFromSelf; mana_spendtrigger.go). genState.defSrc holds the source's
// slot + 1 while its {R} floats; pay compares the pool's {R} across the payment.

// floatDeferred adds the deferred source sl's {R} to p's pool and remembers
// the source (tapGap's deferred arm).
func (s *State) floatDeferred(k *prep, p uint8, sl Slot) {
	s.pl[p&1].pool[kop.R]++
	s.artTag(k, p, sl, kop.R)
	s.g.defSrc[p&1] = uint8(sl) + 1
}

// deferredPool is p's floating deferred {R} before a payment, 0 when none,
// and the source's slot + 1 (the payment's pool spend forgets it).
func (s *State) deferredPool(k *prep, p uint8) (r uint8, src uint8) {
	if k.gen == nil || !k.gen.anyDeferred || s.g.defSrc[p&1] == 0 {
		return 0, 0
	}
	return s.pl[p&1].pool[kop.R], s.g.defSrc[p&1]
}

// deferredSpent records the deferred source on the cast being built when the
// payment spent some of its {R} (r0 was the pool's before).
//
//go:noinline
func (s *State) deferredSpent(k *prep, p uint8, r0, src uint8) {
	if s.pl[p&1].pool[kop.R] >= r0 || !s.buildLive() || s.bld().e.kind != seSpell {
		return
	}
	sl := Slot(src - 1)
	if s.bld().obsSl != 0 && s.bld().obsSl != sl+1 {
		s.capFull(StallObsSrc)
		return
	}
	s.bld().obsSl = sl + 1
}

// scriptDeferred: p's activatable deferred sources, a slot mask (a subset of
// scriptRestr's).
func (s *State) scriptDeferred(k *prep, p uint8) slotMask {
	if k.gen == nil || !k.gen.anyDeferred {
		return slotMask{}
	}
	return s.gapMask(k, p, gfDeferred)
}
