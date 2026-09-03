package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// Defined resolves a Defined$ parameter to concrete targets. With no Defined$
// the effect acts on the spell's chosen targets, which is Forge's default.
//
// Every return here is a defensive copy, never a slice sharing a backing
// array with Ctx.Targets or Ctx.Remembered: Ctx is threaded by pointer through
// Resolve, so a caller that filters the returned slice in place (the ordinary
// out := s[:0]; for range append(out, ...) idiom) must not be able to corrupt
// state a later effect in the same Sub chain still relies on.
func Defined(h Host, c *Ctx, sa *cards.SA) []state.Target {
	g := h.Game()
	switch sa.Params["Defined"] {
	case "":
		return copyTargets(c.Targets)
	case "You":
		return []state.Target{{Player: c.Controller, IsPlayer: true}}
	case "Self":
		return []state.Target{{Obj: c.Source}}
	case "Remembered":
		return copyTargets(c.Remembered)
	case "Targeted", "ParentTarget":
		return copyTargets(c.Targets)
	case "Opponent":
		var out []state.Target
		for _, p := range g.AliveFrom(c.Controller) {
			if p != c.Controller {
				out = append(out, state.Target{Player: p, IsPlayer: true})
			}
		}
		return out
	case "Player":
		var out []state.Target
		for _, p := range g.AliveFrom(c.Controller) {
			out = append(out, state.Target{Player: p, IsPlayer: true})
		}
		return out
	}
	// Any Defined$ form M1 does not model falls back to the chosen targets
	// rather than silently acting on nothing.
	return copyTargets(c.Targets)
}

// copyTargets returns a defensive copy of s: same elements, independent
// backing array. A nil s yields nil, not an empty-but-non-nil slice, so
// Defined's observable results are unchanged for every input — only the
// aliasing is fixed.
func copyTargets(s []state.Target) []state.Target {
	return append([]state.Target(nil), s...)
}

// PlayerOf resolves a target to a player: an explicit player target, or the
// controller of a targeted object.
func PlayerOf(h Host, c *Ctx, t state.Target) state.PlayerID {
	if t.IsPlayer {
		return t.Player
	}
	if o := h.Game().Obj(t.Obj); o != nil {
		return o.Controller
	}
	return c.Controller
}
