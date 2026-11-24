package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const azoriusCluestoneRepro = "azorius_cluestone_draw_offer"

// Azorius Cluestone's "{W}{U}, {T}, Sacrifice: Draw a card" beside a Diamond
// Kaleidoscope (pool tail-12, surface autopay, the recorded game,
// decision 980): the only way to pay the {U} is the one Prism token's
// "Sacrifice a Prism token: Add one mana of any color", a census gap, so gorge
// prices the play by one prefix-search slot. Its Vigil for the Lost beside
// the Prism makes the sacrifice a dies trigger with an {X} payment: the search's
// first answer (W) walks the trigger's X answers and their manual-source
// windows, spends the 160-node budget and never answers U, so gorge offers no
// draw. The kernel's tapX search walks the trigger's ask as one two-answer node
// and reaches U in 37 nodes. The Cluestone is refused by name; the recorded
// game still shows the divergence with it implicated.
func TestMBCAzoriusCluestoneReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Azorius Cluestone" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != azoriusCluestoneRepro {
			t.Fatalf("Azorius Cluestone's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, azoriusCluestoneRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Azorius Cluestone has no dirac/support/refused entry: gorge's prefix search spends its node budget on a Vigil for the Lost X-payment subtree before answering the Prism's {U}")
}
