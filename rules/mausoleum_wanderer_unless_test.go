package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestMausoleumWandererUnlessPayUsesSacrificedPower drives the real corpus
// ability through activation, targeting, the unless-pay ask, and payment. The
// target spell is a small authored instant; the counter source itself is the
// compiled Mausoleum Wanderer card.
func TestMausoleumWandererUnlessPayUsesSacrificedPower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	wanderer := mustCorpusCard(t, reg, "Mausoleum Wanderer")
	bolt := card(t, "Name:Test Instant\nManaCost:0\nTypes:Instant\nA:SP$ Draw | NumCards$ 0\nOracle:test\n")
	e := handEngine(t, wanderer, bolt)
	ids := handIDsByFace(e)
	wid, spell := ids["Mausoleum Wanderer"], ids["Test Instant"]
	if wid == 0 || spell == 0 {
		t.Fatalf("real Wanderer or target missing: %v", ids)
	}
	// Put the real card on the battlefield and assert its live power before
	// activation; the captured LKI must survive the payment suspension.
	e.emit(events.Event{Kind: events.MoveZone, Obj: wid, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.CounterChange, Obj: wid, Counter: "P1P1", Amount: 2})
	if got := e.Power(wid); got != 3 {
		t.Fatalf("Wanderer power precondition = %d, want 3", got)
	}
	e.G.Obj(wid).SummonSick = false

	// Cast the target spell, then activate the Wanderer in response.
	e.G.Players[0].Pool[state.MC] = 3
	e.priorityRound()
	submitChoices(t, e, passToCast(t, e, spell))
	for i := 0; i < 4; i++ {
		if _, ok := findAbilityOption(e, wid, 0); ok {
			break
		}
		passPriorityOnce(t, e)
	}
	submitChoices(t, e, abilityOption(t, e, wid, 0).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after activation got %+v, want target ask", d)
	}
	target := -1
	for _, o := range d.Options {
		if o.Obj == spell {
			target = o.Index
		}
	}
	if target < 0 {
		t.Fatalf("target spell not offered: %+v", d.Options)
	}
	submitChoices(t, e, target)
	d = drainUntilUnlessPay(t, e, 20)
	if d == nil || d.ResumeKind != "unless_pay" {
		t.Fatalf("got %+v, want unless-pay ask", d)
	}
	if len(d.Options) == 0 || d.Options[0].Label == "Pay the cost" || d.Options[0].Label == "Pay the cost, or decline" {
		t.Fatalf("resolved generic amount missing from ask: %+v", d.Options)
	}
	if got := d.Options[0].Label; got != "Pay {3} — don't counter" {
		t.Fatalf("unless ask label = %q, want resolved {3}", got)
	}
	before := e.G.Players[0].Pool[state.MC]
	submitChoices(t, e, d.Options[0].Index)
	if got := e.G.Players[0].Pool[state.MC]; got != before-3 {
		t.Fatalf("payment used %d generic mana, want 3", before-got)
	}
}
