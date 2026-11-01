package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// cfgxCantDraw: the table has a StatCantDraw static, so draw() consults
// playerBanned. Set once at Init (engine.go). Bit 6 is free in State.cfgx
// (state.go uses bits 0-2 and 7).
const cfgxCantDraw uint8 = 1 << 6

// DebugPlayerBanOff is TestPbanMutation's switch.
var DebugPlayerBanOff bool

// playerBanned: a functioning static of kind sk (StatCantGainLife /
// StatCantDraw, every player) is on the battlefield (gorge
// lifeGainForbidden / drawForbidden with an empty ValidPlayer$).
//
//go:noinline
func (s *State) playerBanned(k *prep, sk kop.StaticKind) bool {
	if DebugPlayerBanOff {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		if st.a.Kind == kop.AbStatic && st.a.Static.Kind == sk && s.statCond(k, st) {
			hit = true
			return false
		}
		return true
	})
	return hit
}
