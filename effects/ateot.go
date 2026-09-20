package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// AtEOT$ is Forge's end-of-turn rider: "at the beginning of the next end
// step" the affected object leaves the battlefield (exiled, sacrificed,
// returned to hand, destroyed). It rides the same delayed-registration
// machinery dash/warp/encore and CopyPermanent already use -- one
// events.DelayedRegister per affected object, fired one-shot by
// rules.checkDelayedTriggers at the first end step reached after resolution,
// resolved through the builtin SVar bodies in cards/builtinSVars.
//
// The affected set is the primitive's own: Animate/Pump act on their
// per-object targets, Token on each mint, ChangeZone/ChangeZoneAll on the
// objects the move actually moved (some carriers RememberChanged$ and some
// do not, so the moved set is passed in rather than read back out of
// Remembered).
//
// TIMING READING (deferred nuance, stated here once): Forge distinguishes
// "at the beginning of YOUR next end step" (YourExile/YourSacrifice/
// YourExileUpkeep...) from the bare "the next end step". This build treats
// the Your-prefixed values as their plain equivalents -- the first end step
// reached after resolution -- because the corpus's carriers overwhelmingly
// resolve on their controller's own turn, where the two coincide. The
// divergence (a rider resolving on an opponent's turn would fire at that
// opponent's end step, a turn early) is recorded in the report.
//
// Values outside the implemented set stay LOUD: one Note per call naming the
// value, with the body still applied (the CopyPermanent convention). A value
// is never silently dropped.
func scheduleAtEOT(h Host, c *Ctx, sa *cards.SA, affected []state.ObjID) {
	if len(affected) == 0 {
		return
	}
	value := strings.TrimSpace(sa.Params["AtEOT"])
	if value == "" {
		return
	}
	// The in-scope value -> builtin body table. Reusing the existing
	// dash/warp/encore bodies gives each family its established semantics for
	// free: __kwWarpExile is incarnation-tracked (a copy that left the
	// battlefield and returned as a new incarnation is not exiled by a stale
	// promise), __kwEncoreSacrifice is not.
	body := ""
	switch value {
	case "Exile", "YourExile":
		body = "__kwWarpExile"
	case "Sacrifice", "YourSacrifice", "SacrificeCtrl":
		body = "__kwEncoreSacrifice"
	case "Hand":
		body = "__kwDashReturn"
	case "Destroy":
		body = "__kwAtEOTDestroy"
	}
	// RememberChanged$'s Ledger of remembered ids is not consulted: the
	// caller owns the moved set.
	for _, id := range affected {
		if h.Game().Obj(id) == nil {
			continue // a caller may hand over an id that never resolved
		}
		if body == "" {
			// Out of scope (Library shuffle-into, the end-of-combat and
			// next-upkeep families, an unrecognised spelling): one loud Note
			// per call, and the body still stands.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "AtEOT$ " + value + " is not implemented; the affected object stays on the battlefield (one Note per call)"})
			return
		}
		// The registration's Source IS the affected object, so the builtin
		// body's Defined$ Self resolves to it -- exactly the dash/warp/
		// encore shape. StepEnd is the first end step reached after
		// resolution (one-shot, removed by DelayedPush).
		h.Emit(events.Event{Kind: events.DelayedRegister, Obj: id,
			Player: c.Controller, Step: state.StepEnd, Counter: body})
	}
}

// atEOTInclude decides whether id is part of an AtEOT$-bearing body's affected
// set. For a `Defined$ Remembered` spec the engine seeds Ctx.Remembered with
// the trigger REFERENT (rules.triggerRemembered returns the event's object),
// which Forge's own card remembered list does not contain -- so a bare
// referent is not an affected object. The source is kept only when the card's
// own persistent event-backed list genuinely remembered it (a
// Self-remembering body), the rememberedWithSource convention
// Count$RememberedSize and the condition grammar already read.
func atEOTInclude(h Host, c *Ctx, sa *cards.SA, id state.ObjID) bool {
	if strings.TrimSpace(sa.Params["Defined"]) != "Remembered" || id != c.Source {
		return true
	}
	if o := h.Game().Obj(c.Source); o != nil {
		for _, t := range o.Remembered {
			if !t.IsPlayer && t.Obj == id {
				return true
			}
		}
	}
	return false
}
