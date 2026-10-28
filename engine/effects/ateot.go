package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
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
// TIMING READING (deferred nuance, stated here once): for the END-STEP
// family Forge distinguishes "at the beginning of YOUR next end step"
// (YourExile/YourSacrifice) from the bare "the next end step"; this build
// treats the Your-prefixed values as their plain equivalents, because the
// corpus's carriers overwhelmingly resolve on their controller's own turn.
// The upkeep family honours Your (ValidPlayer$ You); the end-of-combat
// family fires at this turn's end-of-combat step only.
//
// Values outside the implemented set stay LOUD: one Note per call naming the
// value, with the body still applied (the CopyPermanent convention). A value
// is never silently dropped.
func scheduleAtEOT(h Host, c *Ctx, sa *cards.SA, affected []state.ObjID) {
	value := strings.TrimSpace(sa.ParamStr(cards.PKAtEOT))
	if value == "" {
		return
	}
	rider, ok := atEOTRiders.Get(value)
	if !ok {
		// Out of scope (Library shuffle-into, an unrecognised spelling): one
		// loud Note per call -- emitted even when the affected set is EMPTY,
		// so a body carrying an unimplemented value is never dropped
		// silently -- and the body still stands. cards.Primitives reports
		// the same value as unsupported to `make report`.
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "AtEOT$ " + value + " is not implemented; the affected object stays on the battlefield (one Note per call)"})
		return
	}
	scheduleAtEOTRider(h, c.Controller, c.Source, rider, affected)
}

// scheduleAtEOTRider registers one one-shot delayed promise per affected
// object. RememberChanged$'s Ledger of remembered ids is not consulted: the
// caller owns the moved set. The registration's Obj stays the AFFECTED
// object, because that is the builtin body's Defined$ Self referent (and the
// incarnation the stale-promise check tracks). CR 603.7d makes the creating
// permanent the delayed trigger's SOURCE, so a creator that differs from the
// affected object rides the Text trailer "|CR=<id>": rules pushes the stack
// object under it while the body still acts on the affected object. A creator
// equal to the affected object (dash/warp/encore and the self-target riders)
// emits exactly the event it did before.
func scheduleAtEOTRider(h Host, controller state.PlayerID, creator state.ObjID, r atEOTRider, affected []state.ObjID) {
	text := ""
	if r.thisTurn {
		// CR 511.2 / 603.7: an end-of-combat promise made after this turn's
		// combat has ended is not carried into a later turn's combat.
		text = "|TT=" + strconv.Itoa(int(h.Game().Turn))
	}
	if r.yours {
		text += "|VP=You"
	}
	for _, id := range affected {
		if h.Game().Obj(id) == nil {
			continue // a caller may hand over an id that never resolved
		}
		regText := text
		if creator != 0 && creator != id {
			regText += "|CR=" + strconv.FormatUint(uint64(creator), 10)
		}
		h.Emit(events.Event{Kind: events.DelayedRegister, Obj: id,
			Player: controller, Step: r.step, Counter: r.body, Text: regText})
	}
}

// atEOTRider is the registration one AtEOT$ value schedules: the builtin
// SVar body, the step it fires at, whether the step is gated to the
// registering player's own turn (ValidPlayer$ You), and whether the promise
// expires at the end of the turn it was made in.
type atEOTRider struct {
	body     string
	step     state.Step
	yours    bool
	thisTurn bool
}

// atEOTInclude protects the source from an AtEOT$ Remembered rider unless the
// source card's persistent remembered list actually contains it. The shared
// Defined$ resolver removes a trigger referent that Forge's card list does not
// remember; retain this guard as a defensive check for delayed riders.
func atEOTInclude(h Host, c *Ctx, sa *cards.SA, id state.ObjID) bool {
	if !DefinedRefOf(sa).Is(RefRemembered) || id != c.Source {
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

// atEOTRiders maps each AtEOT$ value to its rider. Reusing the existing
// dash/warp/encore bodies gives each family its established semantics:
// __kwWarpExile is incarnation-tracked (a card that left the battlefield and
// returned as a new incarnation is not exiled by a stale promise;
// engine/events/apply_delayed.go tracks the __kwWarp and __kwAtEOT prefixes),
// __kwDashReturn and __kwAtEOTDestroy likewise. __kwEncoreSacrifice is not
// tracked, the CopyPermanent convention. Every off-battlefield stale promise
// is a no-op at fire time: effSacrifice skips an object that is not on the
// battlefield and the exile/return bodies carry an Origin$ Battlefield guard.
// cards.atEOTValues must list exactly these keys.
var atEOTRiders = state.NewStrTable[atEOTRider](
	state.StrEntry[atEOTRider]{Key: "Exile", Val: atEOTRider{body: "__kwWarpExile", step: state.StepEnd}},
	state.StrEntry[atEOTRider]{Key: "YourExile", Val: atEOTRider{body: "__kwWarpExile", step: state.StepEnd}},
	state.StrEntry[atEOTRider]{Key: "Sacrifice", Val: atEOTRider{body: "__kwEncoreSacrifice", step: state.StepEnd}},
	state.StrEntry[atEOTRider]{Key: "YourSacrifice", Val: atEOTRider{body: "__kwEncoreSacrifice", step: state.StepEnd}},
	state.StrEntry[atEOTRider]{Key: "SacrificeCtrl", Val: atEOTRider{body: "__kwEncoreSacrifice", step: state.StepEnd}},
	state.StrEntry[atEOTRider]{Key: "Hand", Val: atEOTRider{body: "__kwDashReturn", step: state.StepEnd}},
	state.StrEntry[atEOTRider]{Key: "Destroy", Val: atEOTRider{body: "__kwAtEOTDestroy", step: state.StepEnd}},
	// End of combat (CR 511.2 step; Geist of Saint Traft's Angel token).
	state.StrEntry[atEOTRider]{Key: "ExileCombat", Val: atEOTRider{body: "__kwWarpExile", step: state.StepEndCombat, thisTurn: true}},
	state.StrEntry[atEOTRider]{Key: "SacrificeCombat", Val: atEOTRider{body: "__kwEncoreSacrifice", step: state.StepEndCombat, thisTurn: true}},
	// Next upkeep ("your" next upkeep for the Your-prefixed values).
	state.StrEntry[atEOTRider]{Key: "SacrificeUpkeep", Val: atEOTRider{body: "__kwEncoreSacrifice", step: state.StepUpkeep}},
	state.StrEntry[atEOTRider]{Key: "YourSacrificeUpkeep", Val: atEOTRider{body: "__kwEncoreSacrifice", step: state.StepUpkeep, yours: true}},
	state.StrEntry[atEOTRider]{Key: "YourExileUpkeep", Val: atEOTRider{body: "__kwWarpExile", step: state.StepUpkeep, yours: true}},
)
