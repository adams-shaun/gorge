package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A granted activated ability is the permanent's own ability (CR 613.1f,
// 602.1). The lowering names such a pick with ability_index 0 and the
// offered label; the runner must select it by that label even when the
// bearer has no printed activated ability (Memnite under Hypervolt Grasp).
func TestOracleActivateGrantedAbilityByIndexBareBearer(t *testing.T) {
	const scenario = `{"name":"granted-index-bare","cr":["602.1","613.1f"],"why":"a granted ability on a bearer without printed abilities",
 "setup":{"p0":{"battlefield":["Memnite","Hypervolt Grasp"]},"p1":{"battlefield":["Mountain"]}},
 "steps":[
  {"op":"attach","seat":0,"card":"p0:Hypervolt Grasp","attached_to":"p0:Memnite"},
  {"op":"activate","seat":0,"card":"p0:Memnite","targets":["p1"],"ability_index":0,"ability":"Memnite: CARDNAME deals 1 damage to any target."},
  {"op":"pass","seat":0},{"op":"pass","seat":1}]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario did not play cleanly: %v", res.Fails)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	if got := last.Players[1].Life; got != 19 {
		t.Fatalf("p1 life = %d, want 19 (Memnite's granted ability dealt 1)", got)
	}
}

// A granted non-mana ability on a land whose printed ability 0 is an
// untapped mana ability (Security Blockade on Seraph Sanctuary): the labelled
// pick selects the granted ability, not the "for mana" placeholder.
func TestOracleActivateGrantedAbilityByIndexBesideManaAbility(t *testing.T) {
	const scenario = `{"name":"granted-index-beside-mana","cr":["602.1","613.1f"],"why":"a granted ability beside a printed mana ability",
 "setup":{"p0":{"battlefield":["Seraph Sanctuary","Security Blockade"]},"p1":{"battlefield":["Mountain"]}},
 "steps":[
  {"op":"attach","seat":0,"card":"p0:Security Blockade","attached_to":"p0:Seraph Sanctuary"},
  {"op":"activate","seat":0,"card":"p0:Seraph Sanctuary","ability_index":0,"ability":"Seraph Sanctuary: Prevent the next 1 damage that would be dealt to you this turn."},
  {"op":"pass","seat":0},{"op":"pass","seat":1}]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario did not play cleanly: %v", res.Fails)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	if p := last.Players[0]; p.Pool != "" {
		t.Fatalf("p0 mana pool = %q: the mana ability was activated instead of the granted one", p.Pool)
	}
	for _, p := range last.Permanents {
		if p.Ref == "p0:Seraph Sanctuary" {
			if !p.Tapped {
				t.Fatal("Seraph Sanctuary is untapped: the granted {T} ability did not resolve")
			}
			return
		}
	}
	t.Fatal("Seraph Sanctuary not on the battlefield")
}
