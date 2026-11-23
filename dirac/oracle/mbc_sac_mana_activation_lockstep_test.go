package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const sacManaActivationRepro = "sac_mana_funds_sorcery_sac_land_activation"

// CR 602.2b with CR 605.3b: Surtland Frostpyre's {2}{U}{U}{R}, {T}, Sacrifice
// activation is payable when Diamond Kaleidoscope's Prism token ("Sacrifice a
// Prism token: Add one mana of any colour") is the only red source, so the
// kernel offers it on Plains, Plains, Island, Island and a summoning-sick Prism
// (tail-12 pair 27, seed 7910311743838239922, decision 876: only the kernel's
// menu carries activate(#76)). With Vigil for the Lost out, the sacrificed
// Prism queues its pay-X trigger, and gorge poses that ask with a third,
// manual route (trigger_cost_pay_manual) that opens a window over every
// untapped mana source: gorge's prefix search spends its 160-node budget there
// before it reaches the red answer, and offers no activation. The kernel's
// tap-another search sees the ask as two answers; mirroring the manual window
// node for node is far past bounded dirac code. Surtland Frostpyre is refused
// by name and its recorded game still diverges with it implicated.
func TestMBCSurtlandFrostpyreReprosLockstep(t *testing.T) {
	t.Parallel()
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Surtland Frostpyre" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != sacManaActivationRepro {
			t.Fatalf("Surtland Frostpyre's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, sacManaActivationRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Surtland Frostpyre has no dirac/support/refused entry: gorge's prefix search spends its node budget in Vigil for the Lost's manual pay route before it finds the activation the kernel offers")
	}
}
