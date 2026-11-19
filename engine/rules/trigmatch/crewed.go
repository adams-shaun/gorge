package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CrewedMatches implements Mode$ Crewed ("Whenever this creature crews a
// Vehicle ...", CR 702.122; Canyon Vaulter, Reckless Velocitaur, Veteran
// Motorist and the mode's 7 corpus carriers). It reads the events.Crew marker
// engine/rules/pay's Crew cost records: one event per crewing creature, Obj the
// CREWING creature and IDs[0] the Vehicle it crewed. Forge's ValidCrew$
// names the crewer (Card.Self on every carrier), so it is filtered against
// ev.Obj -- NOT ev.IDs[0] -- mirroring EnlistedMatches' ValidEnlisted$ vs
// ev.IDs[0] split.
func CrewedMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if ev.Kind != events.Crew {
		return false
	}
	ctrl := e.ControllerOf(source)
	if v := t.ParamStr(cards.PKValidCrew); v != "" && !e.MatchesSpec(v, ev.Obj, source, ctrl, SpecOpts{}) {
		return false
	}
	return true
}

// SaddledMatches implements Mode$ Saddled ("Whenever this creature saddles a
// Mount ...", CR 702.171; Canyon Vaulter, Reckless Velocitaur). It is the
// exact twin of CrewedMatches over the events.Saddle marker engine/rules/pay's
// Saddle cost records: Obj the SADDLING creature, IDs[0] the Mount. Forge's
// ValidCrew$ names the saddler, so it filters ev.Obj, exactly as the Crewed
// half does.
func SaddledMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if ev.Kind != events.Saddle {
		return false
	}
	ctrl := e.ControllerOf(source)
	if v := t.ParamStr(cards.PKValidCrew); v != "" && !e.MatchesSpec(v, ev.Obj, source, ctrl, SpecOpts{}) {
		return false
	}
	return true
}

// BecomesCrewedMatches implements Mode$ BecomesCrewed ("Whenever this Vehicle
// becomes crewed", CR 702.122; Mobilizer Mech, Ghost Ark, Mighty Servant of
// Leuk-o). It is the SUBJECT view of the crew action -- the Vehicle that was
// crewed, not the crewing creature (that is Mode$ Crewed's ValidCrew$
// subject above) -- and matches the events.CrewAction marker effects' Animate
// primitive emits once per crew ability RESOLUTION (CR 702.122e): Obj the
// VEHICLE, IDs the crewing creatures in tap order. One marker per resolution
// is what makes "crewed by three creatures" queue the trigger once, not once
// per crewer; the per-crewer Crew markers stay TriggerNone for the
// Creature.CrewedThisTurn filter.
//
// ValidCard$ names the Vehicle and is tested against ev.Obj (the
// becomeRenownedMatches shape; every corpus carrier spells Card.Self).
// ValidCrewAmount$ N gates on the action's crew count -- "if it was crewed by
// exactly two creatures" (Mighty Servant of Leuk-o); a bare integer is
// Forge's exact-count spelling, an <OP><N> expression goes through the
// CompareIntCount grammar. FirstTimeCrewed$ True ("for the first time each
// turn") is the per-turn latch over the same marker, evaluated by scanning
// the replayed log for an earlier CrewAction of this Vehicle since the last
// TurnChange -- the firstSaddledThisTurn contract, keyed by object rather
// than player; the current event is already logged when triggers match, so
// exactly one record means this is the first.
func BecomesCrewedMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if ev.Kind != events.CrewAction || ev.Obj == 0 {
		return false
	}
	ctrl := e.ControllerOf(source)
	if v := t.ParamStr(cards.PKValidCard); v != "" && !e.MatchesSpec(v, ev.Obj, source, ctrl, SpecOpts{}) {
		return false
	}
	if v := t.ParamStr(cards.PKValidCrewAmount); v != "" && !crewAmountMatches(int32(len(ev.IDs)), v) {
		return false
	}
	if ParamTrue(t, cards.PKFirstTimeCrewed) && !firstCrewedThisTurn(e, ev.Obj) {
		return false
	}
	return true
}

// crewAmountMatches reads ValidCrewAmount$'s count grammar: a bare integer is
// Forge's exact-count spelling ("crewed by exactly two creatures") and rides
// the shared grammar as its EQ form; anything else goes through the same
// <OP><N> grammar as written, failing closed on an expression it cannot
// evaluate (a gate the engine cannot evaluate must stay silent, never fire
// wide).
func crewAmountMatches(count int32, expr string) bool {
	if isIntLiteral(expr) {
		return CompareIntCount(count, "EQ"+expr)
	}
	return CompareIntCount(count, expr)
}

// isIntLiteral reports whether s is a non-empty run of ASCII digits.
func isIntLiteral(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// firstCrewedThisTurn is the BecomesCrewed FirstTimeCrewed$ latch: true only
// when the CrewAction being matched is this Vehicle's first in the current
// turn. It walks the log backwards to the TurnChange boundary, counting
// CrewAction markers for obj -- exactly one (the current event) means first.
// The walk also stops at obj's own last zone change, where the object it is
// now began (CR 400.7): a Vehicle crewed, bounced and recast this turn has
// not yet been crewed as the object it is now. Replay-stable: the log is the
// same on a rebuilt stream, so no mutable counter survives a Clone and no
// reset hook is needed.
func firstCrewedThisTurn(e Board, obj state.ObjID) bool {
	seen := false
	for i := len(e.Log().Events) - 1; i >= 0; i-- {
		ev := e.Log().Events[i]
		if ev.Kind == events.TurnChange {
			return seen
		}
		// An earlier zone change of obj made it a new object (CR 400.7).
		if ev.Kind == events.MoveZone && ev.Obj == obj {
			return seen
		}
		if ev.Kind != events.CrewAction || ev.Obj != obj {
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
	registerTrigMatcher(CrewedMatches, "Crewed")
	registerTrigMatcher(SaddledMatches, "Saddled")
	registerTrigMatcher(BecomesCrewedMatches, "BecomesCrewed")
	effects.RegisterNonAPI("trig:BecomesCrewed")
}
