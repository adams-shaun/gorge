package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// emptyDrawProposal reports whether ev is the mark of an attempted draw from
// an empty library (events.DrawEmpty, Amount 0; the Amount 1 clear the
// state-based check emits is not an attempt).
func emptyDrawProposal(ev events.Event) bool {
	return ev.Kind == events.DrawEmpty && ev.Amount == 0
}

// emptyDrawReplacement is the Draw-class line shape CR 614.11 lets meet an
// attempted draw from an empty library: a mandatory line whose body is not
// itself a draw (Laboratory Maniac's and Jace, Wielder of Mysteries' "you
// win the game instead"). A "draw N cards instead" body would draw from the
// same empty library, so those lines keep meeting only a draw that takes a
// card.
func emptyDrawReplacement(r *cards.Repl) bool {
	return r.With != nil && r.With.API != "Draw" && !r.OptionalValue()
}

// applyEmptyDrawReplacement applies the first live Draw replacement that
// meets p's attempted draw from an empty library (CR 614.11: "These effects
// are applied even if no cards could be drawn because there are no cards in
// the affected player's library"). It reports handled when one applied; the
// DrawEmpty mark is then not recorded, so the CR 704.5b loss never follows.
func (e *Engine) applyEmptyDrawReplacement(ev events.Event) (events.Event, bool) {
	var found *replMatch
	e.forEachReplacementSourceFor(replEventBits[cards.ReplDraw], func(id state.ObjID) {
		if found != nil {
			return
		}
		f := e.replacementFace(id, ev)
		if f == nil {
			return
		}
		for i := range f.Repls {
			r := &f.Repls[i]
			if replacementEventNameMatches(r.Event, "Draw") && emptyDrawReplacement(r) &&
				e.replacementMatches(*r, id, ev) {
				found = &replMatch{id: id, face: f, repl: r}
				return
			}
		}
	})
	if found == nil {
		return ev, false
	}
	return e.applyReplacement(ev, *found)
}
