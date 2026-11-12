package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const improviseUSpellSrc = "Name:Whirling Gear\nManaCost:1 U\nTypes:Sorcery\nK:Improvise\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"

// TestImproviseCastPaysColouredPipFromLand: CR 702.126a / 601.2h -- an
// artifact pays the generic {1}, and the coloured {U} is paid by activating a
// land's mana ability during payment. With one artifact, one untapped Island
// and nothing floating the cast is offered with a payment witness (it taps the
// Island and leaves the artifact to the announcement) and the planned cast
// completes: the artifact is tapped for Improvise, the Island for {U}.
func TestImproviseCastPaysColouredPipFromLand(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9611, improviseUSpellSrc)
	toMain1(t, e)
	art := onBoard(t, e, 0, plainArtifactSrc)
	island := onBoard(t, e, 0, "Name:Island\nTypes:Basic Land Island\nOracle:x\n")
	e.G.Obj(island).SummonSick = false
	e.pending = nil
	e.askPriority(0)
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatal("pool must start empty")
	}
	if !e.hasCastImprovise(spell) {
		t.Fatal("fixture spell lost Improvise")
	}
	got := e.PlanCastPayment(0, paymentCast(spell))
	if got.Plan == nil || got.Reason != "" {
		t.Fatalf("plan = %+v, want a witness for the pip the artifact cannot pay", got)
	}
	if len(got.Plan.Activations) != 1 || got.Plan.Activations[0].Source != island {
		t.Fatalf("witness taps %+v, want only the Island (the artifact is the Improvise payment)", got.Plan.Activations)
	}
	d := e.Pending()
	d.PaymentActions = e.PaymentActionsForPriority(0, d.Seq)
	var act *decision.PaymentAction
	for i := range d.PaymentActions {
		if d.PaymentActions[i].Cast.Object == spell {
			act = &d.PaymentActions[i]
		}
	}
	if act == nil || len(act.Plans) == 0 {
		t.Fatalf("no payment action for the improvise cast: %+v", d.PaymentActions)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Payment: &decision.PaymentSelection{ActionID: act.ID, Plan: act.Plans[0]}}); err != nil {
		t.Fatalf("Submit planned improvise cast: %v", err)
	}
	if q := e.Pending(); q != nil && q.Kind == decision.KChoose {
		pick := -1
		for j, o := range q.Options {
			if o.Kind == "improvise_generic" && o.Obj == art {
				pick = j
			}
		}
		if pick < 0 {
			t.Fatalf("improvise announcement does not offer the artifact: %+v", q)
		}
		submitChoices(t, e, pick)
	}
	if e.cast != nil || e.G.Obj(spell).Zone != state.ZStack {
		t.Fatalf("cast did not complete: cast=%+v zone=%s pending=%+v", e.cast, e.G.Obj(spell).Zone, e.Pending())
	}
	if !e.G.Obj(art).Tapped || !e.G.Obj(island).Tapped {
		t.Fatalf("artifact tapped=%v Island tapped=%v, want both", e.G.Obj(art).Tapped, e.G.Obj(island).Tapped)
	}
}
