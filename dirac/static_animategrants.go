package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// DebugAnimAddColorsOff is TestAnimgMutation's switch: an AddColors record adds nothing.
var DebugAnimAddColorsOff bool

// animAddColors is c with an Animate record's added colours (gorge
// animateGrant Colors$ without OverwriteColors$: layer 5 adds, CR 613.1e).
// A colourless object takes the colour alone, as the copy exceptions do.
func animAddColors(c kop.ColorMask, o *kop.Override) kop.ColorMask {
	if DebugAnimAddColorsOff {
		return c
	}
	return c&^kop.ColC | o.AddColors
}

// animLiveCtrl is the stored half of a live P/T record: gorge registers the
// granted static with the animating effect's controller (registerStaticEffectGrant
// Controller: c.Controller), so "you" in its value stays that player after a
// control change. The record's p carries that controller; its t is unused.
func animLiveCtrl(s *State) (p, t int16) {
	return int16(s.res.e.ctrl), 0
}

// LivePTStats counts liveP(T) reads.
var LivePTStats struct{ Reads atomic.Int64 }

// livePT evaluates the live P/T of override record o on the permanent at
// sl: the granted static's SetPower$/SetToughness$ values, read at every
// layer-7b check for the animating controller ctrl (gorge registers the
// static, so its values are re-evaluated, not fixed at resolution).
//
//go:noinline
func (s *State) livePT(k *prep, sl Slot, o *kop.Override, ctrl int16) (p, t int16) {
	LivePTStats.Reads.Add(1)
	pm := &s.perms[sl]
	ctx := evalCtx{src: pm.obj, srcSl: sl, ctrl: uint8(ctrl)}
	if !o.KeepP {
		p = int16(s.evalV(k, o.PA, ctx))
	}
	if !o.KeepT {
		t = int16(s.evalV(k, o.TA, ctx))
	}
	return
}

// livePTSrc evaluates the live P/T of static override record o carried by
// the permanent at src, from that permanent and its controller: gorge
// anchors a static's value on its source (an Equipment's "your life total"
// is its controller's), unlike a resolution-time Animate record, whose
// context is the affected permanent.
//
//go:noinline
func (s *State) livePTSrc(k *prep, src Slot, o *kop.Override) (p, t int16) {
	pm := &s.perms[src]
	ctx := evalCtx{src: pm.obj, srcSl: src, ctrl: pm.ctrl}
	if !o.KeepP {
		p = int16(s.evalV(k, o.PA, ctx))
	}
	if !o.KeepT {
		t = int16(s.evalV(k, o.TA, ctx))
	}
	return
}

// livePTLost reports that an effect making sl lose all abilities is newer than
// the Animate record applied at ts: the granted static is an ability, so the
// later layer-6 loss removes it and its set P/T no longer applies (CR 613.1f,
// 613.7; Beorn's Hospitality animated, then enchanted by Enchanted River's Grasp).
func (s *State) livePTLost(k *prep, sl Slot, ts uint16) bool {
	lt, lost := s.loss6(k, sl)
	return lost && lt > ts
}
