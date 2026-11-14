package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// cfgxCantDraw: the table has a StatCantDraw static, so draw() consults
// playerBanned. Set once at Init (engine.go). Bit 6 is free in State.cfgx
// (state.go uses bits 0-2 and 7; repl_draw.go bit 5).
const cfgxCantDraw uint8 = 1 << 6

// DebugPlayerBanOff is TestPbanMutation's switch.
var DebugPlayerBanOff bool

// playerBanned: a functioning static of kind sk (StatCantGainLife /
// StatCantDraw) is on the battlefield and bans player p: every player, or with
// StBanOpp only the seats other than the static's controller (gorge
// lifeGainForbidden / drawForbidden: an empty ValidPlayer$, or Opponent).
//
//go:noinline
func (s *State) playerBanned(k *prep, sk kop.StaticKind, p uint8) bool {
	if DebugPlayerBanOff {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		if st.a.Kind == kop.AbStatic && st.a.Static.Kind == sk &&
			(st.a.Static.StFlags&kop.StBanOpp == 0 || st.ctrl != p) && s.statCond(k, st) {
			hit = true
			return false
		}
		return true
	})
	return hit
}
