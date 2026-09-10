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
	// TriggerAmount is the magnitude the causing event carried -- the Damage
	// event's dealt-damage amount for a DamageDone/DamageDealtOnce trigger,
	// etc. It is what the TriggerCount$ heads (DamageAmount, LifeAmount,
	// Amount) answer: the value has to come from the event that fired the
	// trigger, so it is captured here exactly like the other provenance roles
	// and survives to resolution through the per-stack-instance
	// triggerContexts map. Zero when the causing event carried no amount.
	TriggerAmount int32
}

// controlReferent is the single classifier for the two-token ownership and
// control grammar. The Triggered* arms read event provenance; the Targeted*
// arms read only the targets of the resolving object. A provenance chain (>)
// remains unknown: state.Object has no spawner identity to dereference.
func controlReferent(p string) (op, ref string, ok bool) {
	op, ref, ok = strings.Cut(p, " ")
	if !ok || (op != "ControlledBy" && op != "OwnedBy") {
		return "", "", false
	}
	switch ref {
	case "TriggeredTarget", "TriggeredDefendingPlayer", "TriggeredPlayer", "TriggeredCard",
		"Targeted", "TargetedPlayer", "ThisTargetedPlayer", "TargetedController", "TargetedOrController":
		return op, ref, true
	}
	return "", "", false
}

// targetReferent is the shared classifier for TargetedPlayerCtrl and the
// matching arm. It must remain separate from controlReferent because this is
// a one-token predicate, not a ControlledBy/OwnedBy argument.
func targetReferent(p string) bool { return p == "TargetedPlayerCtrl" }

// controlReferentPlayers resolves the player or players a control/ownership
// referent names. A Targeted* referent is resolution-only: SpecContext has
// ResolutionTargets set only by effects.Ctx.SpecContext (or the resolution
// legality recheck), never while an offer is being built. Every absent target,
// gone object, or unsupported reference is unbound rather than guessed.
func controlReferentPlayers(g *state.Game, sc SpecContext, op, ref string) ([]state.PlayerID, bool) {
	if g == nil {
		return nil, false
	}
	var targets []state.Target
	switch ref {
	case "TriggeredTarget":
		targets = []state.Target{sc.TriggerTarget}
	case "TriggeredDefendingPlayer":
		targets = []state.Target{sc.DefendingPlayer}
	case "TriggeredPlayer":
		targets = []state.Target{sc.TriggerPlayer}
	case "TriggeredCard":
		targets = []state.Target{{Obj: sc.TriggerCard}}
	case "Targeted", "TargetedPlayer", "ThisTargetedPlayer", "TargetedController", "TargetedOrController":
		if !sc.Resolving {
			return nil, false
		}
		targets = sc.ResolutionTargets
	default:
		return nil, false
	}

	players := make([]state.PlayerID, 0, len(targets))
	for _, t := range targets {
		if t.IsPlayer {
			// TargetedController means the controller of an object target; a
			// player target is instead Targeted/TargetedPlayer (or the explicit
			// TargetedOrController union).
			if ref == "TargetedController" {
				continue
			}
			if int(t.Player) >= len(g.Players) {
				return nil, false
			}
			players = append(players, t.Player)
			continue
		}
		if t.Obj == 0 {
			return nil, false
		}
		obj := g.Obj(t.Obj)
		if obj == nil {
			return nil, false
		}
		// Targeted and TargetedPlayer name a player target, not an object's
		// controller. TargetedOrController explicitly admits both forms.
		if ref == "Targeted" || ref == "TargetedPlayer" || ref == "ThisTargetedPlayer" {
			continue
		}
		// TargetedController and TargetedOrController resolve the target's
		// controller to a PLAYER before ControlledBy/OwnedBy compares its
		// candidate. In particular, OwnedBy TargetedController means "owned
		// by that controller", not "owned by the targeted permanent's owner".
		if ref == "TargetedController" || ref == "TargetedOrController" {
			players = append(players, obj.Controller)
		} else if op == "OwnedBy" {
			players = append(players, obj.Owner)
		} else {
			players = append(players, obj.Controller)
		}
	}
	if len(players) == 0 {
		return nil, false
	}
	return players, true
}

// matchControlReferent returns unresolved as ok=false, including beneath '!':
// absence of a binding must never turn into a match by negation.
func matchControlReferent(g *state.Game, o *state.Object, sc SpecContext, op, ref string) (bool, bool) {
	players, ok := controlReferentPlayers(g, sc, op, ref)
	if !ok {
		return false, false
	}
	for _, p := range players {
		if (op == "OwnedBy" && o.Owner == p) || (op == "ControlledBy" && o.Controller == p) {
			return true, true
		}
	}
	return false, true
}

// matchTargetedPlayerCtrl is TargetedPlayerCtrl's resolution-only one-token
// form: the candidate is controlled by one of this resolution's PLAYER
// targets. It deliberately does not treat an object target's controller as a
// player target; TargetedOrController is the Forge spelling for that union.
func matchTargetedPlayerCtrl(g *state.Game, o *state.Object, sc SpecContext) (bool, bool) {
	players, ok := controlReferentPlayers(g, sc, "ControlledBy", "TargetedPlayer")
	if !ok {
		return false, false
	}
	for _, p := range players {
		if o.Controller == p {
			return true, true
		}
	}
	return false, true
}

// SpecContext binds a resolution's filter without adding a numeric resolver
// that the old MatchesSpecFrom call sites did not have. That grammar is
// independent of trigger provenance.
func (c *Ctx) SpecContext(you state.PlayerID) SpecContext {
	return SpecContext{You: you, Source: c.Source, TriggerContext: c.TriggerContext,
		ResolutionTargets: c.Targets, Resolving: true}
}
