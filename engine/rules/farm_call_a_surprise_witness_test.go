package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmCallaSurpriseWitness pins the MKM farm windows (MKM-s649 w20,
// MKM-s654 w21, MKM-s139 w18, and the Forge-only Chalk Outline rows s141/s152)
// where gorge shows a "whenever one or more creature cards leave your
// graveyard" watcher on the stack after Call a Surprise Witness resolves and
// Forge and XMage show an empty stack.
//
// Gorge is right by the rules: Witness returns a creature card from its
// controller's graveyard to the battlefield, so a creature card left that
// graveyard (CR 603.10a, 400.7) and a watcher already on the battlefield
// triggers once the spell has finished resolving (CR 603.3a). The farm repros
// were pruned before harvest, so the followers' reason for the empty stack was
// not measured; this pins gorge's side of it.
func TestFarmCallaSurpriseWitness(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, watcher := range []string{"Rot Farm Mortipede", "Chalk Outline", "Soul Enervation"} {
		t.Run(watcher, func(t *testing.T) {
			t.Parallel()
			sc := `{"name":"farm call a surprise witness ` + watcher + `","turn":1,"cr":["603.10a","603.3a"],"why":"inline","setup":{` +
				`"p0":{"battlefield":["Plains","Plains","` + watcher + `"],"hand":["Call a Surprise Witness"],` +
				`"graveyard":["Grizzly Bears"],"life":10},"p1":{"battlefield":[],"life":10}},"steps":[` +
				`{"op":"activate","seat":0,"card":"p0:Plains","ability_index":0},` +
				`{"op":"activate","seat":0,"card":"p0:Plains#2","ability_index":0},` +
				`{"op":"cast","seat":0,"card":"p0:Call a Surprise Witness","targets":["p0:Grizzly Bears"]},` +
				`{"op":"pass","seat":0},{"op":"pass","seat":1}]}`
			res, err := RunOracleScenarioJSON(reg, []byte(sc))
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if len(res.Fails) != 0 {
				t.Fatalf("scenario failed: %v", res.Fails)
			}
			byCheckpoint := map[string]*OracleSnapshot{}
			for i := range res.Snapshots {
				byCheckpoint[res.Snapshots[i].Checkpoint] = &res.Snapshots[i]
			}
			stack := func(s *OracleSnapshot) []string {
				var out []string
				for _, e := range s.Stack {
					out = append(out, e.Kind+":"+e.Source)
				}
				return out
			}
			// Precondition: the spell is on the stack alone, the watcher is
			// on the battlefield and the creature card is not (it is still
			// in the graveyard), so the resolution below is what moves it.
			cast, resolved := byCheckpoint["step 2 (cast)"], byCheckpoint["step 4 (pass)"]
			if cast == nil || resolved == nil {
				t.Fatalf("missing snapshots: %v", len(res.Snapshots))
			}
			if got := fmt.Sprint(stack(cast)); got != "[spell:p0:Call a Surprise Witness]" {
				t.Fatalf("stack after the cast = %s, want only the Witness (precondition)", got)
			}
			onBattlefield := func(s *OracleSnapshot, name string) bool {
				for _, p := range s.Permanents {
					if p.Controller == 0 && p.Name == name {
						return true
					}
				}
				return false
			}
			if !onBattlefield(cast, watcher) || onBattlefield(cast, "Grizzly Bears") {
				t.Fatalf("precondition: watcher on battlefield=%v, Grizzly Bears on battlefield=%v (want true, false)",
					onBattlefield(cast, watcher), onBattlefield(cast, "Grizzly Bears"))
			}
			if !onBattlefield(resolved, "Grizzly Bears") {
				t.Fatal("Witness resolved without returning Grizzly Bears to the battlefield")
			}
			want := "[ability:p0:" + watcher + "]"
			if got := fmt.Sprint(stack(resolved)); got != want {
				t.Fatalf("stack after Witness resolves = %s, want %s: a creature card left p0's graveyard (CR 603.10a)", got, want)
			}
		})
	}
}
