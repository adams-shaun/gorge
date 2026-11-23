package oracle

import (
	"path/filepath"
	"slices"
	"testing"
)

const bloodletterQuillRepro = "bloodletter_quill_prefix_search_budget_over_deferred_lands_unmodelled"

// Bloodletter Quill's {U}{B}: Remove a blood counter ability beside Price of
// Glory (a TapsForMana destroy watcher whose lands are deferred census gaps on
// the opponent's turn) once parted at decision 764 (tail-70, pair 29): the
// kernel's chain model offered the activation, gorge's depth-first prefix
// search ran out of its clone budget before the {U}{B} line and left it out.
// Main no longer parts there, so the recording replays whole and clean and the
// Quill stays unrefused; the test pins that, and fails if the recorded game
// drifts (a shorter script) or the divergence returns.
func TestMBCBloodletterQuillReprosLockstep(t *testing.T) {
	r, err := ReadRepro(filepath.Join("testdata", "repro", bloodletterQuillRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	held := func(name string) bool {
		return slices.Contains(r.Cards[0], name) || slices.Contains(r.Cards[1], name)
	}
	for _, name := range []string{"Bloodletter Quill", "Price of Glory"} {
		if !held(name) {
			t.Fatalf("precondition: the recording's decks must hold %s", name)
		}
	}
	if len(r.Actions) < 764 {
		t.Fatalf("precondition: the recording must reach decision 764, has %d actions", len(r.Actions))
	}
	stdReproNamedServed(t, bloodletterQuillRepro)
}
