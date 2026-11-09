package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// altAddMerfolkSrc is the behold candidate: a Merfolk on the battlefield makes
// Silvergill Mentor's Behold<1/Merfolk> part payable, so its plain cast takes
// the planCastAltAdd arm.
const altAddMerfolkSrc = "Name:Alt Add Merfolk\nManaCost:U\nTypes:Creature Merfolk\nPT:1/1\nOracle:x\n"

// A planned either-or additional cost prices the cast while the card is still
// in hand. The hand-demand rank (key 6) must still exclude that card, as the
// plain cast planner does: otherwise the witness reserves a source for a pip
// that is leaving the hand. Here Silvergill Mentor's own {U} would keep an
// Island untapped, while the correct demand (zero, the hand empty apart from
// the cast) lets the typed witness tap the two Islands.
func TestAltAddPlanExcludesCastCardFromHandDemand(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	c, ok := testutil.CorpusRegistry(t).Lookup("Silvergill Mentor")
	if !ok {
		t.Fatal("corpus lacks Silvergill Mentor")
	}
	spell := handCard(e, c, 0)
	onBoardGrant(t, e, 0, altAddMerfolkSrc)
	island1 := onBoardGrant(t, e, 0, ppIsland)
	island2 := onBoardGrant(t, e, 0, ppIsland)
	onBoardGrant(t, e, 0, ppSwamp)
	mountain := onBoardGrant(t, e, 0, ppMountain)
	if !(island1 < island2 && island2 < mountain) {
		t.Fatalf("precondition: source ids %d %d %d not ascending", island1, island2, mountain)
	}
	e.priorityRound()
	d := e.Pending()
	if d == nil || len(d.PaymentActions) == 0 {
		e.EnsurePaymentActions()
		d = e.Pending()
	}
	a := ppAction(t, d, spell)
	if len(a.Plans) != 1 {
		t.Fatalf("planned payments = %d, want 1", len(a.Plans))
	}
	plan := a.Plans[0]
	// Precondition: the witness prices the behold part, the printed {1}{U}.
	if plan.Cost.Generic != 1 || plan.Cost.Mana[state.MU] != 1 {
		t.Fatalf("witness cost = %+v, want {1}{U}", plan.Cost)
	}
	got := make(map[state.ObjID]bool, len(plan.Activations))
	for _, act := range plan.Activations {
		got[act.Source] = true
	}
	if !got[island1] || !got[island2] {
		t.Fatalf("witness sources = %v, want the two Islands %d and %d", got, island1, island2)
	}
	if got[mountain] {
		t.Fatalf("witness tapped the Mountain %d: the cast card's own {U} was counted as hand demand", mountain)
	}
}
