package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// ActivationPhases$ is Forge's cast/activation WINDOW restriction: the
// phase or range a spell's SP$ or an ability's AB$ may be offered in
// ("Cast CARDNAME only before the combat damage step", "Activate only
// during your upkeep"). The ONE phase-name parser (state.ParsePhases) is
// already built for it -- engine/state/phase.go's doc names ActivationPhases$ as a
// consumer -- but until now nothing read the parameter, so every such card
// was offered in every step.
//
// The gate is a PURE READ, like activationLimitReached: the offer walks are
// pure reads and must never emit an event. An absent parameter is ungated
// (the same "absent Phase$ remains ungated" rule phaseGate follows). An
// UNRESOLVABLE spec fails CLOSED -- the spell/ability is withheld entirely,
// the same conservative direction phaseGate uses for an unknown trigger
// Phase$ name. Measured at the corpus pin, all 150 ActivationPhases$ lines
// resolve, so the fail-closed branch has zero live population.
//
// The three rider qualifiers on the same lines are read here too, because
// the window is meaningless without the most common ones: ActivationFirstCombat$
// and ActivationAfterBlockers$ narrow the window within combat, and
// PlayerTurn$/OpponentTurn$ name whose turn it must be (PlayerTurn$ was
// already read on the ability-offer loop, but not on the cast half; the
// other three were read nowhere).
//
// One helper serves every gate site -- the cast half through spellTimingOK
// (every zone which offers a spell cast) and the ability half through the
// AB$ offer loop and the mana-ability gate. A future cast-offer site that
// goes through spellTimingOK, as all of them already do, is covered by
// construction rather than by remembering to add a call.
func (e *Engine) activationPhasesOK(p state.PlayerID, sa *cards.SA) bool {
	if sa == nil {
		return true
	}
	// Every restriction below is keyed; an ability whose compiled set holds
	// none of the keys passes them all.
	if !sa.MayHaveAnyParam(activationPhaseKeys) {
		return true
	}
	params := sa.Params

	// ActivationPhases$ <spec>: the step set the offer is confined to. An
	// unresolvable element fails closed (withheld), never widened.
	if raw, ok := params["ActivationPhases"]; ok {
		spec := strings.TrimSpace(raw)
		if spec != "" {
			pp := e.parsedPhaseSpec(spec)
			if !pp.valid || pp.set.Empty() || !pp.set.Has(e.G.Step) {
				return false
			}
		}
	}

	// ActivationFirstCombat$ True: the offer exists only in the turn's FIRST
	// combat phase (Berserk's "before the combat damage step"). CombatsThisTurn
	// counts combat phases BEGUN this turn (state.Game.CombatsThisTurn, one
	// increment per BeginCombat entry), so the first combat -- and every step
	// before it, where an Upkeep->BeginCombat window also lives -- is <= 1.
	// The ConditionFirstCombat$ condition reads the same folded count. Only
	// "True" is a corpus shape; any other value is a restriction this build
	// cannot price and fails closed.
	if raw, ok := params["ActivationFirstCombat"]; ok {
		if !strings.EqualFold(strings.TrimSpace(raw), "True") || e.G.CombatsThisTurn > 1 {
			return false
		}
	}

	// ActivationAfterBlockers$ True: the offer exists only during combat
	// AFTER blockers are declared (trap_runner's "only during combat after
	// blockers are declared", always paired with
	// Declare Blockers->EndCombat). A priority window IN
	// StepDeclareBlockers is by construction after blockers are declared:
	// CR 509.1 makes declaring blockers a turn-based action at the START of
	// the step, and turn.go's StepDeclareBlockers arm asks it (e.askBlockers)
	// before it grants any priority -- e.priorityRound() runs only once
	// declarationMadeThisStep(events.DeclareBlockers) holds (CR 509.4, "then
	// the active player gets priority"). So the step itself must be
	// INCLUDED; comparing with <= excluded it and withheld the offer from
	// every window the paired range actually has. The step enum walks in
	// turn order, so a plain comparison clamps the set.
	if raw, ok := params["ActivationAfterBlockers"]; ok {
		if !strings.EqualFold(strings.TrimSpace(raw), "True") || e.G.Step < state.StepDeclareBlockers {
			return false
		}
	}

	// PlayerTurn$ / OpponentTurn$: whose turn the offer requires. PlayerTurn$
	// was read on the ability-offer loop alone (Wishclaw Talisman); reading
	// it here too covers the cast half and keeps one comparison for both.
	if raw, ok := params["PlayerTurn"]; ok {
		if !strings.EqualFold(strings.TrimSpace(raw), "True") || e.G.Active != p {
			return false
		}
	}
	if raw, ok := params["OpponentTurn"]; ok {
		if !strings.EqualFold(strings.TrimSpace(raw), "True") || e.G.Active == p {
			return false
		}
	}

	return true
}

// activationPhaseKeys are the keys activationPhasesOK reads.
var activationPhaseKeys = cards.ParamMaskOf(cards.PKActivationPhases, cards.PKActivationFirstCombat,
	cards.PKActivationAfterBlockers, cards.PKPlayerTurn, cards.PKOpponentTurn)

// grantedWindowOK is the granted/gained-ability offer's Activator$, window
// and ActivationZone$ gate: what the printed loop applies, one call so the offer
// walk's branch count does not grow (Necrotic Ooze's copy of Wu Longbowman's
// "only during your turn, before attackers are declared").
func grantedWindowOK(g grantedActivationGate, p state.PlayerID, id state.ObjID, ab *cards.SA, z state.Zone) bool {
	return grantedActivationAllows(g, p, id, ab) && abilityZoneOK(ab, z)
}
