package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const bloodletterQuillRepro = "bloodletter_quill_prefix_search_budget_over_deferred_lands_unmodelled"

// Price of Glory ("whenever a player taps a land for mana, if it's not that
// player's turn, destroy that land") is a TapsForMana watcher gorge's planner
// cannot vet as a post-payment consequence (only the Mode$ Taps destroy-self
// row is), so on the opponent's turn every land of the payer is a deferred
// census gap and a play only those lands pay needs the prefix search. Gorge's
// depth-first search walks the subsets of ten gaps with a colour ask on each
// dual and exhausts its 160-clone budget (limited, approx) before reaching the
// {U}{B} Bloodletter Quill's "Remove a blood counter" needs from an Island and
// Jwar Isle Refuge, so the play stays census_incomplete and unoffered; the
// kernel's chain model finds the line at node 90 and offers it (tail-70, pair
// 29, decision 764: gorge [pass, Great Mound x2], kernel adds the Quill). The
// node walk is a clone-for-clone mirror that cannot be bounded in a few
// lines, so the Quill is refused by name and the recorded game still shows the
// divergence with it implicated.
func TestMBCBloodletterQuillReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Bloodletter Quill" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != bloodletterQuillRepro {
			t.Fatalf("Bloodletter Quill's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, bloodletterQuillRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Bloodletter Quill has no dirac/support/refused entry: gorge's prefix search over Price of Glory's deferred lands runs out of nodes before the {U}{B} line")
}
