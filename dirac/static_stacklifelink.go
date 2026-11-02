package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugStackLifelinkOff makes stackLifelinkKw grant nothing (mutation test).
var DebugStackLifelinkOff bool

// stackLifelinkKw: the resolving spell has lifelink from a live
// AffectedZone$ Stack grant its controller controls (gorge
// effects/damage.go:284 HasKeyword on the stack object; rules/
// layers_static.go stackSelfStaticOK for a carrier spell).
//
//go:noinline
func (s *State) stackLifelinkKw(k *prep) kop.KwMask {
	if DebugStackLifelinkOff {
		return 0
	}
	e := &s.res.e
	if n, _ := s.stackKwN(k, kop.StackKwLifelink, e.card, e.obj, e.ctrl, true); n > 0 {
		return kop.KwLifelink
	}
	return 0
}
