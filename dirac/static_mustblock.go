package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Printed S:Mode$ MustBlock statics (kop.StatMustBlock, CR 509.1a; gorge
// combat.MustBlockCandidates in engine/rules/combat/block.go): a battlefield
// creature matching a live static's Filter, resolved with the static's
// controller and source, must block this combat if able. The duty rides the
// same mask and search as the Effect-delivered one (blockDuty's
// mechBlocksIfAble mark, mbSearch.duty, blockRequiredCore), so the offer walk
// and the requirement search need no change; only the mask grows.
//
// Like gorge's read the duty binds the creature that carries the static, its
// enchanted creature (Spirespine's Aura) or whatever else its ValidCreature$
// names; the shared gates (Condition$, IsPresent$) evaluate through statCond.
// gorge's candidate walk does not gate the printed static's condition but its
// pair check does (MustBlockPairRequired's StaticGateHolds); the corpus
// carries no gated MustBlock line, so the kernel evaluates the gate at both
// reads and stays fail-closed.

// useMustBlkStat arms blockDutyStatic: a table holding a StatMustBlock
// static takes the extended duty read.
var useMustBlkStat = newGenUse(func(t *kop.Table) bool { return tableHasStatic(t, kop.StatMustBlock) })

// blockDutyStatic is the defending player's permanents a live StatMustBlock
// static binds. Its own scan per slot, not staticFlagGen: an absent
// ValidCreature$ (gorge matches any creature) carries Filter 0, and
// matchPermGen's empty filter matches nothing.
func (s *State) blockDutyStatic(k *prep) slotMask {
	out := slotMask{}
	if k.gen == nil || !k.gen.uses.has(useMustBlkStat) {
		return out
	}
	for m := s.ctrlM[s.blockDef()]; m.any(); m = m.rest() {
		sl := m.first()
		if s.typesAt(k, sl)&kop.TCreature == 0 {
			continue
		}
		hit := false
		s.eachStatic(k, func(st statSrc) bool {
			a := st.a
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatMustBlock && s.statCond(k, st) &&
				(a.Static.Filter == 0 || s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerFull)) {
				hit = true
				return false
			}
			return true
		})
		if hit {
			out = out.with(sl)
		}
	}
	return out
}
