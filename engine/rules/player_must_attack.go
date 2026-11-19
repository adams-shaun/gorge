package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// stat:PlayerMustAttack is the printed S:Mode$ PlayerMustAttack static
// (Seeker of Slaanesh: "Each opponent must attack with at least one creature
// each combat if able"; Trove of Temptation: "... must attack you or a
// planeswalker you control with at least one creature each combat if able").
// It is a duty of the PLAYER, not of any creature, so it cannot ride
// Option.Required (that marks creatures that must ALL attack): the attackers
// decision marks the pairs that discharge it (Option.AttackOne) and
// decision.AttackOneSatisfied is the one rule Validate, FitRequired and the
// engine's declaration check share.
//
// CR 508.1d reads "if able": a pair that costs anything to declare never
// discharges the duty (attack costs are never required), and a duty no
// offered pair can discharge binds nothing. Only the two corpus shapes are
// read -- no MustAttack$ (any attack discharges) and MustAttack$
// "You,Planeswalker.YouCtrl" (an attack on the static's controller or on a
// planeswalker they control); any other parameter fails closed to no duty.
func init() { effects.RegisterNonAPI("stat:PlayerMustAttack") }

const playerMustAttackYouOrWalker = "You,Planeswalker.YouCtrl"

// playerMustAttackUnit is one live duty binding the attacking player: any
// attack discharges it (anyDefender), or only an attack on controller or on a
// planeswalker controller controls.
type playerMustAttackUnit struct {
	controller  state.PlayerID
	anyDefender bool
}

// playerMustAttackUnits collects the readable PlayerMustAttack statics that
// bind player p (ValidPlayer$ read relative to each static's controller).
func playerMustAttackUnits(e *Engine, p state.PlayerID) []playerMustAttackUnit {
	var out []playerMustAttackUnit
	for _, sv := range e.activeStatics("PlayerMustAttack") {
		readable := true
		for k := range sv.Params { // membership test only
			switch k {
			case "Mode", "ValidPlayer", "MustAttack", "Description":
			default:
				readable = false
			}
		}
		if !readable {
			continue
		}
		if spec := sv.ParamStr(cards.PKValidPlayer); spec == "" || !effects.MatchesPlayerSpec(e.G, spec, p, sv.Controller) {
			continue
		}
		switch sv.ParamStr(cards.PKMustAttack) {
		case "":
			out = append(out, playerMustAttackUnit{controller: sv.Controller, anyDefender: true})
		case playerMustAttackYouOrWalker:
			out = append(out, playerMustAttackUnit{controller: sv.Controller})
		}
	}
	return out
}

// discharges reports whether attacking def (and the planeswalker battle, or 0
// for the player) obeys the duty.
func (u playerMustAttackUnit) discharges(e *Engine, def state.PlayerID, battle state.ObjID) bool {
	if u.anyDefender {
		return true
	}
	if def != u.controller {
		return false
	}
	if battle == 0 {
		return true
	}
	o := e.G.Obj(battle)
	return o != nil && o.Controller == u.controller && e.hasType(o, "Planeswalker")
}

// markPlayerMustAttack sets Option.AttackOne on the declarable pairs that
// discharge every live PlayerMustAttack duty (or, when no single pair obeys
// them all -- two Troves of different opponents -- any one of them: a legal
// declaration obeys at least one whenever it can). A pair is marked only when
// it costs nothing to declare and a declaration holding it plus the required
// attackers fits the decision's own bounds, so the duty can never leave the
// decision unanswerable. A per-defender cap or a tap-candidate pool can make
// that fit unprovable, so either leaves the duty unmarked (unenforced), the
// attackRequirementGroups precedent.
func markPlayerMustAttack(e *Engine, p state.PlayerID, offers []attackOffer, opts []decision.Option,
	required map[state.ObjID]bool, b attackBounds, tapPool int) {
	units := playerMustAttackUnits(e, p)
	if len(units) == 0 || tapPool > 0 {
		return
	}
	free := make(map[attackOfferKey]bool, len(offers)) // lookup only, never ranged
	for _, of := range offers {
		if of.charge.zero() {
			free[attackOfferKey{id: of.id, def: of.def, battle: of.battle}] = true
		}
	}
	cheapest := make(map[state.ObjID]*decision.Option, len(opts)) // lookup only
	for i := range opts {
		o := &opts[i]
		if o.Group != "" {
			return
		}
		if c, ok := cheapest[o.Obj]; !ok || o.Value < c.Value {
			cheapest[o.Obj] = o
		}
	}
	all := make([]bool, len(opts))
	one := make([]bool, len(opts))
	anyAll := false
	for i := range opts {
		o := &opts[i]
		if !free[attackOfferKey{id: o.Obj, def: o.Player, battle: o.Battle}] || !playerMustAttackFits(o, cheapest, required, b) {
			continue
		}
		met, every := 0, true
		for _, u := range units {
			if u.discharges(e, o.Player, o.Battle) {
				met++
			} else {
				every = false
			}
		}
		one[i] = met > 0
		all[i] = every
		anyAll = anyAll || every
	}
	flags := one
	if anyAll {
		flags = all
	}
	for i := range opts {
		opts[i].AttackOne = flags[i]
	}
}

// playerMustAttackFits reports whether declaring option c beside the required
// attackers (each at its cheapest pair) stays within the Max ceiling, the
// Value budget and the life charge bound.
func playerMustAttackFits(c *decision.Option, cheapest map[state.ObjID]*decision.Option,
	required map[state.ObjID]bool, b attackBounds) bool {
	count, sum, life := 1, c.Value, int32(c.CostLife)+int32(c.CostPhyrexian)*decision.PhyrexianLife
	for id := range required { // a sum: order never reaches the result
		if id == c.Obj || cheapest[id] == nil {
			continue
		}
		r := cheapest[id]
		count++
		sum += r.Value
		life += int32(r.CostLife) + int32(r.CostPhyrexian)*decision.PhyrexianLife
	}
	return count <= b.max && (b.maxSum <= 0 || sum <= b.maxSum) && (b.payerLife <= 0 || life <= b.payerLife)
}
