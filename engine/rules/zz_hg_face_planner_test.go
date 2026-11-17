package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestScratchHGFacePlanner: the toy board (Plains, Island, Heap Gate out;
// Horn of Valhalla in hand; main phase) and what the potential planner says
// about the Adventure face cast.
func TestScratchHGFacePlanner(t *testing.T) {
	e := handEngine(t, card(t, corpusCardText(t, "h/horn_of_valhalla_ysgards_call.txt")))
	onBoardCard(t, e, 0, corpusAlternativeCard(t, "Heap Gate"))
	onBoardCard(t, e, 0, corpusAlternativeCard(t, "Island"))
	onBoardCard(t, e, 0, corpusAlternativeCard(t, "Plains"))
	if e.G.Step != state.StepMain1 || e.G.Priority != 0 {
		t.Fatalf("step %v priority %v", e.G.Step, e.G.Priority)
	}
	e.Advance() // pose the priority decision (the script search reads e.Pending())
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending %v", d)
	}
	for _, pp := range e.PotentialPaymentPlans(0) {
		if !strings.Contains(pp.Action.Label, "Ysgard") && !strings.Contains(pp.Action.Label, "Horn") {
			continue
		}
		t.Logf("play %q mode %q: reason %q detail %q script %d steps, plan %v",
			pp.Action.Label, pp.Action.Mode, pp.Reason, pp.Detail, len(pp.Script), pp.Plan != nil)
		if pp.Action.Mode == "adventure_alt" {
			hyp, opts := e.potentialWalkOf(0, true)
			var o decision.Option
			for _, q := range opts {
				if q.Mode == "adventure_alt" {
					o = q
				}
			}
			census := e.paymentPlanCensusOf(0, &hyp)
			t.Logf("census: complete=%v relaxable=%v bounded=%v gaps=%v fees=%d fund=%d",
				census.complete, census.relaxable, census.bounded, census.gaps, census.fees(), census.fund)
			script, ok := e.potentialPrefixScript(0, o, census.gaps, census.fees())
			t.Logf("script search: found=%t steps=%d", ok, len(script))
			// replicate the search's first node: what the priority decision
			// offers, whether Heap Gate's filter activation is among it, and
			// what the clone poses after it.
			gapID := census.gaps[0]
			d := e.Pending()
			for i := range d.Options {
				op := &d.Options[i]
				if op.Kind == "activate" {
					t.Logf("option %q obj %d ability %d (gap=%t)", op.Label, op.Obj, op.Ability, op.Obj == gapID)
				}
			}
			c := e.hypClone(e)
			defer e.hypRelease(c)
			var act *decision.Option
			for i := range d.Options {
				op := &d.Options[i]
				if op.Kind == "activate" && op.Obj == gapID {
					act = op
				}
			}
			if act == nil {
				t.Log("no activation option for the gap source")
			} else {
				in := decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{act.Index}}
				if err := c.Submit(in); err != nil {
					t.Logf("submit failed: %v", err)
				} else {
					pd := c.Pending()
					if pd == nil {
						t.Logf("clone settled: active=%d step=%v pending=nil", c.G.Active, c.G.Step)
					} else {
						t.Logf("clone asks %q player %d, %d options: %v", pd.Kind, pd.Player, len(pd.Options), pd.Options[:min(6, len(pd.Options))])
					}
					got, _ := c.potentialPlayVerdict(0, decision.Option{Kind: "cast", Obj: o.Obj, Mode: o.Mode})
					t.Logf("node verdict: reason %q detail %q plan %t", got.Reason, got.Detail, got.Plan != nil)
				}
			}
		}
	}
	offered := false
	for _, a := range e.PotentialActions(0) {
		if strings.Contains(a.Label, "Ysgard") {
			t.Logf("potential action %q mode %q offered", a.Label, a.Mode)
			offered = true
		}
	}
	if !offered {
		t.Log("no Ysgard's Call potential action at all")
	}
	pd := e.Pending()
	for i := range pd.Options {
		op := &pd.Options[i]
		if op.Kind == "cast" || op.Kind == "activate" {
			t.Logf("decision option %q kind %q mode %q obj %d", op.Label, op.Kind, op.Mode, op.Obj)
		}
	}
	t.Fatal("scratch diagnostic end")
}
