package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Throwaway: float 1 W + 1 R (activate a Plains and Shadowblood Ridge as the
// found script did), then read the Reveler cast's witness.
func TestZZBedlamScriptTail(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	extras := []*cards.Card{lookup(t, reg, "Bedlam Reveler"), lookup(t, reg, "Devastate"),
		lookup(t, reg, "Wanderer's Intervention"), lookup(t, reg, "Island"),
		lookup(t, reg, "Shadowblood Ridge"), lookup(t, reg, "Mountain"),
		lookup(t, reg, "Untaidake, the Cloud Keeper"), lookup(t, reg, "Forest")}
	e := quietBaseWith(t, reg, extras)
	rev := addHand(t, e, 0, lookup(t, reg, "Bedlam Reveler"))
	addZone(t, e, 0, lookup(t, reg, "Devastate"), state.ZGraveyard)
	addZone(t, e, 0, lookup(t, reg, "Wanderer's Intervention"), state.ZGraveyard)
	addZone(t, e, 0, lookup(t, reg, "Island"), state.ZBattlefield)
	addZone(t, e, 0, lookup(t, reg, "Forest"), state.ZBattlefield)
	addZone(t, e, 0, lookup(t, reg, "Shadowblood Ridge"), state.ZBattlefield)
	addZone(t, e, 0, lookup(t, reg, "Mountain"), state.ZBattlefield)
	addZone(t, e, 0, lookup(t, reg, "Untaidake, the Cloud Keeper"), state.ZBattlefield)

	e.askPriority(0)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending not priority: %+v", d)
	}
	// activate the Plains (floating W)
	var plainsAct, ridgeAct *decision.Option
	for i := range d.Options {
		o := &d.Options[i]
		if o.Kind != "activate" {
			continue
		}
		if oo := e.G.Obj(o.Obj); oo != nil && oo.Face() != nil {
			switch oo.Face().Name {
			case "Plains":
				if plainsAct == nil {
					plainsAct = o
				}
			case "Shadowblood Ridge":
				ridgeAct = o
			}
		}
	}
	if plainsAct == nil || ridgeAct == nil {
		t.Fatalf("missing activations: %v", optKinds(d.Options))
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{plainsAct.Index}}); err != nil {
		t.Fatal(err)
	}
	// the ridge's activation may ask a colour
	d = e.Pending()
	if d == nil {
		t.Fatal("nil pending after plains activation")
	}
	t.Logf("after plains: kind=%v options=%v", d.Kind, optKinds(d.Options))
	if d.Kind == decision.KPriority {
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{ridgeAct.Index}}); err != nil {
			t.Fatal(err)
		}
		d = e.Pending()
	}
	// answer the colour ask with R (or the first offered)
	for d != nil && d.Kind == decision.KChoose {
		pick := -1
		for i := range d.Options {
			if d.Options[i].ManaSymbol == "R" {
				pick = d.Options[i].Index
				break
			}
		}
		if pick < 0 {
			pick = d.Options[0].Index
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pick}}); err != nil {
			t.Fatal(err)
		}
		d = e.Pending()
	}
	t.Logf("pool now=%v", e.G.Players[0].Pool)
	e.askPriority(0)
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending not priority after floats: %+v", d)
	}
	for _, pp := range e.PotentialPaymentPlans(0) {
		nm := ""
		if o := e.G.Obj(pp.Action.Obj); o != nil && o.Face() != nil {
			nm = o.Face().Name
		}
		if nm != "Bedlam Reveler" {
			continue
		}
		t.Logf("reveler: reason=%q script=%d hasPlan=%v", pp.Reason, len(pp.Script), pp.Plan != nil)
		if pp.Plan != nil {
			for _, act := range pp.Plan.Activations {
				sn := ""
				if src := e.G.Obj(act.Source); src != nil && src.Face() != nil {
					sn = src.Face().Name
				}
				t.Logf("   act %s produces=%v", sn, act.Produces)
			}
		}
	}
	if c, ok := e.CastPaymentCost(0, rev); ok {
		t.Logf("CastPaymentCost=%+v", c)
	}
	if d := e.Pending(); d != nil {
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Obj == rev {
				t.Fatal("BUG: the reveler cast is offered on the pending surface")
			}
		}
	}
}
