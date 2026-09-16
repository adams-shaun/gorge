package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// UnlessPayer$ Player.targetedBy and the Draw<N/Spec> unless-cost component,
// on Kuroki, Thief of Talents's real end-step trigger: "target opponent may
// draw four cards. If they do, look at that player's hand and you may cast a
// spell from their hand without paying its mana cost. If they don't, put two
// +1/+1 counters on NICKNAME." TrigReveal carries UnlessCost$
// Draw<4/Player.targetedBy> | UnlessPayer$ Player.targetedBy |
// UnlessSwitched$ True — paying CAUSES the reveal/cast body, and the payer
// is the player the ability targeted.

// driveKurokiTrig fires Kuroki's real end-step trigger through the ordinary
// trigger path (TriggerPush, placement target ask), with Kuroki controlled by
// seat 0. It returns once the unless-pay ask is pending.
func driveKurokiTrig(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e := stealEngine(t, 741)
	kuroki := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Kuroki, Thief of Talents"))
	sa := cards.ResolveSVar(e.G.Obj(kuroki).Face().SVars, "TrigReveal")
	if sa == nil || sa.Params["UnlessPayer"] != "Player.targetedBy" || sa.Params["UnlessCost"] != "Draw<4/Player.targetedBy>" {
		t.Fatalf("Kuroki TrigReveal = %+v, want the targeted unless-draw shape", sa)
	}
	// Fire the trigger the way a real turn does: drive to Kuroki
	// controller's end step, where "At the beginning of your end step"
	// fires and the placement target ask appears.
	driveToStep(t, e, e.G.Turn, 0, state.StepEnd)
	e.putTriggersOnStack()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Player != 0 {
		t.Fatalf("pending = %+v, want the placement target ask", d)
	}
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			submitChoices(t, e, o.Index)
			break
		}
	}
	// The placed ability resolves only after the priority rounds end.
	if d := passUntilNonPriority(t, e, 8); d == nil || d.ResumeKind != "unless_pay" || d.Player != 1 {
		t.Fatalf("pending = %+v, want the unless-pay ask for seat 1", d)
	}
	return e, kuroki
}

// TestUnlessDrawCostKurokiPay pins the binding and the payment: the pay
// decision is offered to the TARGETED opponent (seat 1, not Kuroki's
// controller), and paying draws that opponent exactly four cards — the cost
// the corpus spells as Draw<4/Player.targetedBy>.
func TestUnlessDrawCostKurokiPay(t *testing.T) {
	e, kuroki := driveKurokiTrig(t)
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes || d.ResumeKind != "unless_pay" {
		t.Fatalf("pending = %+v, want the unless-pay ask", d)
	}
	if d.Player != 1 {
		t.Fatalf("payer = seat %d, want the targeted opponent seat 1", d.Player)
	}
	before := countDraw(e)
	answerUnlessPay(t, e, true)
	if got := countDraw(e) - before; got != 4 {
		t.Fatalf("pay drew %d cards for the opponent, want 4", got)
	}
	if o := e.G.Obj(kuroki); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Kuroki zone = %v, want kept", o)
	}
}

// TestUnlessDrawCostKurokiDecline is the mirror: the opponent declines, no
// cards move, and the switched orientation skips the reveal/cast body.
func TestUnlessDrawCostKurokiDecline(t *testing.T) {
	e, _ := driveKurokiTrig(t)
	before := countDraw(e)
	answerUnlessPay(t, e, false)
	if got := countDraw(e) - before; got != 0 {
		t.Fatalf("decline drew %d cards, want 0", got)
	}
}

// TestParseUnlessCostDrawComponents pins the strict parser: a fixed
// Draw<N/Spec> token is priceable (paid by drawing), a Draw<X/...> unfolded
// amount is not, and every unmodelled verb still declines.
func TestParseUnlessCostDrawComponents(t *testing.T) {
	if c, ok := ParseUnlessCost("Draw<4/Player.targetedBy>"); !ok || len(c.Draw) != 1 ||
		c.Draw[0].N != 4 || c.Draw[0].Spec != "Player.targetedBy" {
		t.Fatalf("Draw<4/Player.targetedBy> = %+v ok=%v, want one 4-card targeted part", c, ok)
	}
	if _, ok := ParseUnlessCost("Draw<X/You>"); ok {
		t.Fatal("Draw<X/You> priced; an unfolded draw amount must decline")
	}
	if c, ok := ParseUnlessCost("PayLife<2> Draw<1/You>"); !ok || c.Life != 2 || len(c.Draw) != 1 {
		t.Fatalf("PayLife<2> Draw<1/You> = %+v ok=%v, want life 2 and one draw part", c, ok)
	}
	if _, ok := ParseUnlessCost("TapXType<1/Creature>"); ok {
		t.Fatal("TapXType priced; an unmodelled verb must decline")
	}
}
