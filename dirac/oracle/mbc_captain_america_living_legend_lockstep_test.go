package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const captainAmericaLivingLegendRepro = "prefix_search_retaps_a_source_its_trigger_untapped"

// Paying an activation through a mana prefix whose tap triggers an untap-me
// trigger, gorge resolves that trigger mid-payment (the trigger-order ask put
// it first) and taps the same source again while a second trigger waits on the
// stack (CR 605.3b, 117.1a): Hydraulic Helper's {T} floats a second {U} and
// Rewrite History triggers twice (two draws, two discards, a third plan
// counter). The kernel's prefix search taps each gap once and resolves the
// whole pending stack at the end (scriptDFS, resolvePrefixStack): one tap, one
// trigger, one {U} (std-msh focus pair 182, decision 776). Modelling the
// interleave means a dynamic gap list inside the DFS walk and mid-walk trigger
// resolution with the trigger-order ask's branching, so the card is refused by
// name, as Maze of Ith is, and the recorded game still shows the divergence
// with it implicated.
func TestMBCCaptainAmericaLivingLegendReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Captain America, Living Legend" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != captainAmericaLivingLegendRepro {
			t.Fatalf("Captain America, Living Legend's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, captainAmericaLivingLegendRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Captain America, Living Legend has no dirac/support/refused entry: gorge's prefix search re-taps a source its untap trigger freed and the kernel taps each gap once")
}
