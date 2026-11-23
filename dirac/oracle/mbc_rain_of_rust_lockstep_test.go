package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const rainOfRustRepro = "rain_of_rust_entwine_offer"

// CR 702.42 with CR 605.3b: Rain of Rust's Entwine {3}{R} cast (total
// {6}{R}{R}{R}) is payable on tail-51-c000 pair 0 only when Treasonous Ogre's
// "Pay 3 life: Add {R}" is activated twice beside Dwarven Mine, with 12 life.
// Gorge's potential walk and prefix search offer the entwined cast at decision
// 536; the kernel's script walk prices no kicked cast and spends a life-only
// gap once, so it never offers it. A repeatable life gap in the search is far
// past bounded dirac code, so Rain of Rust is refused by name and its recorded
// game still diverges with it implicated.
func TestMBCRainOfRustReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Rain of Rust" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != rainOfRustRepro {
			t.Fatalf("Rain of Rust's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, rainOfRustRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Rain of Rust has no dirac/support/refused entry: gorge offers its entwined cast through a twice-used life mana ability the kernel never offers")
	}
}
