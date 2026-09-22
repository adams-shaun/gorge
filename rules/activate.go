package rules

import (
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
	if opt.GainedSource != 0 {
		// A has-all-abilities-of activation (Forge's GainsAbilitiesOf$,
		// rules/legal.go's gained offer): the ability is a compiled SA on the
		// named FOREIGN card's face, so the anchor is (gainedFrom, gainedIdx)
		// rather than a printed face index or an SVar name. The SA is
		// re-resolved here and minted through events.GainedAbilityPush, so a
		// replay re-resolves the identical body.
		e.beginGainedActivation(p, opt)
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
	if f == nil {
		return
	}
	// CR 702.140d: the flat pile index spans the top face's abilities and
	// every under-card's. PileAbilityAt resolves it -- the same enumeration
	// the offer loop built it from and events.Apply decodes it with -- so an
	// under-card ability activates as its own SA against its own face's SVar
	// table.
	pa, ok := o.PileAbilityAt(opt.Ability)
	if !ok {
		return
	}
	ab := pa.SA
	// CR 602.2b -> 601.2f: an activated ability's total cost composes its
	// activation cost plus applicable cost increases/reductions. Heartstone's
	// ReduceCost Type=Ability is applied here (raise/reduce), folded into the
	// total by manaToPay when the cost is paid -- the same composition a
	// spell's cast gets.
	mods := e.costModifiers(p, opt.Obj, abilityScope(ab))
	cost, ok := e.fixLifeXCost(p, opt.Obj, e.parseCost(ab.Params["Cost"]))
	if !ok {
		// The offer gate (offerCastable's fixLifeXCost conversion) withheld this
		// ability; a stale option that slips through degrades to a no-op.
		return
	}
	// The ability's own ReduceCost$ (Otawara's Channel): the same fold the
	// offer gate in rules/legal.go applied, so the charge and the gate agree
	// (CR 601.2f — a reduction applied to the stored cost exactly once).
	// Targets do not exist yet (CR 601.2c runs after this), so a
	// target-dependent body reads 0 here; repriceForTargets re-runs the
	// evaluation with the answered targets and net-adjusts pc.ownReduce.
	own := e.ownReduceCost(p, opt.Obj, ab, nil, pa.Merged)
	if own > 0 {
		if cost.Generic >= own {
			cost.Generic -= own
		} else {
			cost.Generic = 0
		}
	}
	e.cast = &pendingCast{player: p, card: opt.Obj, from: o.Zone, ability: opt.Ability,
		abilityMerged: pa.Merged, cost: cost, mods: mods, ownReduce: own}
	e.continueCast()
}

// The activated-ability specifics that differ from a spell's commitCast live
// in cast.go's commitCast under a `pc.ability >= 0` branch: after the shared
// mana/Sac/Discard payment it emits a Tap event (the cost's T part), a CounterChange
// per SubCounter part, the AbilityPush that mints the ability object onto the
// stack, and -- when the ability declares ValidTgts$ -- asks its controller
// for targets against the freshly minted stack object, exactly the shape
// pushTrigger (rules/trigger_queue.go) uses for a trigger's own target ask.
