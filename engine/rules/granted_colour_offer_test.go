package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestScriptPickTellsSameLabelAbilitiesApart: Fire Nation Palace's "{1}{R}, {T}"
// pump over Islands, a Forest and a Jasmine Dragon Tea Shop, the only red being
// what Great Divide Guide grants (every land taps for any colour). The Tea Shop
// then has two abilities labelled "Add any color": its own, spendable only on
// Ally spells and abilities of Ally sources, and the granted one. The prefix
// script the planner finds taps the Tea Shop through the granted one; a replay
// that matched a pick by kind, object and label took the restricted one, whose
// red the Palace cannot spend, and the play was never offered. The pick names
// its ability, so the replay takes the same one.
func TestScriptPickTellsSameLabelAbilitiesApart(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9621, ppFiller)
	e.paidManaCensus = true
	palace := onBoardCard(t, e, 0, corpusCard(t, "Fire Nation Palace"))
	onBoardCard(t, e, 0, corpusCard(t, "Great Divide Guide"))
	for i := 0; i < 3; i++ {
		onBoard(t, e, 0, lrIsland)
	}
	onBoard(t, e, 0, lrForest)
	onBoardCard(t, e, 0, corpusCard(t, "Jasmine Dragon Tea Shop"))
	ppPriority(t, e)

	var script []ScriptStep
	for _, pl := range e.PotentialPaymentPlans(0) {
		if pl.Action.Obj == palace && pl.Action.Kind == "ability" && pl.Action.Ability == 1 {
			script = pl.Script
		}
	}
	if len(script) == 0 {
		t.Fatalf("precondition: the planner found no prefix script for the Palace pump")
	}
	for i, st := range script {
		d := e.Pending()
		if d == nil || d.Kind != st.Kind {
			t.Fatalf("step %d: pending %v, want a %v decision", i, d, st.Kind)
		}
		in := decision.Intent{Seq: d.Seq, Player: d.Player}
		for _, pk := range st.Picks {
			found := -1
			for j := range d.Options {
				if pk.Matches(&d.Options[j]) && !slices.Contains(in.Choices, d.Options[j].Index) {
					found = d.Options[j].Index
					break
				}
			}
			if found < 0 {
				t.Fatalf("step %d: pick %q not offered", i, pk.Label)
			}
			in.Choices = append(in.Choices, found)
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	d := e.Pending()
	if _, ok := autopayAbilityOffer(d, palace); !ok {
		t.Fatalf("Palace pump not offered after the script (restricted batches %+v): %+v", e.G.Players[0].RestrictedMana, d.Options)
	}
}
