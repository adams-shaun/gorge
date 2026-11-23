package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const creepingTarPitRepro = "creeping_tar_pit_paid_filter_chain"

// Creeping Tar Pit's {1}{U}{B} animation, its own {T} already spent on {U} for
// the first one, beside a Swamp wearing Urban Utopia (printed {B}, granted
// "any colour") and Nomadic Elf's repeatable "{1}{G}: Add one mana of any
// color" (tail-44 focus pair 6, decision 589). gorge's prefix search taps the
// granted-ability Swamp first, and the Elf's generic fee then spends that
// floating {B} (WUBRG order), so no path pays the activation within 22 nodes
// and it is not offered; the kernel's granted-mana and repeatable-filter
// search funds the Elf from the Forests first and offers it. Mirroring the
// two searches' interleave on this board is well over 200 lines (grantSrch
// beside filtSearch), so the card is refused by name and the recorded game
// still shows the divergence with it implicated.
func TestMBCCreepingTarPitReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Creeping Tar Pit" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != creepingTarPitRepro {
			t.Fatalf("Creeping Tar Pit's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, creepingTarPitRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Creeping Tar Pit has no dirac/support/refused entry: gorge's prefix search floats the granted Swamp's {B} for Nomadic Elf's fee to spend and the kernel's does not")
}
