package rules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A may-play cast under a MayPlayIgnoreColor$ grant may spend its relaxed
// coloured pips from a coloured unit of any colour (CR 107.4c: "any color"
// never includes colourless), so a plan whose only untapped source offers both
// a colourless and a coloured ability must activate the coloured one. The
// folded plan prices the pip generic and taps the colourless ability; the
// witness then rejects it and the cast is never offered -- gorge loses a legal
// play the kernel's planner makes.

// mayPlayAnycolorGambit puts a corpus Flaming Gambit carrying the exiled
// card's own may-play grant in seat 0's exile.
func mayPlayAnycolorGambit(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	grant := "S:Mode$ Continuous | MayPlay$ True | MayPlayIgnoreColor$ True | Affected$ Card.Self | AffectedZone$ Exile | EffectZone$ Exile | Description$ x\n"
	src := strings.Replace(corpusCardText(t, "f/flaming_gambit.txt"), "Oracle:", grant+"Oracle:", 1)
	o := e.G.AddObject(card(t, src), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZExile})
	return o.ID
}

// TestMayPlayAnyColorPlanFundsTheColouredPipFromAColouredSource: Flaming
// Gambit ({X}{R}) in exile under the grant, one untapped dual-ability land
// (Produced$ C beside Produced$ Combo G U), an empty pool. The plan for the
// X=0 cast must tap the coloured ability.
func TestMayPlayAnyColorPlanFundsTheColouredPipFromAColouredSource(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	coast := onBoard(t, e, 0, "Name:Test Coast\nTypes:Land\n"+
		"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Colourless.\n"+
		"A:AB$ Mana | Cost$ T | Produced$ Combo G U | SpellDescription$ Green or blue.\nOracle:x\n")
	gambit := mayPlayAnycolorGambit(t, e)
	e.pending = nil
	e.askPriority(0)

	plans := e.PotentialPaymentPlans(0)
	var plan *PotentialPlan
	for i := range plans {
		pp := &plans[i]
		if pp.Action.Kind == "cast" && pp.Action.Obj == gambit && pp.Action.Mode == "mayplay" {
			plan = pp
		}
	}
	if plan == nil {
		t.Fatalf("no may-play plan for the exiled Flaming Gambit; plans = %s", plansString(plans))
	}
	if plan.Reason != "" || plan.Plan == nil {
		t.Fatalf("gambit plan = %q/%q with plan %v, want a witness", plan.Reason, plan.Detail, plan.Plan)
	}
	coloured := false
	for _, a := range plan.Plan.Activations {
		if a.Source == coast && (a.Produces[state.MW] > 0 || a.Produces[state.MU] > 0 ||
			a.Produces[state.MB] > 0 || a.Produces[state.MR] > 0 || a.Produces[state.MG] > 0) {
			coloured = true
		}
	}
	if !coloured {
		t.Fatalf("the gambit plan activates no coloured unit: %s", planString(*plan))
	}
}

// plansString renders plans for a failure message, verdicts included.
func plansString(plans []PotentialPlan) string {
	var sb strings.Builder
	for _, pp := range plans {
		sb.WriteString(planString(pp))
		sb.WriteString("; ")
	}
	return sb.String()
}

func planString(pp PotentialPlan) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "(%s obj=%d mode=%s", pp.Action.Kind, pp.Action.Obj, pp.Action.Mode)
	if pp.Plan == nil {
		fmt.Fprintf(&sb, " plan=nil %s/%s)", pp.Reason, pp.Detail)
		return sb.String()
	}
	for _, a := range pp.Plan.Activations {
		fmt.Fprintf(&sb, " act src=%d produces=%v", a.Source, a.Produces)
	}
	fmt.Fprintf(&sb, " %s/%s)", pp.Reason, pp.Detail)
	return sb.String()
}
