package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 709.3a/709.3b: Far cast alone from Far // Away is a mono-blue spell, so
// Pillar of the Paruns' "spend this mana only to cast a multicolored spell"
// mana (CR 106.6) can't pay for it, though the card in hand is blue and black
// (CR 709.4). With a Pillar and an Island untapped, the potential planner
// (the offers the full surface reads) finds no plan for Far ({1}{U}): it
// lacks its generic mana. A second Island funds it.
func TestSplitFrontHalfRestrictedManaReadsTheFrontHalf(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	island := corpusCard(t, "Island")
	extras := []*cards.Card{corpusCard(t, "Far"), corpusCard(t, "Grizzly Bears"), corpusCard(t, "Pillar of the Paruns"), island, island}
	e, _ := corpusEngineCfg(t, reg, extras, nil)
	id := moveByName(t, e, 0, "Far", state.ZHand)
	moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	moveByName(t, e, 0, "Pillar of the Paruns", state.ZBattlefield)
	moveByName(t, e, 0, "Island", state.ZBattlefield)
	e.priorityRound()
	if o := e.G.Obj(id); o.Zone != state.ZHand || o.FaceIdx != 0 || len(o.Card.Faces) != 2 {
		t.Fatalf("precondition: Far zone=%s face=%d faces=%d, want a split card in hand at its front half", o.Zone, o.FaceIdx, len(o.Card.Faces))
	}
	if pp, ok := potentialCastModeOK(e.PotentialPaymentPlans(0), id, ""); ok {
		t.Fatalf("Far alone is mono-blue: Pillar mana can't fund its {1}: %+v", pp)
	}
	moveByName(t, e, 0, "Island", state.ZBattlefield)
	e.priorityRound()
	if _, ok := potentialCastModeOK(e.PotentialPaymentPlans(0), id, ""); !ok {
		t.Fatalf("precondition: two Islands pay Far's {1}{U}: %+v", e.PotentialPaymentPlans(0))
	}
}
