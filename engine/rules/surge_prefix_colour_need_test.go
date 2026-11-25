package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

const surgePrefixProbe = "Name:Surge Prefix Probe\nManaCost:0\nTypes:Sorcery\nA:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"

// TestSurgedCastPrefixNeedsModeColour: Boulder Salvo's surge cost is {1}{R},
// but the only red source on the board is Paradise Mantle's granted
// "{T}: Add one mana of any color" on Castle Raptors -- a census gap. The
// prefix search must order the granted tap's colour ask by the MODE cost's
// {R} (not the printed {4}{R}), or it spends its walk on the wrong colours
// and leaves the surged option unpriced as "unsupported census_incomplete".
func TestSurgedCastPrefixNeedsModeColour(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusCard(t, "Boulder Salvo"), card(t, surgePrefixProbe))
	probe := findAndMoveToHand(t, e, 0, "Surge Prefix Probe")
	salvo := findAndMoveToHand(t, e, 0, "Boulder Salvo")

	ppPriority(t, e)
	castObj(t, e, probe)
	if got := pay.SpellsCastThisTurn(asPayer(e), 0); got == 0 {
		t.Fatal("precondition: no spell cast this turn, so surge is not offered")
	}

	for i := 0; i < 3; i++ {
		onBoardCard(t, e, 0, corpusCard(t, "Plains"))
	}
	raptors := onBoardCard(t, e, 0, corpusCard(t, "Castle Raptors"))
	e.G.Obj(raptors).SummonSick = false
	mantle := onBoardCard(t, e, 0, corpusCard(t, "Paradise Mantle"))
	attachFixture(t, e, mantle, raptors)
	ppPriority(t, e)

	// The mode cost's pip, not the printed cost's: the colour ask must be
	// ordered by {R} or the search reaches for the wrong colours.
	need := scriptNeedColours(e, e.G.Obj(salvo), 0,
		decision.PotentialAction{Kind: "cast", Obj: salvo, Mode: "surged"})
	if !need[state.MR] {
		t.Fatalf("surged cast need = %v, want R (the surge cost's pip)", need)
	}
	if c := scriptPlayCost(e, e.G.Obj(salvo), 0, decision.PotentialAction{Kind: "cast", Obj: salvo, Mode: "surged"}); c.Generic != 1 || c.Colored[state.MR] != 1 {
		t.Fatalf("surged descriptor = %+v, want {1}{R} (not the printed {4}{R})", c)
	}

	var got PotentialPlan
	found := false
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == "cast" && pp.Action.Obj == salvo && pp.Action.Mode == "surged" {
			got, found = pp, true
		}
	}
	if !found {
		t.Fatal("precondition: the planner listed no surged option for Boulder Salvo")
	}
	if got.Reason != "" {
		t.Fatalf("surged cast not priced: reason %q detail %q", got.Reason, got.Detail)
	}
	if got.Plan != nil {
		red := false
		for _, a := range got.Plan.Activations {
			if a.Source == raptors && a.Produces[state.MR] > 0 {
				red = true
			}
		}
		if !red {
			t.Fatalf("witness %+v does not tap the granted source for R", got.Plan.Activations)
		}
		return
	}
	if len(got.Script) == 0 {
		t.Fatal("the plan is priced but names no witness or prefix script")
	}
	// Replay the prefix: the granted tap must be the {R} source.
	for i, st := range got.Script {
		d := e.Pending()
		if d == nil || d.Kind != st.Kind {
			t.Fatalf("step %d: pending %v, want a %v decision", i, d, st.Kind)
		}
		in := decision.Intent{Seq: d.Seq, Player: d.Player}
		for _, pk := range st.Picks {
			match := -1
			for j := range d.Options {
				if pk.Matches(&d.Options[j]) {
					match = d.Options[j].Index
					break
				}
			}
			if match < 0 {
				t.Fatalf("step %d: pick %+v not offered in %+v", i, pk, d.Options)
			}
			in.Choices = append(in.Choices, match)
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	if !e.G.Obj(raptors).Tapped || e.G.Players[0].Pool[state.MR] == 0 {
		t.Fatalf("the prefix did not tap the granted source for R: tapped=%v pool=%v",
			e.G.Obj(raptors).Tapped, e.G.Players[0].Pool)
	}
}
