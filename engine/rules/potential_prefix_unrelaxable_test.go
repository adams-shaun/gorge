package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestPrefixScriptFundsAndColoursAnUnrelaxableGap: Captain Benjamin Sisko's
// "{2}{W}{U}{B}{R}{G}, {T}" needs a red source no other permanent makes;
// Starfleet Academy's "{5}, {T}: Add X mana of any one color" is the only
// one. Its unbounded X leaves the planner census unrelaxable (CR 605.3a), so
// the only witness is the prefix search: the {5} must be funded by tapping
// other sources first, and the activation's colour ask must be answered red
// (CR 602.2b, 601.2h). The search used to try Starfleet's covered
// "{T}: Add {C}" first, spend its node budget in that branch, and leave the
// play unsupported; it also had no funding for an unrelaxable gap at all.
func TestPrefixScriptFundsAndColoursAnUnrelaxableGap(t *testing.T) {
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
	if census.complete || len(census.gaps) == 0 {
		t.Fatalf("census complete=%v gaps=%v, want Starfleet Academy's unbounded mana ability to be a gap", census.complete, census.gaps)
	}
	if census.relaxable {
		t.Fatalf("census is relaxable; the scenario is not the unrelaxable-gap case")
	}
	if got := census.fees(); got < 5 {
		t.Fatalf("census fees = %d, want at least Starfleet Academy's {5} activation fee", got)
	}
	var sisko state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Captain Benjamin Sisko" {
			sisko = id
		}
	}
	var found bool
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind != "ability" || pp.Action.Obj != sisko {
			continue
		}
		found = true
		if pp.Reason != "" || len(pp.Script) == 0 {
			t.Fatalf("Sisko's ability: reason %q detail %q, %d script steps, want a funded script", pp.Reason, pp.Detail, len(pp.Script))
		}
		red := false
		starfleet := false
		for _, st := range pp.Script {
			for _, pk := range st.Picks {
				if pk.ManaSymbol == "R" {
					red = true
				}
				if pk.Kind == "activate" && e.G.Obj(pk.Obj).Face().Name == "Starfleet Academy" {
					starfleet = true
				}
			}
		}
		if !starfleet || !red {
			t.Fatalf("script %+v has Starfleet=%v red=%v, want Starfleet activated and its colour ask answered red", pp.Script, starfleet, red)
		}
	}
	if !found {
		t.Fatal("Sisko's ability is not a potential play")
	}
}
