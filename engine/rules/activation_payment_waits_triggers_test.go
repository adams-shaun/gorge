package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestActivationPaymentWaitsForTriggers: CR 601.2g-i applies to an activated
// ability through CR 602.2b. Hydraulic Helper's {T} is the only way to fund
// Captain America's Shield's equip {2} beside Gleaming Bastion's {C}, but the
// planner census cannot price Helper's restricted {U}: the equip is offered
// through a potential-plan script that taps Helper first. That prefix is one
// payment window, so Helper's tap is paid once. Captain America, Living
// Legend's untap trigger and Rewrite History's TapAll trigger resolve as the
// prefix's stack empties, not between two activations of the same source: a
// second tap would float a second {U} and put a second plan counter on Rewrite
// History (the committed std-msh divergence at decision 776).
func TestActivationPaymentWaitsForTriggers(t *testing.T) {
	t.Parallel()
	e := boundBoard(t, []string{
		"Hydraulic Helper",
		"Rewrite History",
		"Captain America, Living Legend",
		"Captain America's Shield",
		"Gleaming Bastion",
	}, nil)
	shield := findByName(e, "Captain America's Shield", 0)
	helper := findByName(e, "Hydraulic Helper", 0)
	hero := findByName(e, "Captain America, Living Legend", 0)
	history := findByName(e, "Rewrite History", 0)

	ppPriority(t, e)
	var script []ScriptStep
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == "ability" && pp.Action.Obj == shield {
			if pp.Reason != "" || len(pp.Script) == 0 {
				t.Fatalf("the equip's potential plan = reason %q detail %q script %v, want a scripted prefix", pp.Reason, pp.Detail, pp.Script)
			}
			script = pp.Script
		}
	}
	if script == nil {
		t.Fatalf("the equip has no potential plan: %+v", e.PotentialPaymentPlans(0))
	}
	// Precondition: the prefix taps Helper, and it orders Captain America last
	// (top of the stack, so its untap resolves while Rewrite History's TapAll
	// trigger is still waiting) -- the interleave that makes the second tap
	// legal once Helper untaps.
	taps, capOnTop := 0, false
	for _, st := range script {
		for _, pk := range st.Picks {
			if pk.Kind == "activate" && pk.Obj == helper {
				taps++
			}
		}
		if st.Kind == decision.KTriggerOrder && len(st.Picks) > 0 && st.Picks[len(st.Picks)-1].Obj == hero {
			capOnTop = true
		}
	}
	if taps == 0 || !capOnTop {
		t.Fatalf("precondition: script %v taps Helper %d times, Captain America on top %v; want the recorded interleave", script, taps, capOnTop)
	}

	for i, st := range script {
		d := passToActingSeat(t, e, 0)
		if d == nil || d.Kind != st.Kind || d.Player != 0 {
			t.Fatalf("script step %d (%v): pending = %+v", i, st, d)
		}
		var choices []int
		for _, pk := range st.Picks {
			found := -1
			for j := range d.Options {
				if pk.Matches(&d.Options[j]) {
					found = d.Options[j].Index
					break
				}
			}
			if found < 0 {
				t.Fatalf("script step %d (%v): pick %v not offered in %+v", i, st, pk, d.Options)
			}
			choices = append(choices, found)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
			t.Fatalf("script step %d (%v): %v", i, st, err)
		}
	}

	// The prefix has priced the equip; activating it asks for its target.
	d := passToActingSeat(t, e, 0)
	opt := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == shield {
			opt = o.Index
		}
	}
	if opt < 0 {
		t.Fatalf("the equip is not offered after its prefix: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt}}); err != nil {
		t.Fatal(err)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending after activating = %+v, want the equip's target ask", d)
	}

	// The assertions are the state at the target ask, before the activation's
	// mana is paid: one Helper tap happened, its {U} floats, and Rewrite
	// History has one plan counter.
	if got := tapsOf(e, helper); got != 1 {
		t.Errorf("Hydraulic Helper was tapped %d times, want once", got)
	}
	if got := e.G.Players[0].Pool[state.ManaIndex('U')]; got != 1 {
		t.Errorf("pool {U} = %d, want 1 (%v)", got, e.G.Players[0].Pool)
	}
	if got := counterN(e.G.Obj(history), "PLAN"); got != 1 {
		t.Errorf("Rewrite History plan counters = %d, want 1", got)
	}
}

// passToActingSeat answers every other seat's priority and trigger-order decision
// until p's decision is pending, mirroring the scripted-prefix executor.
func passToActingSeat(t *testing.T, e *Engine, p state.PlayerID) *decision.Decision {
	t.Helper()
	for guard := 0; guard < 32; guard++ {
		d := e.Pending()
		if d == nil || d.Player == p {
			return d
		}
		in := decision.Intent{Seq: d.Seq, Player: d.Player}
		switch d.Kind {
		case decision.KTriggerOrder:
			for _, o := range d.Options {
				in.Choices = append(in.Choices, o.Index)
			}
		case decision.KPriority:
			for _, o := range d.Options {
				if o.Kind == "pass" {
					in.Choices = []int{o.Index}
				}
			}
			if in.Choices == nil {
				t.Fatalf("other seat's priority has no pass: %+v", d.Options)
			}
		default:
			t.Fatalf("other seat %d has an unexpected decision %q", d.Player, d.Kind)
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("pass seat %d at %q: %v", d.Player, d.Kind, err)
		}
	}
	t.Fatal("passToActingSeat did not reach the seat")
	return nil
}

// tapsOf counts the Tap events the log recorded for obj.
func tapsOf(e *Engine, obj state.ObjID) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Tap && ev.Obj == obj {
			n++
		}
	}
	return n
}
