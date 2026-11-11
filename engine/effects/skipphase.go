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
// in the one shape False Peace and Empty City Ruse carry: the targeted (or
// Defined$) player skips every combat phase of their NEXT turn, extra combat
// phases included. The grant is one events.SkipPhase per player, folded into
// state.Game.PhaseSkips; the turn structure (engine/rules/turn.go's
// advanceStep tail and extraPhaseBoundary) is the consumer, entering the
// postcombat main phase instead of the beginning of combat. "Next turn" is the
// next turn the player begins after this resolves, so a sorcery that targets
// its own caster skips the caster's following turn, not the current one.
//
// Any other shape (the one-shot "next combat phase this turn" of Moment of
// Silence, Blinding Angel's Defined$ form, Step$ Draw, ...) is a loud Note
// and no grant, the AddPhase convention.
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
			h.Emit(events.Event{Kind: events.SkipPhase, Player: pl, Obj: c.Source, Step: p.step})
		}
	}
}
