package effects

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

type excessDamageRecipientCode uint16

const (
	excessDamageRecipientUnknown excessDamageRecipientCode = iota
	// excessDamageRecipientController is the damaged permanent's controller:
	// Forge's TargetedController / ThisTargetedController, which both name
	// the controller of the object the DealDamage ability is hitting.
	excessDamageRecipientController
)

var excessDamageRecipientCodes = state.NewStrCodes(
	state.StrEntry[excessDamageRecipientCode]{Key: "TargetedController", Val: excessDamageRecipientController},
	state.StrEntry[excessDamageRecipientCode]{Key: "ThisTargetedController", Val: excessDamageRecipientController},
)

// emitObjectDamageExcess is emitObjectDamage for a DealDamage recipient that
// honours ExcessDamage$ (CR 120.4a: "Excess damage is dealt to that creature's
// controller instead"). When the script names a recipient spec, the amount
// exceeds what is lethal to the permanent (excessLethal, read BEFORE the hit:
// marked damage counts, deathtouch does not shrink it, CR 120.10) and the
// ExcessDamageCondition$ filter matches the damage SOURCE (Ram Through's
// Card.withTrample), the permanent is dealt exactly the lethal amount and the
// remainder goes to the named player from the same source, inside the caller's
// one damage batch. Every other case -- no ExcessDamage$, a recipient with no
// lethal threshold, an amount within lethal, a failed condition, a spec this
// build does not model (one Note) -- is the plain emitObjectDamage hit.
// It returns what the permanent itself was dealt.
func emitObjectDamageExcess(h Host, c *Ctx, dp *DealDamageParams, r damageRider, target state.ObjID) int32 {
	o := h.Game().Obj(target)
	if dp.ExcessDamage == "" || o == nil {
		return emitObjectDamage(r, target)
	}
	lethal, ok := excessLethal(h, o)
	if !ok || r.amount <= lethal {
		return emitObjectDamage(r, target)
	}
	if cond := dp.ExcessDamageCondition; cond != "" &&
		!MatchesSpecCtx(h.Game(), cond, r.source, c.SpecContext(c.Controller)) {
		return emitObjectDamage(r, target)
	}
	if excessDamageRecipientCodes.Code(dp.ExcessDamage) != excessDamageRecipientController {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "unmodelled DealDamage ExcessDamage$ " + dp.ExcessDamage})
		return emitObjectDamage(r, target)
	}
	controller := o.Controller
	toPlayer := r
	toPlayer.amount = r.amount - lethal
	toObject := r
	toObject.amount = lethal
	dealt := emitObjectDamage(toObject, target)
	emitPlayerDamage(toPlayer, controller)
	return dealt
}
