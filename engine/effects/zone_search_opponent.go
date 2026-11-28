package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// zone_search_opponent.go is which opponent answers a library search ("An
// opponent chooses two of those cards": Realms Uncharted, Guided Passage). An
// effect that leaves a choice to "an opponent" lets the spell's controller say
// which one, announced while the effect is applied (CR 608.2d; the CR 801.5b
// example).

// beginSearchOwner opens one searched library's walk: it marks the walk's
// target index and, for a Chooser$ Opponent search with two or more live
// opponents, asks the controller which one answers, once. The returned params
// are a copy bound to that seat, so every searchChooser read of the walk (the
// confirmation, the zone election, the look, the picks, the search record)
// names it without asking again. With one live opponent nothing is asked and
// cz itself is returned.
func beginSearchOwner(h Host, c *Ctx, sa *cards.SA, cz *ChangeZoneParams, targetIndex int) *ChangeZoneParams {
	c.Search.Target = targetIndex
	if !cz.chooserOpponent {
		return cz
	}
	opponents, _ := knownDefinedTargets(h, c, cz.Chooser)
	g := h.Game()
	cands := make([]state.PlayerID, 0, len(opponents))
	for _, t := range opponents {
		if t.IsPlayer && int(t.Player) < len(g.Players) && !g.Players[t.Player].Lost {
			cands = append(cands, t.Player)
		}
	}
	if len(cands) < 2 {
		return cz
	}
	bound := *cz
	bound.searchAnswerer = uint8(askOpponentSeat(h, c, &decision.Decision{ResumeKind: "search_opponent",
		ResumeSA: sa, ResumeTarget: targetIndex, Prompt: "Choose an opponent to choose the cards"}, cands)) + 1
	return &bound
}

// askOpponentSeat asks the resolving controller to pick one of cands, the
// live opponents, as d (the caller's resume record and prompt): a KChoose of
// one "player" option each. The no-host stand-in and the fallback for a
// non-candidate answer are the first candidate -- the exact pick botpolicy's
// clamp answers (its "player" arm only overrides "vote").
func askOpponentSeat(h Host, c *Ctx, d *decision.Decision, cands []state.PlayerID) state.PlayerID {
	g := h.Game()
	d.Player, d.Kind, d.Min, d.Max, d.Source = c.Controller, decision.KChoose, 1, 1, c.Source
	for _, p := range cands {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options),
			Kind: "player", Label: g.SeatFacingName(p), Player: p})
	}
	if ans, ok := AskTape(h, d); ok && len(ans) > 0 {
		for _, p := range cands {
			if ans[0].Player == p {
				return p
			}
		}
	}
	return cands[0]
}
