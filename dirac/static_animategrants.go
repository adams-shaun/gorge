package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

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

// livePT evaluates the live P/T of override record o on the permanent at
// sl: the granted static's SetPower$/SetToughness$ values, read at every
// layer-7b check for the animating controller ctrl (gorge registers the
// static, so its values are re-evaluated, not fixed at resolution).
//
//go:noinline
func (s *State) livePT(k *prep, sl Slot, o *kop.Override, ctrl int16) (p, t int16) {
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
