package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// DebugRemTypesCount is TestVrefhMutation's switch: counts remembered
// objects instead of their distinct card types.
var DebugRemTypesCount bool

// rememberedTypes is Remembered$CardTypes (gorge count_ref_property.go:145,
// :398): distinct card types over the remembered objects' printed cards.
//
//go:noinline
func (s *State) rememberedTypes(k *prep) int {
	const cardTypes = kop.TLand | kop.TCreature | kop.TArtifact | kop.TEnchantment | kop.TPlaneswalker |
		kop.TInstant | kop.TSorcery | kop.TBattle | kop.TKindred
	var m kop.TypeMask
	n := 0
	for i := 0; i < int(s.g.nrem); i++ {
		o := s.g.rem[i]
		if o == NoObj {
			continue
		}
		m |= k.t.Cards[s.objs[o].card].Types & cardTypes
		n++
	}
	if DebugRemTypesCount {
		return n
	}
	return bits.OnesCount32(uint32(m))
}

// staticAttachedPT is Equipped$CardPower read by a static (gorge
// count_ref.go:622): the permanent the static's source is attached to, live
// P/T; 0, 0 when unattached or the source is not a permanent.
//
//go:noinline
func (s *State) staticAttachedPT(k *prep, ctx evalCtx) (int, int) {
	if ctx.srcSl == NoSlot {
		return 0, 0
	}
	a := s.perms[ctx.srcSl].attached
	if a == NoSlot {
		return 0, 0
	}
	return s.ptGen(k, a)
}
