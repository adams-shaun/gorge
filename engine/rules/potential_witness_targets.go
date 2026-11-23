package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// witnessTargetReader is the target census witnessAnnouncesBeforePaying reads.
type witnessTargetReader interface {
	legalTargetCandidates(p state.PlayerID, source, excludeSelf state.ObjID, sa *cards.SA) []targetCandidate
	castTargetsAvailable(p state.PlayerID, id state.ObjID, sa *cards.SA) bool
}

// witnessAnnouncesBeforePaying reports whether a plain cast that is no longer
// offered after the witness's consuming steps ran (end) lost its targets only
// to the payment, which CR 601.2c-h orders after the announcement: the spell
// was announceable against the board the witness started from (root), and a
// candidate target of it survives every consumed permanent. Confound
// ("Counter target spell that targets a creature") over Larger Than Life that
// targets the one Eldrazi Spawn the witness sacrifices for {C}: the Spawn is
// not Confound's target, and the target is chosen before the sacrifice. A cast
// whose every candidate is consumed stays refused, as does a cast that still
// has its targets at the end (it is not offered for another reason). The
// survivors must also make a legal declaration (survivorsCanTarget): Combo
// Attack's two creatures you control, one of them a sacrificed Spawn, is
// payable only by paying with a target it announced, which is the planned
// payment's (targetPaidNeedsPlannedPayment), not a cast the walk reaches.
func witnessAnnouncesBeforePaying(root, end witnessTargetReader, g *state.Game, p state.PlayerID, o decision.Option, plan decision.PaymentPlan) bool {
	if o.Kind != "cast" || o.Mode != "" {
		return false
	}
	obj := g.Obj(o.Obj)
	if obj == nil || obj.Face() == nil {
		return false
	}
	sa := obj.Face().SpellAbility()
	if sa == nil || end.castTargetsAvailable(p, o.Obj, sa) {
		return false
	}
	if !root.castTargetsAvailable(p, o.Obj, sa) {
		return false
	}
	consumed := map[state.ObjID]bool{}
	for _, a := range plan.Activations {
		if a.Consequence != nil && (a.Consequence.Sacrifice || a.Consequence.ReturnToHand) {
			consumed[a.Source] = true
		}
	}
	var survivors []targetCandidate
	for _, c := range root.legalTargetCandidates(p, o.Obj, o.Obj, sa) {
		if c.kind == "player" || !consumed[c.obj] {
			survivors = append(survivors, c)
		}
	}
	if len(survivors) == 0 {
		return false
	}
	return survivorsCanTarget(root, p, o.Obj, sa, survivors)
}

// witnessCostReader is the engine surface the replay's repriced-cost arm
// (witnessCostRepriced) reads: the cast's price and target census
// (witnessTargetReader's), plus the offer base, the cost statics and the
// castability probe the payable-in-isolation check prices with. It keeps the
// arm off the whole engine (the engineSurface ratchet).
type witnessCostReader interface {
	witnessTargetReader
	CastPaymentCost(p state.PlayerID, id state.ObjID) (decision.PaymentCost, bool)
	castOfferBase(p state.PlayerID, id state.ObjID) Cost
	collectCostStatics() costStaticViews
	offerCastableUsing(statics costStaticViews, p state.PlayerID, id state.ObjID, base *Cost, scope costScope, ability bool, hyp *state.Mana) bool
}

// witnessMenuOffers reports whether the pending decision's menu itself
// offers the play o: the walk's payment-end acceptance's first arm, before
// the announcement arm (witnessAnnouncesBeforePaying) and, armed by the
// caller, the repriced-cost arm (witnessCostRepriced).
func witnessMenuOffers(d *decision.Decision, o decision.Option) bool {
	for i := range d.Options {
		opt := &d.Options[i]
		if opt.Kind == o.Kind && opt.Obj == o.Obj && opt.Ability == o.Ability && opt.Mode == o.Mode && opt.AltCostIndex == o.AltCostIndex {
			return true
		}
	}
	return false
}

// survivorsCanTarget reports whether the candidates left after a payment's
// sacrifices still admit a legal declaration of sa's targets: its resolved
// minimum under the cross-target constraints targetChoiceFeasible reads. A
// minimum that depends on an unannounced X binds nothing here. A reader that
// is not the engine (the unit test's census) has no declaration rules to
// apply: one survivor is enough there.
func survivorsCanTarget(root witnessTargetReader, p state.PlayerID, id state.ObjID, sa *cards.SA, survivors []targetCandidate) bool {
	e, ok := root.(*Engine)
	if !ok {
		return true
	}
	min, dynamic := targetOfferMin(e, p, id, sa, 0, false, nil)
	return dynamic || e.targetChoiceFeasible(sa, survivors, min)
}
