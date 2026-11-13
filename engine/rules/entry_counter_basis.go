package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// enteringCounterBasisMatch is enteringObjectMatches' fallback for a
// battlefield entry judged by a FOREIGN Moved replacement's ValidCard$ (CR
// 614.12): the replacement applies to the permanent "as it would exist on the
// battlefield, taking into account replacement effects that have already
// modified how it enters". The plain fallback reads the stack object, which
// carries none of its intrinsic entry counters yet -- Authority of the Consuls'
// `ValidCard$ Creature.OppCtrl` then judges an impended Overlord of the
// Balemurk a creature (the TIME counters that make it not-a-creature are still
// only a grant) and taps it, and misses Kaito, Bane of Nightmares, whose
// `counters_GE1_LOYALTY`/PlayerTurn creature static cannot apply to a
// counterless stack card. The judgment is made on an entry preview instead:
// the move applied, the entry's intrinsic grants folded onto the entrant
// (loyalty, impending TIME, and any WithCountersType$ requests), and the
// preview's static memos re-read over the post-entry board (616.1f re-checks
// applicability after each such replacement). Forge's getReplacementList puts
// the entry counters on the LKI the same way.
//
// The preview is built with entryPreview -- the reusable, isolated one the
// entry-counter plan already uses -- and released before the match returns; no
// event kind, Engine field or Ctx joins the real resolution. Counters-first is
// a legal CR 616.1e order (and Kaito's only one). entryCounterGrants returns
// nil for an object already on the battlefield, so the previewed entrant can
// never recurse back into this func.
func enteringCounterBasisMatch(e *Engine, spec string, ev events.Event, sc effects.SpecContext) bool {
	fallback := func() bool { return e.matchesSpec(spec, ev.Obj, sc) }
	// Only a battlefield entry from another zone is an entry to judge; every
	// other move (and a battlefield stay) keeps the plain live-object matcher.
	if ev.Kind != events.MoveZone || ev.To != state.ZBattlefield || ev.From == state.ZBattlefield {
		return fallback()
	}
	// CR 708.5: a face-down entry's grants are not on its face either, and the
	// caller's AsFaceDown override describes a face the preview would not
	// show; the plain matcher already carries the override.
	if events.IsFaceDownEntry(ev.Counter) {
		return fallback()
	}
	// A self replacement's own gates read provenance (wasCastFromYourHandByYou
	// and friends) off the moving card itself, which the stack object carries
	// exactly; only a foreign judge needs the as-entered characteristics.
	if sc.Source == 0 || sc.Source == ev.Obj {
		return fallback()
	}
	grants := e.entryCounterGrants(&ev)
	if len(grants) == 0 {
		// Nothing the permanent would enter with: the stack object and the
		// as-entered permanent are the same for filter purposes.
		return fallback()
	}
	preview, entrant := e.entryPreview(ev)
	defer e.releaseEntryPreview(preview)
	// entryPreview folds the WithCountersType$ requests itself only when the
	// entry has foreign lent bodies (its CR 614.12 / 122.6 block); the
	// intrinsic grants -- always the head of the plan -- are folded here
	// either way, and the requests only when the preview did not already.
	_, requested := events.SplitEntryCounterPairs(ev.Pairs)
	intrinsic := grants
	if n := len(requested); n > 0 && n <= len(grants) {
		intrinsic = grants[:len(grants)-n]
	}
	for _, g := range intrinsic {
		events.Apply(preview.G, events.Event{Kind: events.CounterChange, Obj: entrant, Counter: g.Kind, Amount: g.Amount})
	}
	if len(requested) > 0 && len(preview.foreignEntryBodies(entrant, previewForeignEv(ev, entrant))) == 0 {
		for _, g := range requested {
			events.Apply(preview.G, events.Event{Kind: events.CounterChange, Obj: entrant, Counter: g.Kind, Amount: g.Amount})
		}
	}
	// 616.1f: the entrant's own statics (Kaito's loyalty-counter creature
	// switch) apply to the as-entered permanent; the preview's static and
	// derived-type memos are log-keyed, so they are re-read here.
	preview.refreshEntryPreviewStatics()
	// The caller's context carries the LIVE board's layer tables (withNames
	// bound them when the replacement's context was built): the entrant is
	// not in the live layer-4 table yet, so the match rebinds the preview's
	// own -- the seam withNames is for exactly this class of hand-built
	// context.
	sc = preview.withNames(sc)
	return preview.matchesSpec(spec, entrant, sc)
}
