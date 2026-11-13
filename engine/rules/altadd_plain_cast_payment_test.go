package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// altAddTreasureSrc is the Treasure token's real mana ability: {T},
// Sacrifice this artifact. Its sacrifice makes it the only black source on
// the board, so the printed {B} of Mudbutton Cursetosser is payable only
// through this source and the {2} alternative needs it too.
const altAddTreasureSrc = "Name:Treasure Token\nManaCost:no cost\nTypes:Artifact Treasure\n" +
	"A:AB$ Mana | Cost$ T Sac<1/CARDNAME/this token> | Produced$ Any | Amount$ 1\nOracle:x\n"

// A mandatory either-or additional cost (AlternateAdditionalCost, CR
// 601.2b/h) is chosen at announcement, so the plain cast's payment witness
// must fund the printed cost PLUS the chosen part. The autopay surface only
// offers a plain cast the planner witnesses, so without the altAdd arm
// (planCastAltAdd) Mudbutton Cursetosser was never offered although the
// {2} alternative was plainly payable, and dirac -- which prices the cast
// from its untapped sources -- diverged on the pair's first priority menu.
func TestAlternateAdditionalCostPlainCastHasPaymentPlan(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	c, ok := testutil.CorpusRegistry(t).Lookup("Mudbutton Cursetosser")
	if !ok {
		t.Fatal("corpus lacks Mudbutton Cursetosser")
	}
	spell := handCard(e, c, 0)
	// Precondition: the script carries the either-or additional cost as two
	// parts (Behold a Goblin, or {2}).
	parts := altAddCostParts(e.G.Obj(spell).Face())
	if len(parts) != 2 {
		t.Fatalf("alternate additional cost parsed %v, want two parts", parts)
	}
	// The board pays {2}{B} only: one green source, one red source, and the
	// Treasure that alone can make black.
	onBoardGrant(t, e, 0, "Name:Alt Forest\nTypes:Basic Land Forest\nOracle:x\n")
	onBoardGrant(t, e, 0, "Name:Alt Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	treasure := onBoardGrant(t, e, 0, altAddTreasureSrc)
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision: %+v", d)
	}
	// Precondition: the floating pool is empty, so the ordinary offer walk
	// does not list the cast; only the payment planner can price it.
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == spell {
			t.Fatalf("precondition: the empty pool already offers the cast: %+v", o)
		}
	}
	actions := e.EnsurePaymentActions()
	if !ppHasAction(d, spell) {
		t.Fatalf("no planned payment for the altAdd cast; planner actions: %d", len(actions))
	}
	a := ppAction(t, d, spell)
	if len(a.Plans) != 1 {
		t.Fatalf("planned payments = %d, want 1", len(a.Plans))
	}
	plan := a.Plans[0]
	// The witness must charge the chosen {2} part beside the printed {B},
	// and the Treasure must be one of the sources that funds it.
	if plan.Cost.Generic != 2 || plan.Cost.Mana[state.MB] != 1 {
		t.Fatalf("witness cost = %+v, want {2}{B}", plan.Cost)
	}
	found := false
	for _, act := range plan.Activations {
		if act.Source == treasure {
			found = true
		}
	}
	if !found {
		t.Fatalf("witness does not activate the only black source (Treasure %d): %+v", treasure, plan.Activations)
	}
	// The plan must actually cast the spell: the altAdd ask picks the {2}
	// alternative, then the witnessed sources pay it.
	submitPaymentPlan(t, e, d, a)
	for guard := 0; guard < 20; guard++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		pick := -1
		for _, o := range d.Options {
			if o.Kind == "altaddcost" && o.Amount == 1 {
				pick = o.Index
				break
			}
		}
		if pick < 0 {
			pick = d.Options[0].Index
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pick}}); err != nil {
			t.Fatal(err)
		}
	}
	if z := e.G.Obj(spell).Zone; z != state.ZStack {
		t.Fatalf("the planned altAdd cast left the spell in %s, want the stack", z)
	}
	if z := e.G.Obj(treasure).Zone; z == state.ZBattlefield {
		t.Fatal("the witness paid no black mana: Treasure is still on the battlefield")
	}
}
