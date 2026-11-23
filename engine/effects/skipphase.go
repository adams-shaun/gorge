package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() {
	Register("SkipPhase", effSkipPhase)
}

// effSkipPhase implements DB$ SkipPhase (Forge's SkipPhaseEffect; CR 614.10).
// The shapes are skipPhaseParams': all combat phases of the next turn, the next
// combat phase, the next untap step and the next draw step, each for the
// defined or targeted player (the caster when neither). The grant is one
// events.SkipPhase per player, folded into state.Game.PhaseSkips; the turn
// structure (engine/rules/turn.go's beginTurn and advanceStep tail,
// skipphase_steps.go) is the consumer. A shape outside those is a loud Note
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
			h.Emit(events.Event{Kind: events.SkipPhase, Player: pl, Obj: c.Source, Step: p.step, Amount: p.amount})
		}
	}
}
