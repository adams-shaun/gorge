package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const skarrgRagePitsRepro = "skarrg_rage_pits_pump_offer"

// CR 602.2b, 605.3a: Skarrg, the Rage Pits' "{R}{G}, {T}: pump" is payable
// here only through Chromatic Star ("{1}, {T}, Sacrifice: Add one mana of any
// color"), the board's one green source, with a Mountain funding the Star's
// {1}. Gorge's planner prices no filter from its census, so its prefix search
// runs for the activation. The search tries the Star's red answer first, a dead
// line, and spends its whole 160-node budget on that line's draw triggers (two
// Tolarian Krakens, each asking whether to pay {1} and for a target) before it
// reaches green: the play is "unsupported census_incomplete" and withheld,
// while the kernel's mirror walks a far smaller trigger tree, reaches green at
// node 55 and offers it (tail-62-c000 pair 9, decision 653, "options options").
// Mirroring the Krakens' payment asks node for node is not bounded dirac code,
// so the card is refused by name and the recorded game still shows the
// divergence with it implicated.
func TestMBCSkarrgTheRagePitsReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Skarrg, the Rage Pits" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != skarrgRagePitsRepro {
			t.Fatalf("Skarrg's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, skarrgRagePitsRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Skarrg, the Rage Pits has no dirac/support/refused entry: gorge's prefix search gives up on its pump beside Chromatic Star's draw triggers")
}
