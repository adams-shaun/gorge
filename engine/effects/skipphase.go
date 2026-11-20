package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() {
	Register("SkipPhase", effSkipPhase)
}

// effSkipPhase implements DB$ SkipPhase (Forge's SkipPhaseEffect; CR 614.10),
// in the shapes compileSkipPhaseParams models. The base one (False Peace,
// Empty City Ruse): the targeted (or Defined$) player skips every combat phase
// of their NEXT turn, extra combat phases included. The one-shot shapes (a
// Step$ Untap / Draw, a Phase$ BeginCombat with no Start$) skip the player's
// next such step once, with the scope in IDs[0] (state.SkipScope*). The grant is one events.SkipPhase per player, folded into
// state.Game.PhaseSkips; the turn structure (engine/rules/turn.go's
// advanceStep tail and extraPhaseBoundary) is the consumer, entering the
// postcombat main phase instead of the beginning of combat. "Next turn" is the
// next turn the player begins after this resolves, so a sorcery that targets
// its own caster skips the caster's following turn, not the current one.
//
// Any other shape is a loud Note and no grant, the AddPhase convention.
func effSkipPhase(h Host, c *Ctx, sa *cards.SA) {
	p := compileSkipPhaseParams(sa)
	if !p.ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "unimplemented SkipPhase shape: " + p.desc})
		return
	}
	players := []state.PlayerID{c.Controller}
	if DefinedRefOf(sa).Set() || TargetsOf(sa).Targeted() {
		players = nil
		for _, t := range Defined(h, c, sa) {
			if t.IsPlayer {
				players = append(players, t.Player)
			}
		}
	}
	g := h.Game()
	for _, pl := range players {
		if int(pl) >= 0 && int(pl) < len(g.Players) && !g.Players[pl].Lost {
			ev := events.Event{Kind: events.SkipPhase, Player: pl, Obj: c.Source, Step: p.step}
			if p.scope != 0 {
				ev.IDs = []state.ObjID{p.scope}
			}
			h.Emit(ev)
		}
	}
}
