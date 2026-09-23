package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
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
	// CR 702.6 / CR 601.2f: an ability whose own SA carries an
	// AlternateCost$ rider (the K:Equip expansion's fourth colon field --
	// Transmogrant's Crown's "Equip {2} ... you may pay {B} instead") offers
	// the activator a choice of costs. AltCostIndex selects it: 1 is the
	// alternate, 0 the printed Cost$ (the same field the cast walk uses for
	// an AlternativeCost static's cost). The offer walk (rules/legal.go)
	// gated the alternate option on exactly this cost being payable, so the
	// charge and the gate agree; a stale option whose rider vanished falls
	// back to the printed cost rather than stranding.
	raw := e.parseCost(ab.Params["Cost"])
	if opt.AltCostIndex > 0 {
		if alt, ok := e.abilityAlternateCost(ab); ok {
			raw = alt
		}
	}
	cost, ok := e.fixLifeXCost(p, opt.Obj, raw)
	if !ok {
		// The offer gate (offerCastable's fixLifeXCost conversion) withheld this
		// ability; a stale option that slips through degrades to a no-op.
		return
	}
	// The ability's own ReduceCost$ (Otawara's Channel): the same fold the
	// offer gate in rules/legal.go applied, so the charge and the gate agree
	// (CR 601.2f — a reduction applied to the stored cost exactly once).
	// ownReduceCostOffer resolves a target-dependent body against the BEST
	// legal root target, matching the offer gate's price: the chosen target
	// does not exist yet (CR 601.2c runs after this), and folding 0 here would
	// make continueCast's pre-target payability check reject an ability the
	// offer gate just admitted at the reduced price
	// (belt_of_giant_strength's Equip {10} from a {5} pool).
	// repriceForTargets then re-runs the evaluation with the answered targets
	// and net-adjusts pc.ownReduce to the exact charge.
	own := e.ownReduceCostOffer(p, opt.Obj, ab, pa.Merged)
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

// abilityAlternateCost reads an activated ability's own AlternateCost$ rider:
// an alternative cost the activator may pay INSTEAD of the printed Cost$ (the
// K:Equip expansion's fourth colon field, cards/kw_equip.go). The corpus's
// three carriers are Transmogrant's Crown ("Equip {2} ... pay {B} instead"),
// Bloodthorn Flail (discard a card instead of {3}) and Gavel of the Righteous
// (remove a counter from it instead of {3}). The value is a full Forge cost
// token string parsed by the same ParseCost the printed Cost$ uses, so its
// non-mana parts (Discard/SubCounter) ride the ordinary payment stages.
//
// ok is false when the ability carries no rider, or the rider parses into an
// unmodelled part (Cost.Unknown non-empty) -- the same fail-closed withholding
// every alternative-cost reader takes (rules/statics.go's altCostParse), so an
// unpriceable cost is never offered as an option.
func (e *Engine) abilityAlternateCost(ab *cards.SA) (Cost, bool) {
	if ab == nil {
		return Cost{}, false
	}
	raw := strings.TrimSpace(ab.Params["AlternateCost"])
	if raw == "" {
		return Cost{}, false
	}
	c := e.parseCost(raw)
	if len(c.Unknown) > 0 {
		return Cost{}, false
	}
	return c, true
}

// The activated-ability specifics that differ from a spell's commitCast live
// in cast.go's commitCast under a `pc.ability >= 0` branch: after the shared
// mana/Sac/Discard payment it emits a Tap event (the cost's T part), a CounterChange
// per SubCounter part, the AbilityPush that mints the ability object onto the
// stack, and -- when the ability declares ValidTgts$ -- asks its controller
// for targets against the freshly minted stack object, exactly the shape
// pushTrigger (rules/trigger_queue.go) uses for a trigger's own target ask.
