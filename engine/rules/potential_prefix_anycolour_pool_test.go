package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPrefixScriptAnyColourProductionIsSpendableColour: the prefix script that
// prices Captain Benjamin Sisko's "{2}{W}{U}{B}{R}{G}, {T}" (the red comes only
// from Starfleet Academy's "{5}, {T}: Add X mana of any one color") records the
// colour answer of the any-colour activation, and replaying it on the live
// engine leaves red in the pool, not the colourless fallback an ask with no
// decision channel would give (CR 605.1a, 106.1b). Pins the clone's any-colour
// production as spendable, so the play is offered (CR 602.2b, 601.2h).
func TestPrefixScriptAnyColourProductionIsSpendableColour(t *testing.T) {
	t.Parallel()
	e := tapXBoard(t, []string{
		"Starfleet Academy", "Captain Benjamin Sisko",
		"Plains", "Plains", "Plains", "Plains", "Forest",
		"Hallowed Fountain", "Hallowed Fountain", "Watery Grave", "Watery Grave",
		"Grizzly Bears", "Elite Vanguard", "Savannah Lions", "Serra Angel",
		"Air Elemental", "Hill Giant", "Goblin Piker", "Merfolk of the Pearl Trident",
		"Walking Corpse",
	})
	hyp := e.PotentialMana(0)
	census := e.paymentPlanCensusOf(0, &hyp)
	if census.complete || census.relaxable || census.fees() < 5 {
		t.Fatalf("precondition: census complete=%v relaxable=%v fees=%d, want incomplete, unrelaxable, fees >= 5", census.complete, census.relaxable, census.fees())
	}
	var sisko, starfleet state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil {
			switch o.Face().Name {
			case "Captain Benjamin Sisko":
				sisko = id
			case "Starfleet Academy":
				starfleet = id
			}
		}
	}
	if sisko == 0 || starfleet == 0 {
		t.Fatalf("precondition: sisko=%d starfleet=%d not both on the battlefield", sisko, starfleet)
	}
	var script []ScriptStep
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == "ability" && pp.Action.Obj == sisko {
			if pp.Reason != "" {
				t.Fatalf("Sisko's ability: reason %q detail %q, want a priced play", pp.Reason, pp.Detail)
			}
			script = pp.Script
		}
	}
	if len(script) == 0 {
		t.Fatal("precondition: Sisko's ability has no prefix script")
	}

	colourStep := -1
	for i, st := range script {
		d := e.Pending()
		if d == nil || d.Kind != st.Kind {
			t.Fatalf("step %d: pending %v, want a %v decision", i, d, st.Kind)
		}
		in := decision.Intent{Seq: d.Seq, Player: d.Player}
		isColour := false
		for _, pk := range st.Picks {
			if pk.Kind != "activate" && pk.Obj == starfleet && len(pk.ManaSymbol) == 1 {
				isColour = true
				if pk.ManaSymbol != "R" {
					t.Fatalf("step %d: Starfleet's colour ask answered %q, want R", i, pk.ManaSymbol)
				}
			}
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
		if isColour {
			colourStep = i
			pool := e.G.Players[0].Pool
			if pool[state.MR] <= 0 || pool[state.MC] != 0 {
				t.Fatalf("after the colour ask (step %d) pool = %v, want red > 0 and no colourless", i, pool)
			}
		}
	}
	if colourStep < 0 {
		t.Fatalf("script %+v has no Starfleet colour-ask step", script)
	}
	if _, ok := autopayAbilityOffer(e.Pending(), sisko); !ok {
		t.Fatalf("Sisko's ability not offered after the script (pool %v): %+v", e.G.Players[0].Pool, e.Pending().Options)
	}
}
