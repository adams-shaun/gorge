package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// activate.go is the activated-ability flow. legal.go offers every legal
// non-mana AB$ ability as an "ability" priority option (rules/legal.go),
// priced by its own Cost$ through the same castable gate every "cast" option
// uses so nothing unpayable is ever offered (totality). Choosing one reaches
// beginActivation here, which reuses the cast flow (cast.go's pendingCast /
// continueCast / commitCast) -- an activated ability with an {X}, Sac, or
// Discard part asks and pays those the same way a spell does, and commitCast's
// ability branch pays the tap / SubCounter parts and pushes the ability onto
// the stack via AbilityPush, then asks targets if the ability declares any.

// beginActivation starts the activation flow for opt (an "ability" priority
// option): parse the ability's own Cost$ into a pendingCast and run the same
// stages a cast runs (X, Delve -- an ability never has Delve, so that stage
// is a no-op --, Sac, Discard), then commitCast pays everything and pushes the
// ability. A stale option (the permanent left its zone, or opt.Ability no
// longer indexes the face) degrades to a no-op rather than panicking the one
// goroutine driving the match.
func (e *Engine) beginActivation(p state.PlayerID, opt decision.Option) {
	o := e.G.Obj(opt.Obj)
	if o == nil {
		return
	}
	if opt.SVar != "" {
		// A granted ability (rules/legal.go's AddAbilities offer) anchors on
		// the SVar name, never a face index -- the same anchor the max-speed
		// "granted" option carries, resolved through the same flow.
		e.beginGrantedActivation(p, opt)
		return
	}
	f := o.Face()
	if f == nil || opt.Ability < 0 || opt.Ability >= len(f.Abilities) {
		return
	}
	ab := f.Abilities[opt.Ability]
	// CR 602.2b -> 601.2f: an activated ability's total cost composes its
	// activation cost plus applicable cost increases/reductions. Heartstone's
	// ReduceCost Type=Ability is applied here (raise/reduce), folded into the
	// total by manaToPay when the cost is paid -- the same composition a
	// spell's cast gets.
	mods := e.costModifiers(p, opt.Obj, abilityScope(ab))
	cost, ok := e.fixLifeXCost(p, opt.Obj, ParseCost(ab.Params["Cost"]))
	if !ok {
		// The offer gate (offerCastable's fixLifeXCost conversion) withheld this
		// ability; a stale option that slips through degrades to a no-op.
		return
	}
	// The ability's own ReduceCost$ (Otawara's Channel): the same fold the
	// offer gate in rules/legal.go applied, so the charge and the gate agree
	// (CR 601.2f — a reduction applied to the stored cost exactly once).
	if n := e.ownReduceCost(p, opt.Obj, ab); n > 0 {
		if cost.Generic >= n {
			cost.Generic -= n
		} else {
			cost.Generic = 0
		}
	}
	e.cast = &pendingCast{player: p, card: opt.Obj, from: o.Zone, ability: opt.Ability,
		cost: cost, mods: mods}
	// TargetsWithSameController$ True (Lodestone Bauble): the pairwise
	// same-owner constraint rides the transaction into handleTarget's
	// Submit-time validator (the offered option list spans every player's
	// graveyard, which the wire's option shape cannot constrain).
	if strings.EqualFold(strings.TrimSpace(ab.Params["TargetsWithSameController"]), "True") {
		e.cast.sameCtrlTargets = true
	}
	e.continueCast()
}

// The activated-ability specifics that differ from a spell's commitCast live
// in cast.go's commitCast under a `pc.ability >= 0` branch: after the shared
// mana/Sac/Discard payment it emits a Tap event (the cost's T part), a CounterChange
// per SubCounter part, the AbilityPush that mints the ability object onto the
// stack, and -- when the ability declares ValidTgts$ -- asks its controller
// for targets against the freshly minted stack object, exactly the shape
// pushTrigger (rules/trigger_queue.go) uses for a trigger's own target ask.
