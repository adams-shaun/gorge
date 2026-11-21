package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const thermopodGrantRepro = "thermopod_ashaya_grant_over_a_sacrifice_mana_creature_unmodelled"

// CR 305.6: Ashaya, Soul of the Wild makes each nontoken creature a Forest
// land, and a land with a basic land type has the intrinsic "{T}: Add {G}"
// beside its own abilities. Thermopod's own ability (sacrifice a creature: Add
// {R}) is no plain {T} mana ability, so gorge offers both productions of the
// one creature and the kernel's one-ability pcard holds a single production
// (static_landtype.go blgPlain: stall landgrant, dr-mbc-baaa pair 3). Thermopod
// is refused by name, as Blanket of Night is, and the recorded game still shows
// the divergence with it implicated.
func TestMBCDrMbcBaaaReprosLockstep(t *testing.T) {
	if landGrantReplay(t) {
		return
	}
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Thermopod" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != thermopodGrantRepro {
			t.Fatalf("Thermopod's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, thermopodGrantRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Thermopod has no dirac/support/refused entry: its Ashaya grant stalls landgrant")
	}
	replayCompetingExileInsteadReplacements(t)
	replayLoweringSacrificeExileOrder(t)
}
