package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// DebugTrigCtrLive (TestTrigctrMutation): TriggeredCard$CardCounters reads
// the live object instead of the trigger's LKI snapshot, so 0 once it left
// the battlefield.
var DebugTrigCtrLive bool

// refNumColors is ExNumColorsOf: the colour count of
// the object r names. gorge count_ref_property.go CardNumColors reads
// len(h.ObjectColors(o)): layer-5 colours on the battlefield (the trigger's
// snapshot), the printed face elsewhere (CR 613.6: a spell on the stack, the
// SpellCast trigger's object; rules layers_derived.go objColors).
//
//go:noinline
func (s *State) refNumColors(k *prep, r kop.Ref, ctx evalCtx) int {
	var c kop.ColorMask
	e := ctx.e
	switch {
	case r == kop.RefTriggerObj && e != nil && e.li != 0:
		c = s.lk(e.li).col
	case r == kop.RefTriggerObj && e != nil && e.trigObj != NoObj:
		if ob := &s.objs[e.trigObj]; ob.zone == kop.ZoneBattlefield {
			c = s.colorsAt(k, ob.slot)
		} else if !(k.gen != nil && k.gen.anyFaceDown && s.faceDownAt(k, e.trigObj)) {
			c = k.t.Cards[ob.card].Colors // a face-down spell is colourless (CR 708.4)
		}
	default:
		return 0
	}
	return bits.OnesCount8(uint8(c & kop.ColAny))
}

// refChroma is ExChromaOf: the symbols of colour c in the printed cost of the
// triggering spell, none when it is face down (gorge chromaObjectCount).
//
//go:noinline
func (s *State) refChroma(k *prep, r kop.Ref, c int, ctx evalCtx) int {
	e := ctx.e
	if r != kop.RefTriggerObj || e == nil || e.trigObj == NoObj {
		return 0
	}
	ob := &s.objs[e.trigObj]
	if k.gen != nil && k.gen.anyFaceDown && s.faceDownAt(k, e.trigObj) {
		return 0
	}
	return k.t.Cards[ob.card].Cost.Chroma(c)
}
