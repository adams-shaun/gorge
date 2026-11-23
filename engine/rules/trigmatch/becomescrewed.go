package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// BecomesCrewedMatches implements Mode$ BecomesCrewed (CR 702.122e:
// "Whenever [this Vehicle] becomes crewed" means "Whenever a crew ability of
// [this Vehicle] resolves"; Mindlink Mech, Mobilizer Mech, Ghost Ark, Mighty
// Servant of Leuk-o). It reads the events.AbilityResolved marker
// emitAbilityResolved emits beside the resolving ability's Resolve event: the
// event's Obj is the ability's source permanent (the Vehicle) and its Amount
// carries events.AbilityResolvedCrew when that ability is the `K:Crew`
// expansion's minted Animate SA. A countered crew ability never resolves, so
// it emits no marker and this matcher cannot fire for it.
//
// Forge's ValidCard$ names the Vehicle and is filtered against ev.Obj -- NOT
// the crewing creatures (that is Mode$ Crewed's ValidCrew$ subject).
//
// FirstTimeCrewed$ True (Mindlink Mech, Mighty Servant: "for the first time
// each turn") is the per-object latch firstCrewedThisTurn below.
//
// ValidCrewAmount$ N (Mighty Servant: "if it was crewed by exactly two
// creatures") counts the creatures that paid THIS crew ability. The events.Crew
// markers carry no activation id, so the association is the activation's own
// paid-tap list (Facts.PaidTaps, the same list resolution publishes as
// Ctx.Paid.Tapped); a crew cost is exactly its tap election, so that length is
// the number of events.Crew markers this activation recorded. Counting the
// markers for the whole turn would include an earlier activation whose crew
// ability was countered -- it paid its cost but never became crewed.
func BecomesCrewedMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if ev.Kind != events.AbilityResolved || ev.Amount&events.AbilityResolvedCrew == 0 {
		return false
	}
	ctrl := e.ControllerOf(source)
	if v := t.ParamStr(cards.PKValidCard); v != "" && !e.MatchesSpec(v, ev.Obj, source, ctrl, SpecOpts{}) {
		return false
	}
	if FirstTimeCrewed(t) && !firstCrewedThisTurn(e, ev.Obj) {
		return false
	}
	if v := t.ParamStr(cards.PKValidCrewAmount); v != "" {
		// ValidCrewAmount$ is a plain count ("2"), the equality Forge's
		// parameter carries; CompareIntCount supplies the integer parse and
		// comparison without a new strconv/strings call site.
		if !CompareIntCount(int32(e.Facts().PaidTaps), "EQ"+v) {
			return false
		}
	}
	return true
}

// FirstTimeCrewed reports Mode$ BecomesCrewed's FirstTimeCrewed$ True ("for
// the first time each turn"). It is a pure read of the trigger line; the
// per-turn latch it gates is firstCrewedThisTurn.
func FirstTimeCrewed(t cards.Trigger) bool {
	return ParamTrue(t, cards.PKFirstTimeCrewed)
}

// firstCrewedThisTurn is the FirstTimeCrewed$ latch: true only when the crew
// resolution being matched is this object's first in the current turn. It
// walks the log backwards to the TurnChange boundary, counting earlier crew
// AbilityResolved markers for obj -- exactly one (the current event, already
// logged when triggers match) means first. The walk also stops at obj's own
// last zone change, where the object it is now began (CR 400.7), so a Vehicle
// that leaves and returns is a new object and its next crew resolution is
// that object's first. Replay-stable: the log is the same on a rebuilt stream,
// so no mutable counter survives a Clone and no reset hook is needed.
func firstCrewedThisTurn(e Board, obj state.ObjID) bool {
	seen := false
	for i := len(e.Log().Events) - 1; i >= 0; i-- {
		ev := e.Log().Events[i]
		if ev.Kind == events.TurnChange {
			return seen
		}
		if ev.Kind == events.MoveZone && ev.Obj == obj {
			return seen
		}
		if ev.Kind != events.AbilityResolved || ev.Obj != obj || ev.Amount&events.AbilityResolvedCrew == 0 {
			continue
		}
		if seen {
			return false
		}
		seen = true
	}
	return seen
}

func init() {
	registerTrigMatcher(BecomesCrewedMatches, "BecomesCrewed")
}
