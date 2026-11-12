package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestRestrictedPoolCastPlansSourcesBeforeDiscard: Seize the Spoils ({2}{R},
// discard a card) cast while Abstract Paintmage's restricted {U}{R} floats
// taps its planned source at announcement, before the discard ask, as gorge's
// payment plan does (CR 601.2b, 601.2g-h, 106.6), and then retaps nothing at
// payment. The fixture must replay without a divergence in the Mountain's
// tapped flag or the pool.
func TestRestrictedPoolCastPlansSourcesBeforeDiscard(t *testing.T) {
	t.Parallel()
	const name = "restricted_pool_cast_plans_sources_before_discard"
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	// the precondition: the script ends on a cast, and a deck holds the card
	readable := actionStrings(r.Actions)
	if n := len(readable); n == 0 || !strings.HasPrefix(readable[n-1], "cast(") {
		t.Fatalf("fixture does not end on a cast: %v", readable[max(0, len(readable)-1):])
	}
	if !deckHas(r, "Seize the Spoils") {
		t.Fatal("fixture has no Seize the Spoils")
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-sos")
	replayRepro(t, reg, set, name, false, false)
}

// TestFreeCastUnpayableDiscardReturnsToZoneEnd: a card chosen for a free cast
// whose discard cost cannot be paid is pushed and reversed by gorge (CR 601.2b,
// 733.1), so it returns to the END of its exile zone behind the cards exiled
// with it; the kernel reorders it the same way.
func TestFreeCastUnpayableDiscardReturnsToZoneEnd(t *testing.T) {
	t.Parallel()
	const name = "free_cast_unpayable_discard_returns_to_zone_end"
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range actionStrings(r.Actions) {
		found = found || strings.HasPrefix(a, "mode(")
	}
	if !found || !deckHas(r, "Improvisation Capstone") || !deckHas(r, "Seize the Spoils") {
		t.Fatal("fixture has no Improvisation Capstone mode pick over Seize the Spoils")
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-sos")
	replayRepro(t, reg, set, name, true, false)
}

func deckHas(r *Repro, card string) bool {
	for _, d := range r.Cards {
		for _, n := range d {
			if n == card {
				return true
			}
		}
	}
	return false
}
