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
// has its targets at the end (it is not offered for another reason).
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
	for _, c := range root.legalTargetCandidates(p, o.Obj, o.Obj, sa) {
		if c.kind == "player" || !consumed[c.obj] {
			return true
		}
	}
	return false
}
