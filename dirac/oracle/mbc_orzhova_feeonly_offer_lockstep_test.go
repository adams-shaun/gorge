package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// feeOnlyBoardTally watches a replay for the fee-only class's board: a
// mana-costed activated ability's source and the fee-only repeatable source
// ({2}: Add one mana of any color) its payment needs, both on the same
// battlefield, at a priority decision for that player.
type feeOnlyBoardTally struct {
	play, fee string
	seen      bool
	// offered: the play's own activation option was IN gorge's priority
	// options at a class decision -- the offer gate's printedOK/autoPayOffer
	// accepted it, priced (the prefix script found) or not.
	offered bool
}

// check reads gorge's pending decision and battlefields.
func (y *feeOnlyBoardTally) check(ref *GorgeSide) {
	d := ref.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return
	}
	g := ref.E.G
	playOn, feeOn := false, false
	for _, id := range g.Zone(state.ZBattlefield, d.Player) {
		o := g.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		switch o.Face().Name {
		case y.play:
			playOn = true
		case y.fee:
			feeOn = true
		}
	}
	if !playOn || !feeOn {
		return
	}
	y.seen = true
	if ref.fr == nil {
		return
	}
	for _, a := range ref.fr.cur.Options {
		o, ok := ref.GorgeOption(a)
		if !ok || o.Kind != "ability" || o.Obj == 0 {
			continue
		}
		if src := g.Obj(o.Obj); src != nil && src.Face() != nil && src.Face().Name == y.play {
			y.offered = true
		}
	}
}

// Orzhova, the Church of Deals' "{3}{W}{B}, {T}: Target player loses 1 life
// and you gain 1 life" beside Energy Refractor's "{2}, {T}: Add one mana of
// any color" (tail-02 pair 55, seed 13035303116691166810): the activation's
// payment needs the fee-only repeatable source's {2} paid from other sources'
// production (tap two lands, then activate the Refractor, CR 602.2b), which
// the payment planner cannot interleave -- the Refractor is a deferred source
// the planner's census misses, so the play rides the census's funding and its
// prefix search. The census must never stand COMPLETE over a fee-only source
// whose fee the planner cannot interleave (paymentPlanCensusOf funds the gap,
// CR 605.3a): a complete census turns the planner's no-plan into a proven
// "insufficient", the play is omitted outright, and the kernel -- which
// reaches the line through the same funded prefix space -- parts at the
// offer (the filing run's decisions 393 and 746).
//
// The pin: the game reaches the class board and the two engines agree on
// every decision to its end. Reverting paymentPlanCensusOf's funding hunk
// (b0e2ddde34) parts the same game at decision 393, so the pin bites.
func TestMBCOrzhovaFeeOnlyCensusNotAProofLockstep(t *testing.T) {
	t.Parallel()
	y := &feeOnlyBoardTally{play: "Orzhova, the Church of Deals", fee: "Energy Refractor"}
	stdReproLockstepReaching(t, "testdata/repro/orzhova_feeonly_funded_activation_offer.repro.json", "tail-02", func(ref *GorgeSide) bool {
		y.check(ref)
		return y.seen
	}, func(ref *GorgeSide) bool {
		y.check(ref)
		return y.offered
	})
}
