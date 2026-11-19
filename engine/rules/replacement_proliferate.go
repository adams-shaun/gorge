package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
)

func init() { effects.RegisterNonAPI("repl:Proliferate") }

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
