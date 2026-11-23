package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const wardscaleCrocodileRepro = "wardscale_crocodile_cast_offer"

// Wardscale Crocodile's {4}{G} cast (CR 601.2g) is payable on a board where
// Rain of Filth made every land "Sacrifice this land: Add {B}" until end of
// turn (CR 605.3b) and The World Tree lets the one untapped Swamp tap for {G}:
// one World Tree tap plus four land sacrifices (pool tail-08 pair 20, seed
// 933465, decision 721: the kernel's options alone carry
// cast(#79{Wardscale Crocodile})). Gorge's PlanCastPayment answers
// insufficient/last_resort, since the planner never spends sacrifice-only
// sources unless it must, so the cast is offered only through the prefix
// script search in engine/rules/potential_script.go. That search has a
// 160-state budget (potentialScriptNodes) and runs out at path depth 6 before
// it finds one tap and four sacrifices among ten interchangeable sacrifice
// gaps, so gorge's menu withholds the cast. Matching it means pricing gorge's
// DFS over sacrifice gaps in dirac/mana_window.go, well past bounded dirac
// code, so Wardscale Crocodile is refused by name and its recorded game still
// diverges with it implicated.
func TestMBCWardscaleCrocodileReprosLockstep(t *testing.T) {
	t.Parallel()
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Wardscale Crocodile" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != wardscaleCrocodileRepro {
			t.Fatalf("Wardscale Crocodile's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, wardscaleCrocodileRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Wardscale Crocodile has no dirac/support/refused entry: gorge withholds its {4}{G} cast when only Rain of Filth's land sacrifices fund it past its 160-node script search and the kernel offers it")
	}
}
