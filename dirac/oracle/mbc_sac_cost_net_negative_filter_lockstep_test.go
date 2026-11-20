package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const greatHallRepro = "sac_cost_activation_beside_net_negative_filter"

// CR 601.2g-h: Great Hall of Starnheim's {W}{W}{B}, {T}, sacrifice it and a
// creature is payable on a board whose only white comes from Saruli
// Caretaker (tap another creature) and Llanowar Envoy's {1}{G} any-colour
// filter. Gorge's planner pays it and offers the activation (tail-28 pair 10,
// decision 338 of 338); the kernel's activation script pricing declines a board
// holding a net-negative filter wholesale (actScriptPays) and omits it, and
// pricing a sacrifice part beside the filter search is far past bounded dirac
// code. Great Hall is refused by name and its recorded game still diverges
// with it implicated.
func TestMBCGreatHallOfStarnheimReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Great Hall of Starnheim" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != greatHallRepro {
			t.Fatalf("Great Hall of Starnheim's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, greatHallRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Great Hall of Starnheim has no dirac/support/refused entry: its sacrifice activation is priced beside a net-negative filter the kernel omits")
	}
}
