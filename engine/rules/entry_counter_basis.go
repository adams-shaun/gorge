package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// entryBasisGate is the slice of the engine the cheap entry-basis gate reads:
// the entry's own counter grants and the registered continuous list (through
// its existing read-only accessor). Engine-needing logic is a free func over
// this interface; nothing is added to the Engine.
type entryBasisGate interface {
	entryCounterGrants(ev *events.Event) []events.EntryCounterGrant
	RegisteredContinuous() []ContinuousEffect
}

// entryBasisEngine adds to entryBasisGate what the full basis match needs:
// the spec matcher and the isolated entry preview with its release. *Engine
// satisfies it.
type entryBasisEngine interface {
	entryBasisGate
	matchesSpec(spec string, id state.ObjID, sc effects.SpecContext) bool
	entryPreview(ev events.Event) (*Engine, state.ObjID)
	releaseEntryPreview(preview *Engine)
}

var _ entryBasisEngine = (*Engine)(nil)

// enteringCounterBasisMatch is enteringObjectMatches' fallback for a
// battlefield entry judged by a FOREIGN Moved replacement's ValidCard$ (CR
// 614.12): the replacement applies to the permanent "as it would exist on the
// battlefield, taking into account replacement effects that have already
// modified how it enters". The plain fallback reads the object in its origin
// zone, which carries none of that: Authority of the Consuls' `ValidCard$
// Creature.OppCtrl` then judges an impended Overlord of the Balemurk a
// creature (the TIME counters that make it not-a-creature are still only a
// grant) and taps it, and misses Kaito, Bane of Nightmares, whose
// `counters_GE1_LOYALTY`/PlayerTurn creature static cannot apply to a
// counterless stack card, and taps Enduring Innocence returning through its
// own death trigger while the Animate static its return registered (AddType
// Enchantment, RemoveCardTypes) has not yet touched the origin-zone object.
// The judgment is made on an entry preview instead: the move applied, the
// entry's intrinsic grants folded onto the entrant (loyalty, impending TIME,
// and any WithCountersType$ requests), and the preview's static memos re-read
// over the post-entry board (616.1f re-checks applicability after each such
// replacement). Forge's getReplacementList puts the entry counters on the LKI
// the same way.
//
// The preview is built with entryPreview -- the reusable, isolated one the
// entry-counter plan already uses -- and released before the match returns; no
// event kind, Engine field or Ctx joins the real resolution. Counters-first is
// a legal CR 616.1e order (and Kaito's only one). entryCounterGrants returns
// nil for an object already on the battlefield, so the previewed entrant can
// never recurse back into this func.
func enteringCounterBasisMatch(e entryBasisEngine, spec string, ev events.Event, sc effects.SpecContext) bool {
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
	// and friends) off the moving card itself, which the origin-zone object
	// carries exactly; only a foreign judge needs the as-entered
	// characteristics.
	if sc.Source == 0 || sc.Source == ev.Obj {
		return fallback()
	}
	if !entryBasisDiffers(e, &ev) {
		// Nothing modifies how the permanent would exist on the battlefield:
		// the origin-zone object and the as-entered permanent are the same
		// for filter purposes.
		return fallback()
	}
	preview, entrant := e.entryPreview(ev)
	defer e.releaseEntryPreview(preview)
	// entryPreview folds the WithCountersType$ requests itself only when the
	// entry has foreign lent bodies (its CR 614.12 / 122.6 block). Whether it
	// did is decided once, here, on exactly the post-move board it decided
	// on -- before any intrinsic grant is folded -- so a lent rider whose
	// filter depends on the intrinsic counters (loyalty, TIME) cannot flip
	// the answer later, and the requested grants are never folded twice.
	_, requested := events.SplitEntryCounterPairs(ev.Pairs)
	previewFolded := len(requested) > 0 &&
		len(preview.foreignEntryBodies(entrant, previewForeignEv(ev, entrant))) > 0
	// The intrinsic grants -- always the head of the plan -- are folded here
	// either way, and the requests only when the preview did not already.
	intrinsic := e.entryCounterGrants(&ev)
	if n := len(requested); previewFolded && n > 0 && n <= len(intrinsic) {
		intrinsic = intrinsic[:len(intrinsic)-n]
	}
	for _, g := range intrinsic {
		events.Apply(preview.G, events.Event{Kind: events.CounterChange, Obj: entrant, Counter: g.Kind, Amount: g.Amount})
	}
	if len(requested) > 0 && !previewFolded {
		for _, g := range requested {
			events.Apply(preview.G, events.Event{Kind: events.CounterChange, Obj: entrant, Counter: g.Kind, Amount: g.Amount})
		}
	}
	// 616.1f: the entrant's own statics (Kaito's loyalty-counter creature
	// switch) and the as-enters static its moving effect already registered
	// (Enduring Innocence's Animate) apply to the as-entered permanent; the
	// preview's static and derived-type memos are log-keyed, so they are
	// re-read here.
	preview.refreshEntryPreviewStatics()
	// The caller's context carries the LIVE board's layer tables (withNames
	// bound them when the replacement's context was built): the entrant is
	// not in the live layer-4 table yet, so the match rebinds the preview's
	// own -- the seam withNames is for exactly this class of hand-built
	// context.
	sc = preview.withNames(sc)
	return preview.matchesSpec(spec, entrant, sc)
}

// entryBasisDiffers is the cheap gate before building the entry preview: it
// reads only state, and answers whether something is already modifying how
// the entering object would exist on the battlefield. Either half is
// sufficient; a foreign judge of an entry with neither sees the same object
// both ways.
//
//   - the entry's own counter grants (engine/rules/entry_counters.go's
//     entryCounterGrants): loyalty, impending TIME, the moving effect's
//     WithCountersType$ requests.
//   - a registered continuous effect sourced by the entrant itself: the
//     StaticEffect$ rider its moving effect registered just before the move
//     (effects.applyStaticEffect, Enduring Innocence's return Animate), or a
//     static an earlier effect granted the card. RegisteredContinuous is the
//     raw list, not e.active(): such a rider is stamped against the next
//     incarnation and only goes live once the move has folded the entry, so
//     active() filters it out exactly while the judgment is being made -- and
//     the preview's post-entry re-read is what turns it on, which is the CR
//     614.12 basis. A stale hit costs only the preview's work: the preview
//     re-evaluates the same liveness over its post-entry board, so it can
//     never see an effect the real entry would not apply.
func entryBasisDiffers(e entryBasisGate, ev *events.Event) bool {
	if len(e.entryCounterGrants(ev)) > 0 {
		return true
	}
	for _, ce := range e.RegisteredContinuous() {
		if ce.Source == ev.Obj {
			return true
		}
	}
	return false
}
