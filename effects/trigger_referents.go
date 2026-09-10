package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/state"
)

// TriggerContext is event provenance, not the resolving ability's Targets or
// Source. Rules captures it when a trigger fires and retains it through stack
// placement and suspension. A Target's IsPlayer bit distinguishes seat zero
// from an absent referent. All fields are zero outside a trigger.
//
// TriggerCard and TriggerPlayer are separate: a zone change's card, a phase's
// active player, a damage recipient and a damage source are not interchangeable.
// Unsupported/ambiguous event roles stay absent rather than guessing.
type TriggerContext struct {
	TriggerTarget   state.Target
	TriggerSource   state.ObjID
	DefendingPlayer state.Target
	TriggerPlayer   state.Target
	TriggerCard     state.ObjID
}

// controlReferent is the single shape classifier for both the matcher and
// UnknownPredicates. Provenance chains and self-referential Targeted* refs
// deliberately remain unknown (the next, separate grammar steps).
func controlReferent(p string) (op, ref string, ok bool) {
	op, ref, ok = strings.Cut(p, " ")
	if !ok || (op != "ControlledBy" && op != "OwnedBy") {
		return "", "", false
	}
	switch ref {
	case "TriggeredTarget", "TriggeredDefendingPlayer", "TriggeredPlayer", "TriggeredCard":
		return op, ref, true
	}
	return "", "", false
}

// matchControlReferent returns unresolved as ok=false, including beneath '!':
// absence of a binding must never turn into a match by negation.
func matchControlReferent(g *state.Game, o *state.Object, sc SpecContext, op, ref string) (bool, bool) {
	var t state.Target
	switch ref {
	case "TriggeredTarget":
		t = sc.TriggerTarget
	case "TriggeredDefendingPlayer":
		t = sc.DefendingPlayer
	case "TriggeredPlayer":
		t = sc.TriggerPlayer
	case "TriggeredCard":
		t.Obj = sc.TriggerCard
	}
	var p state.PlayerID
	if t.IsPlayer {
		p = t.Player
		if g == nil || int(p) >= len(g.Players) {
			return false, false
		}
	} else {
		if t.Obj == 0 || g == nil {
			return false, false
		}
		obj := g.Obj(t.Obj)
		if obj == nil {
			return false, false
		}
		p = obj.Controller
		if op == "OwnedBy" {
			p = obj.Owner
		}
	}
	if op == "OwnedBy" {
		return o.Owner == p, true
	}
	return o.Controller == p, true
}

// SpecContext binds a resolution's filter without adding a numeric resolver
// that the old MatchesSpecFrom call sites did not have. That grammar is
// independent of trigger provenance.
func (c *Ctx) SpecContext(you state.PlayerID) SpecContext {
	return SpecContext{You: you, Source: c.Source, TriggerContext: c.TriggerContext}
}
