package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// Defined resolves a Defined$ parameter to concrete targets. With no Defined$
// the effect acts on the spell's chosen targets, which is Forge's default.
func Defined(h Host, c *Ctx, sa *cards.SA) []state.Target {
	g := h.Game()
	switch sa.Params["Defined"] {
	case "":
		return c.Targets
	case "You":
		return []state.Target{{Player: c.Controller, IsPlayer: true}}
	case "Self":
		return []state.Target{{Obj: c.Source}}
	case "Remembered":
		return c.Remembered
	case "Targeted", "ParentTarget":
		return c.Targets
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
	return c.Targets
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
