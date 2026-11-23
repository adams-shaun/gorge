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
// static binds.
func (s *State) blockDutyStatic(k *prep) slotMask {
	out := slotMask{}
	for m := s.ctrlM[s.blockDef()]; m.any(); m = m.rest() {
		sl := m.first()
		if s.typesAt(k, sl)&kop.TCreature != 0 && s.staticFlagGen(k, kop.StatMustBlock, sl) {
			out = out.with(sl)
		}
	}
	return out
}
