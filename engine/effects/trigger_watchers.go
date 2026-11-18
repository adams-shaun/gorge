package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// AnyBattlefieldTriggerMode reports whether a battlefield permanent's face
// carries a trigger line of the given mode. It gates the marker events whose
// only reader is one trigger mode (events.Countered for Mode$ Countered,
// events.AbilityResolved for Mode$ AbilityResolves): a game with no such
// watcher emits no marker and so keeps its event chain.
func AnyBattlefieldTriggerMode(g *state.Game, mode cards.TriggerMode) bool {
	for p := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			for _, t := range o.Face().Triggers {
				if t.ModeKind() == mode {
					return true
				}
			}
		}
	}
	return false
}
