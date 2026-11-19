package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() { effects.RegisterNonAPI("repl:Proliferate") }

// proliferateReplacementMatches is the matcher arm of the proliferate-action
// replacement class (Tekuthal, Inquiry Dominus: "if you would proliferate,
// proliferate twice instead"). Only the synthetic PROPOSAL
// (effProliferate's CountReplacementProposed hold, the roll boundary's
// discipline) reaches here; the completed events.Proliferate markers the
// actions emit carry no proposal text and map no class, so a replacement
// applies before the action and never to its own record. The rewrite is the
// With's shared ReplaceCount$ arithmetic (applyProliferateReplacements), so
// only a With-bearing line matches: a With-less line has no implemented
// application and fails closed.
func (e *Engine) proliferateReplacementMatches(r cards.Repl, ev events.Event,
	you state.PlayerID, source state.ObjID, svars map[string]string) bool {
	if ev.Kind != events.Proliferate || ev.Text != events.ProliferateProposalText {
		return false
	}
	if r.With == nil {
		return false
	}
	if vp, ok := r.Param(cards.PKValidPlayer); ok &&
		!effects.MatchesPlayerSpec(e.G, vp, ev.Player, you) {
		return false
	}
	return e.replacementConditionHoldsWith(r, source, you, svars)
}

// applyProliferateReplacements rewrites a Proliferate proposal's action
// count through every applicable R:Event$ Proliferate replacement. The
// proposal (effProliferate's CountReplacementProposed hold, the roll
// boundary's discipline -- never logged) carries the instruction's action
// count in Amount; each matching body rewrites it in place through the ONE
// shared ReplaceCount$ arithmetic (Engine.replaceCount), whose VarName$ Num
// / ReplaceCount$Num/<op> form is exactly Tekuthal, Inquiry Dominus's
// "if you would proliferate, proliferate twice instead" (1 -> 2, CR
// 614.6/616). The rewritten count flows back through the returned event;
// every rewritten action still runs in full in effProliferate, each with
// its own choice round and its own completed events.Proliferate marker (CR
// 701.34a's per-action shape, the Amount$ 2 precedent).
//
// A body the shared arithmetic cannot price (a With whose VarName$ is not
// Num, or whose value is not a ReplaceCount$Num/<op> body) is skipped,
// never read as "no change": the cards layer keeps such shapes off the
// registered key's books only when they exist; a resolvable body always
// rewrites. CR 616.1e: each body reads the running count, so two
// duplicating bodies compose.
func (e *Engine) applyProliferateReplacements(ev events.Event, matches []replMatch) (events.Event, bool) {
	for _, m := range matches {
		if n, ok := e.replaceCount(m.id, m.repl, "Num", ev.Amount); ok {
			ev.Amount = n
		}
	}
	return ev, false
}
